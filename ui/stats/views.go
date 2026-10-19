package stats

import (
	"fmt"
	"strings"
	"time"

	"github.com/binarynotfound/pomo/db"
	"github.com/binarynotfound/pomo/ui/colors"
	"github.com/charmbracelet/lipgloss"
)

type Tab int

const (
	TabOverview Tab = iota
	TabSessions
	TabByTitle
	TabByDate
	totalTabs
)

var tabNames = []string{
	"1: Overview",
	"2: Sessions",
	"3: By Title (-t)",
	"4: By Date",
}

type itemKind int

const (
	itemSession itemKind = iota
	itemGroup
)

type selectableItem struct {
	kind       itemKind
	session    db.Session
	groupKey   string
	groupLabel string
	sessionIDs []int
	totalDur   time.Duration
	workDur    time.Duration
	breakDur   time.Duration
	latestTime string
	collapsed  bool
}

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.Cream).
			Background(colors.Purple).
			Padding(0, 2).
			Margin(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colors.Gray).
				Padding(0, 2).
				Margin(0, 1)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colors.PurpleLight)

	normalRowStyle = lipgloss.NewStyle()

	dimTextStyle = lipgloss.NewStyle().
			Foreground(colors.Gray)

	workTypeStyle = lipgloss.NewStyle().
			Foreground(colors.Purple)

	breakTypeStyle = lipgloss.NewStyle().
			Foreground(colors.Blue)

	groupHeaderStyle = lipgloss.NewStyle().
				Bold(true)

	confirmBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colors.BorderFg).
				Padding(1, 4)

	activeBtnStyle = lipgloss.NewStyle().
			Foreground(colors.ActiveButtonFg).
			Background(colors.ActiveButtonBg).
			Padding(0, 3).
			Margin(0, 1)

	inactiveBtnStyle = lipgloss.NewStyle().
				Foreground(colors.InactiveButtonFg).
				Background(colors.InactiveButtonBg).
				Padding(0, 3).
				Margin(0, 1)
)

func (m Model) renderTabs() string {
	var tabs []string
	for i, name := range tabNames {
		if Tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(name))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// buildItems returns the navigable list of items for the current tab.
// Sessions in m.sessions are already ordered by date and hour (newest first).
func (m Model) buildItems() []selectableItem {
	switch m.activeTab {
	case TabSessions:
		items := make([]selectableItem, 0, len(m.sessions))
		for _, s := range m.sessions {
			items = append(items, selectableItem{
				kind:    itemSession,
				session: s,
			})
		}
		return items

	case TabByTitle:
		type titleGroup struct {
			title      string
			sessions   []db.Session
			sessionIDs []int
			totalDur   time.Duration
			latestTime string
		}
		var order []string
		groups := make(map[string]*titleGroup)

		for _, s := range m.sessions {
			title := s.DisplayTitle()
			g, exists := groups[title]
			if !exists {
				g = &titleGroup{
					title:      title,
					latestTime: s.FormattedDateTime(),
				}
				groups[title] = g
				order = append(order, title)
			}
			g.sessions = append(g.sessions, s)
			g.sessionIDs = append(g.sessionIDs, s.ID)
			g.totalDur += s.Duration
		}

		var items []selectableItem
		for _, title := range order {
			g := groups[title]
			collapsed := m.collapsedTitles[title]
			items = append(items, selectableItem{
				kind:       itemGroup,
				groupKey:   title,
				groupLabel: title,
				sessionIDs: g.sessionIDs,
				totalDur:   g.totalDur,
				latestTime: g.latestTime,
				collapsed:  collapsed,
			})
			if !collapsed {
				for _, s := range g.sessions {
					items = append(items, selectableItem{
						kind:    itemSession,
						session: s,
					})
				}
			}
		}
		return items

	case TabByDate:
		type dateGroup struct {
			date       string
			sessions   []db.Session
			sessionIDs []int
			totalDur   time.Duration
			workDur    time.Duration
			breakDur   time.Duration
		}
		var order []string
		groups := make(map[string]*dateGroup)

		for _, s := range m.sessions {
			date := s.FormattedDate()
			g, exists := groups[date]
			if !exists {
				g = &dateGroup{
					date: date,
				}
				groups[date] = g
				order = append(order, date)
			}
			g.sessions = append(g.sessions, s)
			g.sessionIDs = append(g.sessionIDs, s.ID)
			g.totalDur += s.Duration
			if s.Type == string(db.BreakSession) {
				g.breakDur += s.Duration
			} else {
				g.workDur += s.Duration
			}
		}

		var items []selectableItem
		for _, date := range order {
			g := groups[date]
			collapsed := m.collapsedDates[date]
			items = append(items, selectableItem{
				kind:       itemGroup,
				groupKey:   date,
				groupLabel: date,
				sessionIDs: g.sessionIDs,
				totalDur:   g.totalDur,
				workDur:    g.workDur,
				breakDur:   g.breakDur,
				collapsed:  collapsed,
			})
			if !collapsed {
				for _, s := range g.sessions {
					items = append(items, selectableItem{
						kind:    itemSession,
						session: s,
					})
				}
			}
		}
		return items

	default:
		return nil
	}
}

func (m Model) renderListView() string {
	items := m.buildItems()
	if len(items) == 0 {
		return dimTextStyle.Render("No saved sessions found.")
	}

	maxVisible := max(m.height-12, 6)
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(items))

	var rows []string
	for i := start; i < end; i++ {
		selected := i == m.cursor
		rows = append(rows, m.formatItemRow(items[i], selected))
	}

	if len(items) > maxVisible {
		footer := dimTextStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(items)))
		rows = append(rows, "", footer)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) formatItemRow(item selectableItem, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "❯ "
	}

	if item.kind == itemGroup {
		arrow := "▼"
		if item.collapsed {
			arrow = "▶"
		}

		var summary string
		if m.activeTab == TabByTitle {
			summary = fmt.Sprintf(
				"%s %s  (%d sessions · %s · latest: %s)",
				arrow,
				item.groupLabel,
				len(item.sessionIDs),
				item.totalDur.Round(time.Second),
				item.latestTime,
			)
		} else {
			summary = fmt.Sprintf(
				"%s %s  (%d sessions · work: %s · break: %s)",
				arrow,
				item.groupLabel,
				len(item.sessionIDs),
				item.workDur.Round(time.Second),
				item.breakDur.Round(time.Second),
			)
		}

		line := prefix + summary
		if selected {
			return selectedRowStyle.Render(line)
		}
		return groupHeaderStyle.Render(line)
	}

	// Session row
	s := item.session
	durStr := fmt.Sprintf("%-8s", s.Duration.Round(time.Second).String())
	typeStr := fmt.Sprintf("%-5s", s.Type)
	if !selected {
		if s.Type == string(db.BreakSession) {
			typeStr = breakTypeStyle.Render(typeStr)
		} else {
			typeStr = workTypeStyle.Render(typeStr)
		}
	}

	var line string
	switch m.activeTab {
	case TabSessions:
		line = fmt.Sprintf("%s%s   %s   %s   %s", prefix, s.FormattedDateTime(), typeStr, durStr, s.DisplayTitle())
	case TabByTitle:
		indent := "    "
		if selected {
			indent = "  ❯ "
		}
		line = fmt.Sprintf("%s%s   %s   %s", indent, s.FormattedDateTime(), typeStr, durStr)
	case TabByDate:
		indent := "    "
		if selected {
			indent = "  ❯ "
		}
		timeOnly := s.FormattedTime()
		if timeOnly == "" {
			timeOnly = s.FormattedDateTime()
		}
		line = fmt.Sprintf("%s%s   %s   %s   %s", indent, timeOnly, typeStr, durStr, s.DisplayTitle())
	}

	if selected {
		return selectedRowStyle.Render(strings.TrimRight(line, " "))
	}
	return normalRowStyle.Render(strings.TrimRight(line, " "))
}

func (m Model) renderDeleteConfirmView() string {
	var yesBtn, noBtn string
	if m.confirmDeleteYes {
		yesBtn = activeBtnStyle.Render("Yes")
		noBtn = inactiveBtnStyle.Render("No")
	} else {
		yesBtn = inactiveBtnStyle.Render("Yes")
		noBtn = activeBtnStyle.Render("No")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesBtn, noBtn)
	dialog := confirmBorderStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(m.deletePrompt),
			"",
			buttons,
		),
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			dialog,
			"",
			m.help.View(ConfirmDeleteKeys),
		),
	)
}

func (m Model) renderEditTitleView() string {
	cursor := lipgloss.NewStyle().Foreground(colors.Purple).Render("█")
	var inputLine string
	if m.editValue == "" {
		placeholder := dimTextStyle.Render("Enter session title...")
		inputLine = "> " + cursor + placeholder
	} else {
		inputLine = "> " + m.editValue + cursor
	}

	dialog := confirmBorderStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(m.editPrompt),
			"",
			inputLine,
		),
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			dialog,
			"",
			m.help.View(EditTitleKeys),
		),
	)
}
