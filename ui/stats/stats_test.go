package stats

import (
	"testing"
	"time"

	"github.com/binarynotfound/pomo/db"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSessionRepoWithTitleAndDeletion(t *testing.T) {
	database, err := sqlx.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer database.Close()

	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS sessions(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			duration INTEGER NOT NULL,
			started_at TEXT NOT NULL
		);
	`)
	require.NoError(t, err)

	repo := db.NewSessionRepo(database)

	t1 := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 19, 10, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 10, 19, 14, 15, 0, 0, time.UTC)

	require.NoError(t, repo.CreateSession(t1, 25*time.Minute, db.WorkSession, "write report"))
	require.NoError(t, repo.CreateSession(t2, 30*time.Minute, db.WorkSession, "review PR"))
	require.NoError(t, repo.CreateSession(t3, 45*time.Minute, db.WorkSession, "write report"))

	sessions, err := repo.GetAllSessions()
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	// Ordered by date and hour descending (newest first: t3, t2, t1)
	assert.Equal(t, "write report", sessions[0].Title)
	assert.Equal(t, "2026-10-19 14:15:00", sessions[0].FormattedDateTime())
	assert.Equal(t, "review PR", sessions[1].Title)
	assert.Equal(t, "2026-10-19 10:30:00", sessions[1].FormattedDateTime())
	assert.Equal(t, "write report", sessions[2].Title)
	assert.Equal(t, "2026-10-18 09:00:00", sessions[2].FormattedDateTime())

	// Update single session title
	require.NoError(t, repo.UpdateSessionTitle(sessions[1].ID, "architecture doc"))
	updatedSessions, err := repo.GetAllSessions()
	require.NoError(t, err)
	assert.Equal(t, "architecture doc", updatedSessions[1].Title)

	// Update multiple sessions title
	require.NoError(t, repo.UpdateSessionsTitle([]int{sessions[0].ID, sessions[2].ID}, "final report"))
	updatedSessions, err = repo.GetAllSessions()
	require.NoError(t, err)
	assert.Equal(t, "final report", updatedSessions[0].Title)
	assert.Equal(t, "final report", updatedSessions[2].Title)

	// Delete middle session
	require.NoError(t, repo.DeleteSession(sessions[1].ID))
	remaining, err := repo.GetAllSessions()
	require.NoError(t, err)
	require.Len(t, remaining, 2)

	// Delete remaining by IDs
	require.NoError(t, repo.DeleteSessionsByIDs([]int{remaining[0].ID, remaining[1].ID}))
	empty, err := repo.GetAllSessions()
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestStatsViewsAndNavigation(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40
	m.sessions = []db.Session{
		{
			ID:        3,
			Type:      "work",
			Title:     "write report",
			Duration:  45 * time.Minute,
			StartedAt: "2026-10-19T14:15:00Z",
		},
		{
			ID:        2,
			Type:      "work",
			Title:     "review PR",
			Duration:  30 * time.Minute,
			StartedAt: "2026-10-19T10:30:00Z",
		},
		{
			ID:        1,
			Type:      "work",
			Title:     "write report",
			Duration:  25 * time.Minute,
			StartedAt: "2026-10-18T09:00:00Z",
		},
	}

	// Default tab is TabOverview
	assert.Equal(t, TabOverview, m.activeTab)

	// Switch to TabSessions with Tab key
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	assert.Equal(t, TabSessions, m.activeTab)
	items := m.buildItems()
	require.Len(t, items, 3)
	assert.Equal(t, 3, items[0].session.ID)

	// Switch to TabByTitle with '3'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = updated.(Model)
	assert.Equal(t, TabByTitle, m.activeTab)
	items = m.buildItems()
	// 2 title groups ("write report" with 2 sessions, "review PR" with 1 session) = 2 + 3 = 5 items
	require.Len(t, items, 5)
	assert.Equal(t, itemGroup, items[0].kind)
	assert.Equal(t, "write report", items[0].groupLabel)
	assert.Equal(t, 70*time.Minute, items[0].totalDur)

	// Collapse first group with Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	items = m.buildItems()
	require.Len(t, items, 3) // 1 collapsed group + 1 expanded group + 1 session

	// Switch to TabByDate with '4'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m = updated.(Model)
	assert.Equal(t, TabByDate, m.activeTab)
	items = m.buildItems()
	// 2 date groups ("2026-10-19" with 2 sessions, "2026-10-18" with 1 session) = 5 items
	require.Len(t, items, 5)
	assert.Equal(t, itemGroup, items[0].kind)
	assert.Equal(t, "2026-10-19", items[0].groupLabel)

	// Move down to first session under 2026-10-19 and press 'd' to delete
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	assert.Equal(t, 1, m.cursor)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	assert.True(t, m.confirmingDelete)
	assert.Equal(t, []int{3}, m.deleteTargetIDs)
	assert.Contains(t, m.View(), "Delete session")

	// Cancel delete with 'n'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(Model)
	assert.False(t, m.confirmingDelete)
	assert.Nil(t, m.deleteTargetIDs)

	// Press 'e' on selected session to edit its title
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)
	assert.True(t, m.editingTitle)
	assert.Equal(t, []int{3}, m.editTargetIDs)
	assert.Equal(t, "write report", m.editValue)
	assert.Contains(t, m.View(), "Session name")

	// Clear and type new title
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("new task")})
	m = updated.(Model)
	assert.Equal(t, "new task", m.editValue)

	// Cancel edit with Esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	assert.False(t, m.editingTitle)
}

func TestOverviewBarChartByTitle(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 40
	m.weeklyStats = []db.DailyStat{
		{Date: "2026-10-18", WorkDuration: 30 * time.Minute},
		{Date: "2026-10-19", WorkDuration: 75 * time.Minute},
	}

	// Without custom titles (only default "work session"), renders plain chart without title legend
	m.weeklyTitleStats = []db.DailyTitleStat{
		{Date: "2026-10-18", Title: "work session", WorkDuration: 30 * time.Minute},
		{Date: "2026-10-19", Title: "work session", WorkDuration: 75 * time.Minute},
	}
	plainView := m.View()
	assert.NotContains(t, plainView, "██ work session")

	// With custom titles (-t), renders chart grouped by title with color legend by default
	m.weeklyTitleStats = []db.DailyTitleStat{
		{Date: "2026-10-18", Title: "backend", WorkDuration: 30 * time.Minute},
		{Date: "2026-10-19", Title: "backend", WorkDuration: 45 * time.Minute},
		{Date: "2026-10-19", Title: "frontend", WorkDuration: 30 * time.Minute},
	}
	groupedView := m.View()
	assert.Contains(t, groupedView, "backend")
	assert.Contains(t, groupedView, "frontend")

	// Cycling with Down ('j') on Overview filters by individual title
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	assert.Equal(t, 1, m.selectedTitleIdx)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	assert.Equal(t, 2, m.selectedTitleIdx)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	assert.Equal(t, 0, m.selectedTitleIdx)
}
