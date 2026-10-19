package ui

import (
	"testing"
	"time"

	"github.com/binarynotfound/pomo/config"
	"github.com/binarynotfound/pomo/db"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
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

	// Reset should clear elapsed time
	_ = m.handleKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Equal(t, time.Duration(0), m.elapsed)
	assert.Equal(t, "00:00", m.buildTimeLeft())

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
}
