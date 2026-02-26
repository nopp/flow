// Package store provides SQLite persistence for pipeline runs and for seed data (users, groups).
// New opens the DB and runs migrations (creates tables if not exist).
// The runs table is used by the API; users/groups/app_groups/user_groups are used only by the seed.
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

// migrate creates tables (runs, users, groups, user_groups, app_groups) and indexes if they do not exist.
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

		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);
		CREATE TABLE IF NOT EXISTS user_groups (
			user_id INTEGER NOT NULL,
			group_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, group_id),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (group_id) REFERENCES groups(id)
		);
		CREATE TABLE IF NOT EXISTS app_groups (
			app_id TEXT NOT NULL,
			group_id INTEGER NOT NULL,
			PRIMARY KEY (app_id, group_id),
			FOREIGN KEY (group_id) REFERENCES groups(id)
		);
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

// ListRuns returns runs, optionally filtered by appID, limit.
func (s *Store) ListRuns(appID string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if appID != "" {
		rows, err = s.db.Query(`
			SELECT id, app_id, status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
			FROM runs WHERE app_id = ? ORDER BY started_at DESC LIMIT ?
		`, appID, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT id, app_id, status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
			FROM runs ORDER BY started_at DESC LIMIT ?
		`, limit)
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

// User represents a user.
type User struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	GroupIDs []int64 `json:"group_ids,omitempty"`
}

// Group represents a group.
type Group struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CreateUser creates a user and returns ID. passwordHash must be bcrypt hash.
func (s *Store) CreateUser(username, passwordHash string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, passwordHash)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UserByID returns user by ID.
func (s *Store) UserByID(id int64) (*User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, username FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.GroupIDs, _ = s.UserGroupIDs(u.ID)
	return &u, nil
}

// UserByUsername returns user by username.
func (s *Store) UserByUsername(username string) (*User, error) {
	var u User
	err := s.db.QueryRow(`SELECT id, username FROM users WHERE username = ?`, username).Scan(&u.ID, &u.Username)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.GroupIDs, _ = s.UserGroupIDs(u.ID)
	return &u, nil
}

// UserPasswordHash returns password hash for user id.
func (s *Store) UserPasswordHash(userID int64) (string, error) {
	var h string
	err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&h)
	return h, err
}

// UserGroupIDs returns group IDs for a user.
func (s *Store) UserGroupIDs(userID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT group_id FROM user_groups WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetUserGroups sets the groups for a user (replaces existing).
func (s *Store) SetUserGroups(userID int64, groupIDs []int64) error {
	_, err := s.db.Exec(`DELETE FROM user_groups WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	for _, gid := range groupIDs {
		_, err = s.db.Exec(`INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)`, userID, gid)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListUsers returns all users with their group IDs.
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		u.GroupIDs, _ = s.UserGroupIDs(u.ID)
		users = append(users, u)
	}
	return users, rows.Err()
}

// CreateGroup creates a group and returns ID.
func (s *Store) CreateGroup(name string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO groups (name) VALUES (?)`, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListGroups returns all groups.
func (s *Store) ListGroups() ([]Group, error) {
	rows, err := s.db.Query(`SELECT id, name FROM groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// AppGroupIDs returns group IDs for an app.
func (s *Store) AppGroupIDs(appID string) ([]int64, error) {
	rows, err := s.db.Query(`SELECT group_id FROM app_groups WHERE app_id = ?`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetAppGroups sets the groups for an app (replaces existing).
func (s *Store) SetAppGroups(appID string, groupIDs []int64) error {
	_, err := s.db.Exec(`DELETE FROM app_groups WHERE app_id = ?`, appID)
	if err != nil {
		return err
	}
	for _, gid := range groupIDs {
		_, err = s.db.Exec(`INSERT INTO app_groups (app_id, group_id) VALUES (?, ?)`, appID, gid)
		if err != nil {
			return err
		}
	}
	return nil
}

// AppIDsByUserGroupIDs returns app IDs that belong to any of the given group IDs.
func (s *Store) AppIDsByUserGroupIDs(groupIDs []int64) ([]string, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	// Build placeholders for IN clause
	args := make([]interface{}, len(groupIDs))
	for i, id := range groupIDs {
		args[i] = id
	}
	placeholders := ""
	for i := range groupIDs {
		if i > 0 {
			placeholders += ",?"
		} else {
			placeholders = "?"
		}
	}
	query := `SELECT DISTINCT app_id FROM app_groups WHERE group_id IN (` + placeholders + `)`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
