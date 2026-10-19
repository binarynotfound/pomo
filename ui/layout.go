package ui

import (
	"fmt"
	"time"

	"github.com/binarynotfound/pomo/ui/ascii"
	"github.com/binarynotfound/pomo/ui/colors"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxWidth           = 80
	margin             = 4
	padding            = 2
	separator          = " — "
	pausedIndicator    = "(paused)"
	completedIndicator = "done!"
)

func (m *Model) buildConfirmDialogView() string {
	switch m.confirmAction {
	case ConfirmReset:
		return m.confirmDialog.View("reset session?", 0)
	case ConfirmQuit:
		return m.confirmDialog.View("quit session?", 0)
	default:
		idle := time.Since(m.confirmStartTime).Truncate(time.Second)
		title := m.currentTaskType.Opposite().GetTask().Title

		// if we're prompting to start a long break
		if m.cyclePosition == m.longBreak.After {
			title = "long " + title
		}

		return m.confirmDialog.View("start "+title+"?", time.Duration(idle))
	}
}

func (m *Model) buildMainContent() string {
	timeLeft := m.buildTimeLeft()

	if m.useTimerArt {
		return timeLeft + "\n\n" + m.currentTask.Title
	}

	content := m.currentTask.Title
	if m.isStopwatch() || m.sessionState == RestCompleted || !m.timer.Timedout() {
		content += separator + timeLeft
	}

	return content
}

func (m *Model) buildStatusIndicators() string {
	if m.sessionState == RestCompleted {
		extra := time.Since(m.confirmStartTime).Truncate(time.Second)
		if extra > 0 {
			return separator + completedIndicator + fmt.Sprintf(" (+%s)", extra)
		}
		return separator + completedIndicator
	}

	if !m.isStopwatch() && m.timer.Timedout() {
		return separator + completedIndicator
	}

	indicators := ""

	if !m.isStopwatch() && !m.isRest() && m.longBreak.Enabled {
		indicators += fmt.Sprintf(" · %d/%d", m.cyclePosition, m.longBreak.After)
	}

	if m.sessionState == Paused {
		indicators += " " + pausedIndicator
	}

	return indicators
}

func (m *Model) buildProgressBar() string {
	return "\n\n" + m.progressBar.View() + "\n"
}

// returns time left (or elapsed/extra time) as a string in HH:MM:SS format
func (m *Model) buildTimeLeft() string {
	left := m.timer.Timeout
	if m.isStopwatch() {
		left = m.elapsed
	} else if m.sessionState == RestCompleted {
		left = time.Since(m.confirmStartTime).Truncate(time.Second)
	}
	hours := int(left.Hours())
	minutes := int(left.Minutes()) % 60
	seconds := int(left.Seconds()) % 60

	time := ""

	// only show hours if they are non-zero
	if hours > 0 {
		time += fmt.Sprintf("%02d:", hours)
	}
	time += fmt.Sprintf("%02d:%02d", minutes, seconds)

	if m.useTimerArt {
		time = ascii.RenderNumber(time, m.timerFont)

		// remove color on pause
		if m.sessionState == Paused {
			noColor := m.asciiTimerStyle.Foreground(colors.PauseFg)
			return noColor.Render(time)
		}

		return m.asciiTimerStyle.Render(time)
	}

	return time
}

func (m *Model) buildHelpView() string {
	if m.isStopwatch() {
		return m.help.View(stopwatchKeyMap)
	}
	if m.isRest() {
		return m.help.View(restKeyMap)
	}
	return m.help.View(keyMap)
}

func (m Model) buildWaitingForCommandsView() string {
	help := m.help.View(KeyMap{Quit: keyMap.Quit})

	message := lipgloss.JoinVertical(
		lipgloss.Center,
		"Waiting for post commands to complete...",
		"\n",
		help,
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		message,
	)
}
