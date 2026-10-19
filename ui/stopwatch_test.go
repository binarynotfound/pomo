package ui

import (
	"testing"
	"time"

	"github.com/binarynotfound/pomo/config"
	"github.com/binarynotfound/pomo/db"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestStopwatchModel(t *testing.T) {
	cfg := config.Config{
		Work: config.Task{
			Title:    "stopwatch test",
			Duration: 25 * time.Minute,
		},
		ASCIIArt: config.ASCIIArt{
			Enabled: false,
		},
		LongBreak: config.LongBreak{
			Enabled: true,
			After:   4,
		},
	}

	config.C = cfg
	m := NewModel(config.StopwatchTask, cfg)
	m.repo = nil // avoid writing to real user DB during tests

	assert.True(t, m.isStopwatch())
	assert.Equal(t, Running, m.sessionState)
	assert.Equal(t, "00:00", m.buildTimeLeft())
	assert.Contains(t, m.buildMainContent(), "stopwatch test — 00:00")
	assert.Equal(t, "", m.buildStatusIndicators())

	// Simulate elapsed time
	m.elapsed = 1*time.Hour + 5*time.Minute + 9*time.Second
	assert.Equal(t, "01:05:09", m.buildTimeLeft())

	// Increase and Skip should be ignored in stopwatch mode
	prevDuration := m.duration
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, prevDuration, m.duration)

	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, config.StopwatchTask, m.currentTaskType)

	// Pause and Resume
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, Paused, m.sessionState)
	assert.Contains(t, m.buildStatusIndicators(), pausedIndicator)

	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, Running, m.sessionState)

	// Reset should prompt for confirmation first
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Equal(t, ShowingConfirm, m.sessionState)
	assert.Equal(t, ConfirmReset, m.confirmAction)
	assert.Contains(t, m.View(), "reset session?")

	// Canceling reset ('n') should preserve elapsed time and return to Running
	cmd := m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil {
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	assert.Equal(t, Running, m.sessionState)
	assert.Equal(t, "01:05:09", m.buildTimeLeft())

	// Confirming reset ('h' then 'y') should clear elapsed time
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Equal(t, ShowingConfirm, m.sessionState)
	cmd = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd != nil {
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	assert.Equal(t, Running, m.sessionState)
	assert.Equal(t, time.Duration(0), m.elapsed)
	assert.Equal(t, "00:00", m.buildTimeLeft())

	// Quit should prompt for confirmation first
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Equal(t, ShowingConfirm, m.sessionState)
	assert.Equal(t, ConfirmQuit, m.confirmAction)
	assert.Contains(t, m.View(), "quit session?")

	// Canceling quit ('n') should return to Running
	cmd = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil {
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	assert.Equal(t, Running, m.sessionState)

	// Confirming quit ('q' then 'y') should transition to Quitting
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	cmd = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd != nil {
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	assert.Equal(t, Quitting, m.sessionState)

	// Session type for DB should be WorkSession
	assert.Equal(t, db.WorkSession, db.GetSessionType(config.StopwatchTask))
}

func TestRestModel(t *testing.T) {
	cfg := config.Config{
		Break: config.Task{
			Title:    "rest session",
			Duration: 10 * time.Minute,
			Notification: config.Notification{
				Enabled: false,
			},
		},
		ASCIIArt: config.ASCIIArt{
			Enabled: false,
		},
		LongBreak: config.LongBreak{
			Enabled: true,
			After:   4,
		},
	}

	config.C = cfg
	m := NewModel(config.RestTask, cfg)
	m.repo = nil

	assert.True(t, m.isRest())
	assert.Equal(t, Running, m.sessionState)
	assert.Equal(t, "10:00", m.buildTimeLeft())
	assert.Equal(t, "", m.buildStatusIndicators()) // no cycle indicator for rest

	// Skip should be ignored in rest mode
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, config.RestTask, m.currentTaskType)

	// Simulate completion of rest timer
	m.elapsed = 10 * time.Minute
	_ = m.handleCompletion()
	assert.Equal(t, RestCompleted, m.sessionState)

	// Simulate 5 seconds of extra time after rest ends
	m.confirmStartTime = time.Now().Add(-5 * time.Second)
	assert.Equal(t, "00:05", m.buildTimeLeft())
	assert.Contains(t, m.buildStatusIndicators(), "done! (+5s)")

	// Reset in RestCompleted prompts for confirmation and restarts rest session on 'y'
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Equal(t, ShowingConfirm, m.sessionState)
	assert.Equal(t, ConfirmReset, m.confirmAction)
	cmd := m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd != nil {
		msg := cmd()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	assert.Equal(t, Running, m.sessionState)
	assert.Equal(t, time.Duration(0), m.elapsed)
}

func TestOnlyWorkSessionsSavedToDB(t *testing.T) {
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

	cfg := config.Config{
		Work: config.Task{
			Title:    "work session",
			Duration: 25 * time.Minute,
		},
		Break: config.Task{
			Title:    "break session",
			Duration: 5 * time.Minute,
		},
	}
	config.C = cfg

	// Break session in pomodoro mode should NOT be saved to DB
	breakModel := NewModel(config.BreakTask, cfg)
	breakModel.repo = repo
	breakModel.elapsed = 5 * time.Minute
	breakModel.recordSession()

	sessions, err := repo.GetAllSessions()
	require.NoError(t, err)
	assert.Empty(t, sessions)

	// Work session in pomodoro mode SHOULD be saved to DB
	workModel := NewModel(config.WorkTask, cfg)
	workModel.repo = repo
	workModel.elapsed = 25 * time.Minute
	workModel.recordSession()

	sessions, err = repo.GetAllSessions()
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "work", sessions[0].Type)
}
