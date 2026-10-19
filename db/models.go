package db

import (
	"time"

	"github.com/binarynotfound/pomo/config"
)

var schema = `
CREATE TABLE IF NOT EXISTS sessions(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	duration INTEGER NOT NULL,
	started_at TEXT NOT NULL
);
`

type Session struct {
	ID        int           `db:"id"`
	Type      string        `db:"type"`
	Title     string        `db:"title"`
	Duration  time.Duration `db:"duration"`
	StartedAt string        `db:"started_at"`
}

func (s Session) ParsedTime() time.Time {
	if t, err := time.Parse(time.RFC3339, s.StartedAt); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s.StartedAt); err == nil {
		return t
	}
	if t, err := time.Parse(DateFormat, s.StartedAt); err == nil {
		return t
	}
	return time.Time{}
}

func (s Session) FormattedDateTime() string {
	t := s.ParsedTime()
	if t.IsZero() {
		return s.StartedAt
	}
	return t.Format("2006-01-02 15:04:05")
}

func (s Session) FormattedDate() string {
	t := s.ParsedTime()
	if t.IsZero() {
		if len(s.StartedAt) >= 10 {
			return s.StartedAt[:10]
		}
		return s.StartedAt
	}
	return t.Format(DateFormat)
}

func (s Session) FormattedTime() string {
	t := s.ParsedTime()
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04:05")
}

func (s Session) DisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	return s.Type + " session"
}

type AllTimeStats struct {
	TotalSessions      int           `db:"total_sessions"`
	TotalWorkDuration  time.Duration `db:"total_work_duration"`
	TotalBreakDuration time.Duration `db:"total_break_duration"`
}

type DailyStat struct {
	Date         string        `db:"day"`
	WorkDuration time.Duration `db:"work_duration"`
}

type DailyTitleStat struct {
	Date         string        `db:"day"`
	Title        string        `db:"title"`
	WorkDuration time.Duration `db:"work_duration"`
}

type StreakStats struct {
	Current int
	Best    int
}

type SessionType string

const (
	WorkSession  SessionType = "work"
	BreakSession SessionType = "break"
)

func GetSessionType(taskType config.TaskType) SessionType {
	if taskType == config.BreakTask {
		return BreakSession
	}
	return WorkSession
}
