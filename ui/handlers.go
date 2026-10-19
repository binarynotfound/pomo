package ui

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/binarynotfound/pomo/actions"
	"github.com/binarynotfound/pomo/config"
	"github.com/binarynotfound/pomo/db"
	"github.com/binarynotfound/pomo/ui/confirm"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
)

type (
	confirmTickMsg  struct{}
	commandsDoneMsg struct{}
)

func (m *Model) handleKeys(msg tea.KeyMsg) tea.Cmd {
	if m.sessionState == ShowingConfirm {
		return m.confirmDialog.HandleKeys(msg)
	}

	if m.sessionState == WaitingForCommands {
		// allow quitting immediately while waiting for commands
		if key.Matches(msg, keyMap.Quit) {
			return m.Quit()
		}

		// ignore other keys
		return nil
	}

	switch {
	case key.Matches(msg, keyMap.Increase):
		if m.isStopwatch() || m.sessionState == RestCompleted {
			return nil
		}
		m.duration += time.Minute
		return m.updateProgressBar()

	case key.Matches(msg, keyMap.Pause):
		if m.sessionState == RestCompleted {
			return nil
		}

		if m.isStopwatch() {
			if m.sessionState == Paused {
				m.sessionState = Running
				return m.stopwatch.Start()
			}
			m.sessionState = Paused
			return m.stopwatch.Stop()
		}

		if m.sessionState == Paused {
			m.sessionState = Running
		} else if m.getPercent() != 1.0 { // prevent pausing if session is already completed
			m.sessionState = Paused
		}

		if m.sessionState == Running {
			return m.timer.Start()
		}

		return nil

	case key.Matches(msg, keyMap.Reset):
		return m.promptConfirm(ConfirmReset)

	case key.Matches(msg, keyMap.Skip):
		if m.isStopwatch() || m.isRest() {
			return nil
		}
		m.recordSession()
		return m.nextSession()

	case key.Matches(msg, keyMap.Quit):
		return m.promptConfirm(ConfirmQuit)

	default:
		return nil
	}
}

func (m *Model) promptConfirm(action ConfirmAction) tea.Cmd {
	m.prevSessionState = m.sessionState
	m.confirmAction = action
	m.sessionState = ShowingConfirm
	m.confirmDialog.Reset(false)

	if m.isStopwatch() && m.prevSessionState == Running {
		return m.stopwatch.Stop()
	}
	return nil
}

func (m *Model) resumeFromConfirm() tea.Cmd {
	m.sessionState = m.prevSessionState
	if m.sessionState == RestCompleted {
		return func() tea.Msg {
			return confirmTickMsg{}
		}
	}
	if m.sessionState == Running {
		if m.isStopwatch() {
			return m.stopwatch.Start()
		}
		return m.timer.Start()
	}
	return nil
}

func (m *Model) handleConfirmChoice(msg confirm.ChoiceMsg) tea.Cmd {
	switch m.confirmAction {
	case ConfirmReset:
		if msg.Choice == confirm.Confirm {
			if m.prevSessionState == RestCompleted {
				return m.startSession(m.currentTaskType, m.currentTask, false)
			}
			m.sessionState = m.prevSessionState
			m.elapsed = 0
			if m.isStopwatch() {
				if m.sessionState == Running {
					return tea.Batch(m.stopwatch.Reset(), m.stopwatch.Start())
				}
				return m.stopwatch.Reset()
			}
			m.duration = m.currentTask.Duration
			if m.sessionState == Running {
				return tea.Batch(m.updateProgressBar(), m.timer.Start())
			}
			return m.updateProgressBar()
		}
		return m.resumeFromConfirm()

	case ConfirmQuit:
		if msg.Choice == confirm.Confirm {
			m.recordSession()
			return m.Quit()
		}
		return m.resumeFromConfirm()

	default:
		if m.currentTaskType == config.WorkTask {
			extra := time.Since(m.confirmStartTime).Truncate(time.Second)
			if extra > 0 {
				m.elapsed += extra
			}
			m.recordSession()
		}
		switch msg.Choice {
		case confirm.Confirm:
			return m.nextSession()
		case confirm.ShortSession:
			return m.shortSession()
		case confirm.Cancel:
			return m.Quit()
		}
	}

	return nil
}

func (m *Model) handleWindowResize(msg tea.WindowSizeMsg) tea.Cmd {
	m.confirmDialog.HandleWindowResize(msg) // always update it

	m.width = msg.Width
	m.height = msg.Height
	m.progressBar.Width = min(m.width-2*padding-margin, maxWidth)

	return nil
}

func (m *Model) handleTimerTick(msg timer.TickMsg) tea.Cmd {
	if m.sessionState != Running {
		return nil
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.timer, cmd = m.timer.Update(msg)
	cmds = append(cmds, cmd)

	if cmd == nil {
		// msg rejected, ignore
		// i.e. old timer tick
		log.Println("tick rejected, ignoring")
		return nil
	}

	m.elapsed += m.timer.Interval

	percent := m.getPercent()
	cmds = append(cmds, m.progressBar.SetPercent(percent))

	return tea.Batch(cmds...)
}

func (m *Model) handleStopwatchTick(msg stopwatch.TickMsg) tea.Cmd {
	if m.sessionState != Running {
		return nil
	}

	var cmd tea.Cmd
	m.stopwatch, cmd = m.stopwatch.Update(msg)

	if cmd == nil {
		log.Println("stopwatch tick rejected, ignoring")
		return nil
	}

	m.elapsed = m.stopwatch.Elapsed()

	return cmd
}

func (m *Model) handleStopwatchMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.stopwatch, cmd = m.stopwatch.Update(msg)

	return cmd
}

func (m *Model) handleConfirmTick() tea.Cmd {
	if (m.sessionState != ShowingConfirm || m.confirmAction != ConfirmNextSession) && m.sessionState != RestCompleted {
		return nil
	}

	// send tick every second to update idle/extra time
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return confirmTickMsg{}
	})
}

func (m *Model) handleTimerStartStop(msg timer.StartStopMsg) tea.Cmd {
	var cmd tea.Cmd
	m.timer, cmd = m.timer.Update(msg)

	return cmd
}

func (m *Model) handleProgressBarFrame(msg progress.FrameMsg) tea.Cmd {
	if m.progressBar.Percent() >= 1.0 && !m.progressBar.IsAnimating() && m.sessionState == Running {
		return m.handleCompletion()
	}

	progressModel, cmd := m.progressBar.Update(msg)
	m.progressBar = progressModel.(progress.Model)

	return cmd
}

func (m *Model) updateProgressBar() tea.Cmd {
	// reset timer with new duration minus passed time
	m.timer.Timeout = m.duration - m.elapsed

	// update progress bar
	return m.progressBar.SetPercent(m.getPercent())
}

// returns the elapsed time as a percentage of total duration,
// rounded down to 2 decimal places to avoid floating point precision issues.
func (m Model) getPercent() float64 {
	passed := float64(m.elapsed.Milliseconds())
	duration := float64(m.duration.Milliseconds())

	// round to 2 decimal places
	return math.Floor((passed/duration)*100) / 100
}

func (m *Model) handleCompletion() tea.Cmd {
	log.Println("timer completed")

	ctx, cancel := context.WithTimeout(context.Background(), actions.CommandTimeout)
	m.commandsCancel = cancel
	m.commandsWg = actions.RunPostActions(ctx, m.currentTask)

	if m.isRest() {
		m.sessionState = RestCompleted
		m.confirmStartTime = time.Now()

		// send first tick to count up extra time after rest ends
		return func() tea.Msg {
			return confirmTickMsg{}
		}
	}

	// after a pomodoro work session ends, count up extra time until the user
	// starts the break or exits, and save that extra time with the work session
	if m.currentTaskType == config.WorkTask {
		if m.onSessionEnd == "quit" {
			m.recordSession()
			return m.Quit()
		}

		m.confirmAction = ConfirmNextSession
		m.confirmDialog.Reset(true)
		m.sessionState = ShowingConfirm
		m.confirmStartTime = time.Now()

		return func() tea.Msg {
			return confirmTickMsg{}
		}
	}

	m.recordSession()

	// continue after break completion according to config
	switch m.onSessionEnd {
	case "ask":
		m.confirmAction = ConfirmNextSession
		m.confirmDialog.Reset(true)
		m.sessionState = ShowingConfirm
		m.confirmStartTime = time.Now()

		// send first confirm tick
		return func() tea.Msg {
			return confirmTickMsg{}
		}
	case "start":
		return m.nextSession()
	case "quit":
		return m.Quit()
	default:
		log.Printf("unknown onSessionEnd value %q, defaulting to quit", m.onSessionEnd)
		return m.Quit()
	}
}

// starts session with the opposite task type (work <-> break)
// handles long break logic if enabled
func (m *Model) nextSession() tea.Cmd {
	if m.longBreak.Enabled {
		// increment step count after break sessions
		if m.currentTaskType == config.BreakTask {
			m.cyclePosition++
		}

		// start long break if cycle position reaches configured value after a work session
		if m.currentTaskType == config.WorkTask && m.cyclePosition == m.longBreak.After {
			return m.longBreakSession()
		}

		// reset step count after long break
		if m.cyclePosition > m.longBreak.After {
			m.cyclePosition = 1
		}
	}

	nextTaskType := m.currentTaskType.Opposite()
	return m.startSession(nextTaskType, *nextTaskType.GetTask(), false)
}

// starts a long break session
func (m *Model) longBreakSession() tea.Cmd {
	longBreak := *config.BreakTask.GetTask()
	longBreak.Duration = m.longBreak.Duration
	longBreak.Title = "long " + longBreak.Title

	return m.startSession(config.BreakTask, longBreak, false)
}

// starts a short session of the current task type
func (m *Model) shortSession() tea.Cmd {
	shortTask := m.currentTask
	shortTask.Duration = 2 * time.Minute // TODO: make configurable
	shortTask.Title = "short " + m.currentTaskType.GetTask().Title

	return m.startSession(m.currentTaskType, shortTask, true)
}

// initializes and starts a new session with the given task
func (m *Model) startSession(taskType config.TaskType, task config.Task, isShortSession bool) tea.Cmd {
	// cancel any running post actions
	// before starting the next session
	//
	// for onSessionEnd == "start", we don't cancel immediately
	// will run commands in the background with the context time limit
	if m.commandsCancel != nil && m.onSessionEnd != "start" {
		m.commandsCancel()
	}

	// clean up previous commands state
	m.commandsWg, m.commandsCancel = nil, nil

	m.isShortSession = isShortSession
	m.currentTaskType = taskType
	m.currentTask = task

	m.elapsed = 0
	m.duration = m.currentTask.Duration
	m.timer = timer.New(m.currentTask.Duration)

	m.sessionState = Running
	return tea.Batch(
		m.progressBar.SetPercent(0.0),
		m.timer.Start(),
	)
}

// records the current session into the session summary
func (m *Model) recordSession() {
	// rest sessions are standalone and not saved in the database
	if m.isRest() {
		return
	}

	// ignore very short or zero duration sessions
	if m.elapsed < time.Second {
		return
	}

	// short sessions extend the current session without incrementing the count
	if m.isShortSession {
		m.sessionSummary.AddDuration(m.currentTaskType, m.elapsed)
		return
	}

	m.sessionSummary.AddSession(m.currentTaskType, m.elapsed)

	// only store work sessions in the database
	if db.GetSessionType(m.currentTaskType) != db.WorkSession {
		return
	}

	// return if no database is configured
	if m.repo == nil {
		return
	}

	if err := m.repo.CreateSession(
		time.Now(),
		m.elapsed,
		db.GetSessionType(m.currentTaskType),
		m.currentTask.Title,
	); err != nil {
		log.Printf("failed to record session: %v", err)
	}
}

// handles the completion of post actions and quits the application
func (m *Model) handleCommandsDone() tea.Cmd {
	m.sessionState = Quitting
	return tea.Quit
}

// waits for any running post actions to complete before quitting the application
func (m *Model) waitForCommands() tea.Cmd {
	m.sessionState = WaitingForCommands

	return func() tea.Msg {
		if m.commandsWg != nil {
			log.Println("waiting for post actions to complete...")
			m.commandsWg.Wait()
			log.Println("post actions completed")
		}

		// cancel any running commands
		// in case they are still running after the wait
		if m.commandsCancel != nil {
			m.commandsCancel()
		}

		return commandsDoneMsg{}
	}
}

// Quit handles quitting the application
// ensuring that any running post actions are completed before exiting
func (m *Model) Quit() tea.Cmd {
	// if we're already waiting for commands to finish, force quit
	if m.sessionState == WaitingForCommands {
		log.Println("force quitting...")

		// cancel any running commands
		if m.commandsCancel != nil {
			m.commandsCancel()
		}

		m.sessionState = Quitting
		return tea.Quit
	}

	// wait for any running post actions to complete before quitting
	if m.commandsWg != nil {
		return m.waitForCommands()
	}

	m.sessionState = Quitting
	return tea.Quit
}
