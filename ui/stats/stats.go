// Package stats implements the statistics view for pomo.
package stats

import (
	"errors"
	"fmt"
	"time"

	"github.com/Bahaaio/pomo/db"
	"github.com/Bahaaio/pomo/ui/colors"
	"github.com/Bahaaio/pomo/ui/stats/components"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	barChartHeight     = 12
	durationRatioWidth = 30
)

var errStyle = lipgloss.NewStyle().
	Foreground(colors.ErrorMessageFg).
	AlignHorizontal(lipgloss.Center)

type Model struct {
	// components
	durationRatio components.DurationRatio
	barChart      components.BarChart
	heatMap       components.HeatMap
	streak        components.Streak

	// error message
	err error

	// stats
	allTimeStats db.AllTimeStats
	weeklyStats  []db.DailyStat
	monthlyStats []db.DailyStat
	streakStats  db.StreakStats
	sessions     []db.Session

	// navigation & view state
	activeTab        Tab
	cursor           int
	collapsedTitles  map[string]bool
	collapsedDates   map[string]bool
	confirmingDelete bool
	confirmDeleteYes bool
	deleteTargetIDs  []int
	deletePrompt     string

	// state
	width, height int
	help          help.Model
	quitting      bool
}

func New() Model {
	return Model{
		durationRatio:   components.NewDurationRatio(durationRatioWidth),
		barChart:        components.NewBarChart(barChartHeight),
		heatMap:         components.NewHeatMap(),
		streak:          components.NewStreak(),
		collapsedTitles: make(map[string]bool),
		collapsedDates:  make(map[string]bool),
		help:            help.New(),
	}
}

type statsMsg struct {
	allTimeStats db.AllTimeStats
	weeklyStats  []db.DailyStat
	monthlyStats []db.DailyStat
	streakStats  db.StreakStats
	sessions     []db.Session
}

type errMsg struct {
	err error
}

// fetchStats retrieves statistics and sessions from the database and returns them as a statsMsg.
// If an error occurs, it returns an errMsg instead.
func fetchStats() tea.Msg {
	database, err := db.Connect()
	if err != nil {
		return errMsg{err: errors.New("failed to connect to the database")}
	}
	defer database.Close()

	repo := db.NewSessionRepo(database)

	stats, err := repo.GetAllTimeStats()
	if err != nil {
		return errMsg{err: errors.New("failed to fetch all-time stats")}
	}

	weeklyStats, err := repo.GetWeeklyStats()
	if err != nil {
		return errMsg{err: errors.New("failed to fetch weekly stats")}
	}

	monthlyStats, err := repo.GetLastMonthsStats(components.NumberOfMonths)
	if err != nil {
		return errMsg{err: errors.New("failed to fetch heatmap stats")}
	}

	streakStats, err := repo.GetStreakStats()
	if err != nil {
		return errMsg{err: errors.New("failed to fetch streak stats")}
	}

	sessions, err := repo.GetAllSessions()
	if err != nil {
		return errMsg{err: errors.New("failed to fetch saved sessions")}
	}

	return statsMsg{
		allTimeStats: stats,
		weeklyStats:  weeklyStats,
		monthlyStats: monthlyStats,
		streakStats:  streakStats,
		sessions:     sessions,
	}
}

func deleteSessionsCmd(ids []int) tea.Cmd {
	return func() tea.Msg {
		database, err := db.Connect()
		if err != nil {
			return errMsg{err: errors.New("failed to connect to the database")}
		}
		defer database.Close()

		repo := db.NewSessionRepo(database)
		if err := repo.DeleteSessionsByIDs(ids); err != nil {
			return errMsg{err: errors.New("failed to delete session(s)")}
		}

		return fetchStats()
	}
}

func (m Model) Init() tea.Cmd {
	return fetchStats
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.err != nil {
		return m.buildErrorMessage()
	}

	if m.confirmingDelete {
		return m.renderDeleteConfirmView()
	}

	tabs := m.renderTabs()

	var body string
	var helpView string

	switch m.activeTab {
	case TabOverview:
		durationRatio := m.durationRatio.View(
			m.allTimeStats.TotalWorkDuration,
			m.allTimeStats.TotalBreakDuration,
		)
		streak := m.streak.View(m.streakStats)
		chart := m.barChart.View(m.weeklyStats)
		hMap := m.heatMap.View(m.monthlyStats)
		charts := lipgloss.JoinHorizontal(lipgloss.Bottom, chart, "   ", hMap)

		body = lipgloss.JoinVertical(
			lipgloss.Center,
			durationRatio,
			"",
			streak,
			"\n",
			charts,
		)
		helpView = m.help.View(OverviewKeys)

	case TabSessions:
		body = m.renderListView()
		helpView = m.help.View(SessionsKeys)

	case TabByTitle, TabByDate:
		body = m.renderListView()
		helpView = m.help.View(GroupedKeys)
	}

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			"Pomodoro statistics",
			"",
			tabs,
			"\n",
			body,
			"",
			helpView,
		),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statsMsg:
		m.allTimeStats = msg.allTimeStats
		m.weeklyStats = msg.weeklyStats
		m.monthlyStats = msg.monthlyStats
		m.streakStats = msg.streakStats
		m.sessions = msg.sessions
		m.clampCursor()
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		return m.handleKeys(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) clampCursor() {
	items := m.buildItems()
	if len(items) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(items) {
		m.cursor = len(items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) buildErrorMessage() string {
	title := "An error occurred while fetching statistics."
	message := m.err.Error()

	help := m.help.View(OverviewKeys)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"",
		message,
		"",
		help,
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		errStyle.Render(content),
	)
}

func (m Model) handleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmingDelete {
		return m.handleDeleteConfirmKeys(msg)
	}

	switch {
	case key.Matches(msg, Keys.Quit):
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, Keys.NextTab):
		m.activeTab = (m.activeTab + 1) % totalTabs
		m.cursor = 0
		return m, nil

	case key.Matches(msg, Keys.PrevTab):
		m.activeTab = (m.activeTab - 1 + totalTabs) % totalTabs
		m.cursor = 0
		return m, nil

	case key.Matches(msg, Keys.Tab1):
		m.activeTab = TabOverview
		m.cursor = 0
		return m, nil

	case key.Matches(msg, Keys.Tab2):
		m.activeTab = TabSessions
		m.cursor = 0
		return m, nil

	case key.Matches(msg, Keys.Tab3):
		m.activeTab = TabByTitle
		m.cursor = 0
		return m, nil

	case key.Matches(msg, Keys.Tab4):
		m.activeTab = TabByDate
		m.cursor = 0
		return m, nil

	case key.Matches(msg, Keys.Up):
		if m.activeTab != TabOverview && m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, Keys.Down):
		if m.activeTab != TabOverview {
			items := m.buildItems()
			if m.cursor < len(items)-1 {
				m.cursor++
			}
		}
		return m, nil

	case key.Matches(msg, Keys.Toggle):
		if m.activeTab == TabByTitle || m.activeTab == TabByDate {
			items := m.buildItems()
			if m.cursor >= 0 && m.cursor < len(items) && items[m.cursor].kind == itemGroup {
				gk := items[m.cursor].groupKey
				if m.activeTab == TabByTitle {
					m.collapsedTitles[gk] = !m.collapsedTitles[gk]
				} else {
					m.collapsedDates[gk] = !m.collapsedDates[gk]
				}
				m.clampCursor()
			}
		}
		return m, nil

	case key.Matches(msg, Keys.Delete):
		if m.activeTab != TabOverview {
			items := m.buildItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				item := items[m.cursor]
				if item.kind == itemSession {
					s := item.session
					m.deleteTargetIDs = []int{s.ID}
					m.deletePrompt = fmt.Sprintf(
						"Delete session %q (%s on %s)?",
						s.DisplayTitle(),
						s.Duration.Round(time.Second),
						s.FormattedDateTime(),
					)
				} else {
					m.deleteTargetIDs = item.sessionIDs
					m.deletePrompt = fmt.Sprintf(
						"Delete all %d session(s) in %q (%s)?",
						len(item.sessionIDs),
						item.groupLabel,
						item.totalDur.Round(time.Second),
					)
				}
				m.confirmingDelete = true
				m.confirmDeleteYes = true
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleDeleteConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, Keys.Confirm):
		ids := m.deleteTargetIDs
		m.confirmingDelete = false
		m.deleteTargetIDs = nil
		return m, deleteSessionsCmd(ids)

	case key.Matches(msg, Keys.Cancel):
		m.confirmingDelete = false
		m.deleteTargetIDs = nil
		return m, nil

	case key.Matches(msg, ConfirmDeleteKeys.NextTab):
		m.confirmDeleteYes = !m.confirmDeleteYes
		return m, nil

	case key.Matches(msg, ConfirmDeleteKeys.Toggle):
		if m.confirmDeleteYes {
			ids := m.deleteTargetIDs
			m.confirmingDelete = false
			m.deleteTargetIDs = nil
			return m, deleteSessionsCmd(ids)
		}
		m.confirmingDelete = false
		m.deleteTargetIDs = nil
		return m, nil

	case key.Matches(msg, Keys.Quit):
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}
