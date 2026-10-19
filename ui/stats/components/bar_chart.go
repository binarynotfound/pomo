// Package components implements UI components for stats.
package components

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Bahaaio/pomo/db"
	"github.com/Bahaaio/pomo/ui/colors"
	"github.com/charmbracelet/lipgloss"
)

var barStyle = lipgloss.NewStyle().Foreground(colors.WorkSessionFg)

const (
	barChar     = "█"
	axisChar    = "│"
	tickChar    = "┤"
	cornerChar  = "└"
	lineChar    = "─"
	paddingChar = " "

	barThickness = 3
	spacing      = 2
	daysInWeek   = 7
)

var spacer = strings.Repeat(paddingChar, spacing)

type chartLayout struct {
	barHeight       int
	yAxisLabelWidth int
	yAxisWidth      int // label + space + tick char
	barAreaWidth    int
	totalWidth      int
}

type BarChart struct {
	chartLayout
}

func NewBarChart(height int) BarChart {
	return BarChart{
		chartLayout: chartLayout{
			barHeight: height - 1 - 1, // leave space for x-axis and labels
		},
	}
}

func displayTitle(title string) string {
	if title == "" {
		return "work session"
	}
	return title
}

// HasTitledData reports whether any weekly work session was registered with a custom (-t) title.
func HasTitledData(titleStats []db.DailyTitleStat) bool {
	for _, s := range titleStats {
		if s.WorkDuration > 0 && s.Title != "" && s.Title != "work session" && s.Title != "break session" {
			return true
		}
	}
	return false
}

// ExtractTitles returns the ordered list of unique display titles with work duration > 0.
func ExtractTitles(titleStats []db.DailyTitleStat) []string {
	seen := make(map[string]bool)
	var titles []string

	for _, s := range titleStats {
		if s.WorkDuration <= 0 {
			continue
		}
		t := displayTitle(s.Title)
		if !seen[t] {
			seen[t] = true
			titles = append(titles, t)
		}
	}

	return titles
}

func (b *BarChart) calculateLayout(maxDuration, scale time.Duration) chartLayout {
	longestLabel := 0

	for duration := maxDuration; duration > 0; duration -= scale {
		label := formatDuration(duration)
		longestLabel = max(longestLabel, len(label))
	}

	yAxisLabelWidth := longestLabel
	yAxisWidth := yAxisLabelWidth + 1 + 1 // length of label + space + tick char

	barAreaWidth := spacing + (barThickness+spacing)*daysInWeek

	return chartLayout{
		barHeight:       b.barHeight,
		yAxisLabelWidth: yAxisLabelWidth,
		yAxisWidth:      yAxisWidth,
		barAreaWidth:    barAreaWidth,
		totalWidth:      yAxisWidth + barAreaWidth,
	}
}

func (b *BarChart) View(stats []db.DailyStat) string {
	return b.renderSingleColorChart(stats, barStyle)
}

// ViewWithTitles renders the weekly bar chart grouped and colored by session title if custom titles exist.
// If no custom titles exist, it falls back to the standard View.
// If selectedTitle is non-empty, only that title's bars are rendered in its assigned color.
func (b *BarChart) ViewWithTitles(stats []db.DailyStat, titleStats []db.DailyTitleStat, selectedTitle string) string {
	if len(stats) == 0 {
		return ""
	}

	if !HasTitledData(titleStats) {
		return b.View(stats)
	}

	titles := ExtractTitles(titleStats)
	titleColors := make(map[string]lipgloss.Color, len(titles))
	for i, t := range titles {
		titleColors[t] = colors.GetTitleColor(i)
	}

	// Build per-day per-title duration map
	dayTitleDur := make(map[string]map[string]time.Duration)
	for _, ts := range titleStats {
		if ts.WorkDuration <= 0 {
			continue
		}
		if dayTitleDur[ts.Date] == nil {
			dayTitleDur[ts.Date] = make(map[string]time.Duration)
		}
		t := displayTitle(ts.Title)
		dayTitleDur[ts.Date][t] += ts.WorkDuration
	}

	// If filtering by a single title, render single-color chart with that title's color
	if selectedTitle != "" {
		filtered := make([]db.DailyStat, len(stats))
		for i, st := range stats {
			filtered[i] = db.DailyStat{
				Date:         st.Date,
				WorkDuration: dayTitleDur[st.Date][selectedTitle],
			}
		}
		style := lipgloss.NewStyle().Foreground(titleColors[selectedTitle])
		chart := b.renderSingleColorChart(filtered, style)
		legend := b.buildTitleLegend(titles, titleColors, selectedTitle)
		return lipgloss.JoinVertical(lipgloss.Left, chart, "", legend)
	}

	// Grouped (stacked multi-color) chart across all titles
	maxDuration := getMaxDuration(stats)
	targetTicks := b.barHeight / 2
	scale := calculateScale(maxDuration, targetTicks)

	if maxDuration == 0 {
		maxDuration = time.Hour
		scale = time.Minute * 10
	}

	b.chartLayout = b.calculateLayout(maxDuration, scale)

	yAxis := b.buildYAxis(maxDuration, scale)
	bars := b.buildGroupedBars(stats, titles, dayTitleDur, titleColors, maxDuration)

	top := lipgloss.JoinHorizontal(lipgloss.Left, yAxis, spacer, bars)
	xAxis := b.buildXAxis()
	labels := b.buildLabels(stats)
	legend := b.buildTitleLegend(titles, titleColors, "")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		top,
		xAxis,
		labels,
		"",
		legend,
	)
}

func (b *BarChart) renderSingleColorChart(stats []db.DailyStat, style lipgloss.Style) string {
	if len(stats) == 0 {
		return ""
	}

	maxDuration := getMaxDuration(stats)

	// dividing by half of max height to leave space for tick chars
	targetTicks := b.barHeight / 2
	scale := calculateScale(maxDuration, targetTicks)

	// fallback for empty stats
	if maxDuration == 0 {
		maxDuration = time.Hour
		scale = time.Minute * 10
	}

	b.chartLayout = b.calculateLayout(maxDuration, scale)

	yAxis := b.buildYAxis(maxDuration, scale)
	bars := b.buildBarsWithStyle(stats, maxDuration, style)

	top := lipgloss.JoinHorizontal(lipgloss.Left, yAxis, spacer, bars)
	xAxis := b.buildXAxis()
	labels := b.buildLabels(stats)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		top,
		xAxis,
		labels,
	)
}

func (b *BarChart) buildBars(stats []db.DailyStat, maxDuration time.Duration) string {
	return b.buildBarsWithStyle(stats, maxDuration, barStyle)
}

func (b *BarChart) buildBarsWithStyle(stats []db.DailyStat, maxDuration time.Duration, style lipgloss.Style) string {
	bars := make([]string, 0, len(stats))

	for _, stat := range stats {
		if stat.WorkDuration == 0 {
			bars = append(bars, renderBarWithStyle(0, style), spacer)
			continue
		}

		barHeight := int((float64(stat.WorkDuration) / float64(maxDuration)) * float64(b.barHeight))
		if barHeight == 0 && stat.WorkDuration > 0 {
			barHeight = 1
		}
		bar := renderBarWithStyle(barHeight, style)
		bars = append(bars, bar, spacer)
	}

	return lipgloss.JoinHorizontal(lipgloss.Bottom, bars...)
}

func (b *BarChart) buildGroupedBars(
	stats []db.DailyStat,
	titles []string,
	dayTitleDur map[string]map[string]time.Duration,
	titleColors map[string]lipgloss.Color,
	maxDuration time.Duration,
) string {
	bars := make([]string, 0, len(stats)*2)

	for _, stat := range stats {
		if stat.WorkDuration == 0 {
			bars = append(bars, renderBarWithStyle(0, barStyle), spacer)
			continue
		}

		totalBarHeight := int((float64(stat.WorkDuration) / float64(maxDuration)) * float64(b.barHeight))
		if totalBarHeight == 0 && stat.WorkDuration > 0 {
			totalBarHeight = 1
		}

		dayMap := dayTitleDur[stat.Date]
		type segment struct {
			title string
			dur   time.Duration
			rows  int
		}
		var active []segment
		for _, t := range titles {
			if d := dayMap[t]; d > 0 {
				active = append(active, segment{title: t, dur: d})
			}
		}

		if len(active) == 0 {
			bars = append(bars, renderBarWithStyle(totalBarHeight, barStyle), spacer)
			continue
		}

		// Ensure totalBarHeight is at least len(active) (up to b.barHeight) so each active title on that day is visible
		effectiveHeight := min(max(totalBarHeight, len(active)), b.barHeight)
		assigned := 0
		for i := range active {
			rows := max(int(math.Round((float64(active[i].dur)/float64(stat.WorkDuration))*float64(effectiveHeight))), 1)
			active[i].rows = rows
			assigned += rows
		}

		// Adjust largest segment so total rows == effectiveHeight
		diff := effectiveHeight - assigned
		if diff != 0 {
			maxIdx := 0
			for i := 1; i < len(active); i++ {
				if active[i].dur > active[maxIdx].dur {
					maxIdx = i
				}
			}
			active[maxIdx].rows = max(1, active[maxIdx].rows+diff)
		}

		var renderedRows []string
		barRow := strings.Repeat(barChar, barThickness)
		for _, seg := range active {
			st := lipgloss.NewStyle().Foreground(titleColors[seg.title])
			for range seg.rows {
				renderedRows = append(renderedRows, st.Render(barRow))
			}
		}

		bars = append(bars, strings.Join(renderedRows, "\n"), spacer)
	}

	return lipgloss.JoinHorizontal(lipgloss.Bottom, bars...)
}

func (b *BarChart) buildTitleLegend(
	titles []string,
	titleColors map[string]lipgloss.Color,
	selectedTitle string,
) string {
	var parts []string
	for _, t := range titles {
		swatch := lipgloss.NewStyle().Foreground(titleColors[t]).Render("██")
		label := t
		if selectedTitle != "" {
			if t == selectedTitle {
				label = lipgloss.NewStyle().Bold(true).Underline(true).Render(t)
			} else {
				label = lipgloss.NewStyle().Foreground(colors.Gray).Render(t)
			}
		}
		parts = append(parts, swatch+" "+label)
	}

	padding := strings.Repeat(paddingChar, b.yAxisWidth+spacing)
	return padding + strings.Join(parts, "  ")
}

func (b *BarChart) buildYAxis(maxDuration, scale time.Duration) string {
	builder := strings.Builder{}

	// smallest duration to print
	epsilon := time.Millisecond * 500

	// print all rows with same width of longest duration label
	for duration := maxDuration; duration >= epsilon; duration -= scale {
		tick := fmt.Sprintf("%-*s %s\n", b.yAxisLabelWidth, formatDuration(duration), tickChar)
		builder.WriteString(tick)

		axis := strings.Repeat(paddingChar, b.yAxisLabelWidth) + paddingChar + axisChar
		builder.WriteString(axis)

		// don't print the last new line
		if duration-scale >= epsilon {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func (b *BarChart) buildXAxis() string {
	zeroLabel := fmt.Sprintf("%-*s", b.yAxisLabelWidth, "0")
	return zeroLabel + " " + cornerChar + strings.Repeat(lineChar, b.barAreaWidth)
}

func (b *BarChart) buildLabels(stats []db.DailyStat) string {
	var labels strings.Builder

	for _, stat := range stats {
		day := getDayLabel(stat.Date)
		labels.WriteString(day)
		labels.WriteString(spacer)
	}

	// yaxis width + spacing between yaxis and bars
	paddingLength := b.yAxisWidth + spacing
	padding := strings.Repeat(paddingChar, paddingLength)

	return padding + labels.String()
}

func getDayLabel(day string) string {
	t, err := time.Parse(db.DateFormat, day)
	if err != nil {
		return strings.Repeat(paddingChar, barThickness)
	}

	// get first three letters of weekday
	return t.Weekday().String()[:barThickness]
}

func renderBar(height int) string {
	return renderBarWithStyle(height, barStyle)
}

func renderBarWithStyle(height int, style lipgloss.Style) string {
	if height == 0 {
		// return an empty bar for alignment
		return strings.Repeat(paddingChar, barThickness)
	}

	bar := strings.Repeat(barChar, barThickness)

	// don't render last new line char
	return style.Render(strings.Repeat(bar+"\n", height-1) + bar)
}

func getMaxDuration(stats []db.DailyStat) time.Duration {
	var maxDuration time.Duration

	for _, stat := range stats {
		if stat.WorkDuration > maxDuration {
			maxDuration = stat.WorkDuration
		}
	}

	return maxDuration
}

func calculateScale(maxDuration time.Duration, targetTicks int) time.Duration {
	scale := time.Duration(
		float64(maxDuration.Milliseconds())/float64(targetTicks),
	) * time.Millisecond

	// minimum scale of 100 ms to avoid too many ticks
	scale = max(scale, time.Millisecond*100)

	return scale
}

func formatDuration(d time.Duration) string {
	seconds := d.Seconds()
	if seconds < 60 {
		if seconds == float64(int(seconds)) {
			return fmt.Sprintf("%ds", int(seconds))
		}

		// show one decimal place for seconds less than 1
		return fmt.Sprintf("%0.1fs", seconds)
	}

	minutes := int(d.Minutes())
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}

	hours := minutes / 60
	mins := minutes % 60

	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}

	return fmt.Sprintf("%dh%dm", hours, mins)
}
