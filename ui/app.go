// Package ui provides the terminal user interface for pomodoro sessions.
package ui

import (
	"github.com/binarynotfound/pomo/ui/confirm"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) Init() tea.Cmd {
	if m.isStopwatch() {
		return m.stopwatch.Init()
	}
	return m.timer.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m, m.handleKeys(msg)

	case tea.WindowSizeMsg:
		return m, m.handleWindowResize(msg)

	case timer.TickMsg:
		return m, m.handleTimerTick(msg)

	case stopwatch.TickMsg:
		return m, m.handleStopwatchTick(msg)

	case stopwatch.StartStopMsg, stopwatch.ResetMsg:
		return m, m.handleStopwatchMsg(msg)

	case confirmTickMsg:
		return m, m.handleConfirmTick()

	case timer.StartStopMsg:
		return m, m.handleTimerStartStop(msg)

	case progress.FrameMsg:
		return m, m.handleProgressBarFrame(msg)

	case confirm.ChoiceMsg:
		return m, m.handleConfirmChoice(msg)

	case commandsDoneMsg:
		return m, m.handleCommandsDone()

	default:
		return m, nil
	}
}

func (m Model) View() string {
	if m.sessionState == Quitting {
		return ""
	}

	if m.sessionState == WaitingForCommands {
		return m.buildWaitingForCommandsView()
	}

	// show confirmation dialog
	if m.sessionState == ShowingConfirm {
		return m.buildConfirmDialogView()
	}

	content := m.buildMainContent()
	content += m.buildStatusIndicators()
	if !m.isStopwatch() {
		content += m.buildProgressBar()
	} else {
		content += "\n"
	}

	help := m.buildHelpView()

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, content, help),
	)
}
