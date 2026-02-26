// Package store provides SQLite persistence for pipeline runs.
// New opens the DB and runs migrations (creates the runs table if not exist).
package store

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Run represents a single pipeline run stored in the runs table.
// Status is one of: pending, running, success, failed.
type Run struct {
	ID        int64     `json:"id"`
	AppID     string    `json:"app_id"`
	Status    string    `json:"status"` // pending, running, success, failed
	CommitSHA string    `json:"commit_sha,omitempty"`
	Log       string    `json:"log,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Store holds the SQLite connection and is the single entry point for all DB operations.
type Store struct {
	db *sql.DB
}

// New opens the database at dbPath, runs migrate to create tables, and returns the Store.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// migrate creates the runs table and indexes if they do not exist.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			status TEXT NOT NULL,
			commit_sha TEXT,
			log TEXT,
			started_at DATETIME NOT NULL,
			ended_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_runs_app_id ON runs(app_id);
		CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at);
	`)
	return err
}

// CreateRun inserts a new run and returns its ID.
func (s *Store) CreateRun(appID, commitSHA string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO runs (app_id, status, commit_sha, started_at) VALUES (?, 'pending', ?, datetime('now'))`,
		appID, commitSHA,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateRunLog updates only the log content for a run (e.g. while streaming).
func (s *Store) UpdateRunLog(id int64, log string) error {
	_, err := s.db.Exec(`UPDATE runs SET log = ? WHERE id = ?`, log, id)
	return err
}

// UpdateRunStatus sets status, log, and ended_at for a run.
func (s *Store) UpdateRunStatus(id int64, status, log string) error {
	if status == "success" || status == "failed" {
		_, err := s.db.Exec(
			`UPDATE runs SET status = ?, log = ?, ended_at = datetime('now') WHERE id = ?`,
			status, log, id,
		)
		return err
	}
	_, err := s.db.Exec(`UPDATE runs SET status = ?, log = ? WHERE id = ?`, status, log, id)
	return err
}

// GetRun returns a run by ID.
func (s *Store) GetRun(id int64) (*Run, error) {
	var r Run
	var endedAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, app_id, status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
		FROM runs WHERE id = ?
	`, id).Scan(&r.ID, &r.AppID, &r.Status, &r.CommitSHA, &r.Log, &r.StartedAt, &endedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if endedAt.Valid {
		r.EndedAt = &endedAt.Time
	}
	return &r, nil
}

// ListRuns returns runs, optionally filtered by appID, with limit and offset for pagination.
func (s *Store) ListRuns(appID string, limit, offset int) ([]Run, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var rows *sql.Rows
	var err error
	if appID != "" {
		rows, err = s.db.Query(`
			SELECT id, app_id, status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
			FROM runs WHERE app_id = ? ORDER BY started_at DESC LIMIT ? OFFSET ?
		`, appID, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, app_id, status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
			FROM runs ORDER BY started_at DESC LIMIT ? OFFSET ?
		`, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]Run, 0)
	for rows.Next() {
		var r Run
		var endedAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.AppID, &r.Status, &r.CommitSHA, &r.Log, &r.StartedAt, &endedAt); err != nil {
			return nil, err
		}
		if endedAt.Valid {
			r.EndedAt = &endedAt.Time
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// CountRuns returns the total number of runs, optionally filtered by appID.
func (s *Store) CountRuns(appID string) (int64, error) {
	var count int64
	if appID != "" {
		err := s.db.QueryRow(`SELECT COUNT(*) FROM runs WHERE app_id = ?`, appID).Scan(&count)
		return count, err
	}
	err := s.db.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&count)
	return count, err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
