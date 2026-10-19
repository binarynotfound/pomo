package db

import (
	"time"

	"github.com/jmoiron/sqlx"
)

const DateFormat = "2006-01-02"

type SessionRepo struct {
	db *sqlx.DB
}

func NewSessionRepo(db *sqlx.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// CreateSession inserts a new session record into the database.
func (r *SessionRepo) CreateSession(startedAt time.Time, duration time.Duration, sessionType SessionType, titles ...string) error {
	startedAtStr := startedAt.Format(time.RFC3339)
	title := ""
	if len(titles) > 0 {
		title = titles[0]
	}

	if _, err := r.db.Exec(
		"insert into sessions (started_at, duration, type, title) values (?, ?, ?, ?);",
		startedAtStr,
		duration,
		sessionType,
		title,
	); err != nil {
		return err
	}

	return nil
}

// GetAllSessions retrieves all saved work sessions ordered by date and time (newest first).
func (r *SessionRepo) GetAllSessions() ([]Session, error) {
	var sessions []Session

	if err := r.db.Select(
		&sessions,
		`
		SELECT id, type, COALESCE(title, '') AS title, duration, started_at
		FROM sessions
		WHERE type = 'work'
		ORDER BY datetime(started_at) DESC, id DESC;
		`,
	); err != nil {
		return nil, err
	}

	return sessions, nil
}

// DeleteSession deletes a session by its ID.
func (r *SessionRepo) DeleteSession(id int) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE id = ?;", id)
	return err
}

// DeleteSessionsByIDs deletes multiple sessions by their IDs.
func (r *SessionRepo) DeleteSessionsByIDs(ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	query, args, err := sqlx.In("DELETE FROM sessions WHERE id IN (?);", ids)
	if err != nil {
		return err
	}

	query = r.db.Rebind(query)
	_, err = r.db.Exec(query, args...)
	return err
}

// UpdateSessionTitle updates the title of a single session by its ID.
func (r *SessionRepo) UpdateSessionTitle(id int, title string) error {
	_, err := r.db.Exec("UPDATE sessions SET title = ? WHERE id = ?;", title, id)
	return err
}

// UpdateSessionsTitle updates the title of multiple sessions by their IDs.
func (r *SessionRepo) UpdateSessionsTitle(ids []int, title string) error {
	if len(ids) == 0 {
		return nil
	}

	query, args, err := sqlx.In("UPDATE sessions SET title = ? WHERE id IN (?);", title, ids)
	if err != nil {
		return err
	}

	query = r.db.Rebind(query)
	_, err = r.db.Exec(query, args...)
	return err
}

// GetAllTimeStats retrieves aggregate statistics across all sessions.
func (r *SessionRepo) GetAllTimeStats() (AllTimeStats, error) {
	var totalStats AllTimeStats

	// sqlite treats (type = 'work') as 1 or 0
	if err := r.db.Get(
		&totalStats,
		`
		SELECT
			COUNT(*) AS total_sessions,
			COALESCE(SUM(duration * (type = 'work')), 0)  AS total_work_duration,
			COALESCE(SUM(duration * (type = 'break')), 0) AS total_break_duration
		FROM sessions;
		`,
	); err != nil {
		return AllTimeStats{}, err
	}

	return totalStats, nil
}

// GetWeeklyStats retrieves daily work duration statistics for the past 7 days.
func (r *SessionRepo) GetWeeklyStats() ([]DailyStat, error) {
	today := time.Now()
	firstDay := today.AddDate(0, 0, -6)

	return r.getDailyStats(firstDay, today)
}

// GetWeeklyTitleStats retrieves daily work duration statistics grouped by day and title for the past 7 days.
func (r *SessionRepo) GetWeeklyTitleStats() ([]DailyTitleStat, error) {
	today := time.Now()
	firstDay := today.AddDate(0, 0, -6)

	fromStr := firstDay.Format(DateFormat)
	toStr := today.Format(DateFormat)

	var stats []DailyTitleStat

	if err := r.db.Select(
		&stats,
		`
		SELECT
			date(started_at) AS day,
			COALESCE(title, '') AS title,
			COALESCE(SUM(duration), 0) AS work_duration
		FROM sessions
		WHERE type = 'work'
		  AND date(started_at) BETWEEN ? AND ?
		GROUP BY day, title
		ORDER BY day, work_duration DESC;
		`,
		fromStr, toStr,
	); err != nil {
		return nil, err
	}

	return stats, nil
}

// GetLastMonthsStats retrieves daily work duration statistics for the past specified number of months.
func (r *SessionRepo) GetLastMonthsStats(numberOfMonths int) ([]DailyStat, error) {
	today := time.Now()
	firstDay := today.AddDate(0, -numberOfMonths, -today.Day()+1)

	return r.getDailyStats(firstDay, today)
}

// GetStreakStats calculates the current and best streaks of consecutive work days.
// A streak is consecutive days with at least one 'work' session.
func (r *SessionRepo) GetStreakStats() (StreakStats, error) {
	var dates []string

	if err := r.db.Select(
		&dates,
		`
		SELECT DISTINCT date(started_at) AS day
		FROM sessions
		WHERE type = 'work'
		ORDER BY day DESC;
		`,
	); err != nil {
		return StreakStats{}, err
	}

	return calculateStreak(dates), nil
}

// retrieves daily work duration statistics between the specified dates.
// from and to are inclusive.
// The results are normalized to include all days in the range.
func (r *SessionRepo) getDailyStats(from, to time.Time) ([]DailyStat, error) {
	fromStr := from.Format(DateFormat)
	toStr := to.Format(DateFormat)

	var stats []DailyStat

	if err := r.db.Select(
		&stats,
		`
		SELECT
			date(started_at) AS day,
			COALESCE(SUM(duration * (type = 'work')), 0) AS work_duration
		FROM sessions
		WHERE date(started_at) BETWEEN ? AND ?
		GROUP BY day
		ORDER BY day;
		`,
		fromStr, toStr,
	); err != nil {
		return nil, err
	}

	return r.normalizeStats(from, to, stats), nil
}

// ensures that there is a DailyStat entry for each day
func (r *SessionRepo) normalizeStats(from, to time.Time, stats []DailyStat) []DailyStat {
	m := make(map[string]DailyStat)

	for _, stat := range stats {
		m[stat.Date] = stat
	}

	var normalized []DailyStat
	current := from
	for !current.After(to) {
		day := current.Format(DateFormat)

		normalized = append(normalized, DailyStat{
			Date:         day,
			WorkDuration: m[day].WorkDuration,
		})

		current = current.AddDate(0, 0, 1) // next day
	}

	return normalized
}
