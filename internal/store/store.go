// Package store provides persistence for pipeline runs (SQLite or MySQL).
// New opens the DB and runs migrations (creates the runs table if not exist).
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

// Run represents a single pipeline run stored in the runs table.
// Status is one of: pending, running, success, failed.
type Run struct {
	ID          int64      `json:"id"`
	AppID       string     `json:"app_id"`
	TriggeredBy string     `json:"triggered_by,omitempty"`
	Status      string     `json:"status"` // pending, running, success, failed
	CommitSHA   string     `json:"commit_sha,omitempty"`
	Log         string     `json:"log,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}

// User represents a user and the groups they belong to.
type User struct {
	ID           int64   `json:"id"`
	Username     string  `json:"username"`
	PasswordHash string  `json:"-"`
	GroupIDs     []int64 `json:"group_ids"`
	IsAdmin      bool    `json:"is_admin"`
}

// Group represents a group.
type Group struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Store holds the DB connection and is the single entry point for all DB operations.
type Store struct {
	db     *sql.DB
	driver string
}

// New opens the database and runs migrations. driver is "sqlite3" or "mysql".
// For sqlite3, dsn is the file path (e.g. "data/cicd.db"). For mysql, dsn is the connection string (e.g. "user:password@tcp(host:3306)/dbname?parseTime=true").
func New(driver, dsn string) (*Store, error) {
	if driver == "" {
		driver = "sqlite3"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db, driver); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, driver: driver}, nil
}

func (s *Store) nowExpr() string {
	if s.driver == "mysql" {
		return "NOW()"
	}
	return "datetime('now')"
}

// migrate creates the runs table and indexes if they do not exist.
func migrate(db *sql.DB, driver string) error {
	if driver == "mysql" {
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS runs (
				id BIGINT AUTO_INCREMENT PRIMARY KEY,
				app_id VARCHAR(255) NOT NULL,
				triggered_by VARCHAR(255),
				status VARCHAR(50) NOT NULL,
				commit_sha VARCHAR(255),
				log TEXT,
				started_at DATETIME NOT NULL,
				ended_at DATETIME NULL
			);
		`)
		if err != nil {
			return err
		}
		_, _ = db.Exec(`ALTER TABLE runs ADD COLUMN triggered_by VARCHAR(255)`)
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS users (
				id BIGINT AUTO_INCREMENT PRIMARY KEY,
				username VARCHAR(255) NOT NULL UNIQUE,
				password_hash VARCHAR(255) NOT NULL,
				is_admin TINYINT(1) NOT NULL DEFAULT 0
			);
		`)
		if err != nil {
			return err
		}
		_, _ = db.Exec(`ALTER TABLE users ADD COLUMN is_admin TINYINT(1) NOT NULL DEFAULT 0`)
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS groups (
				id BIGINT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(255) NOT NULL UNIQUE
			);
		`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS user_groups (
				user_id BIGINT NOT NULL,
				group_id BIGINT NOT NULL,
				PRIMARY KEY (user_id, group_id)
			);
		`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS app_groups (
				app_id VARCHAR(255) NOT NULL,
				group_id BIGINT NOT NULL,
				PRIMARY KEY (app_id, group_id)
			);
		`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`CREATE INDEX idx_runs_app_id ON runs(app_id)`)
		if err != nil {
			// ignore if exists
		}
		_, err = db.Exec(`CREATE INDEX idx_runs_started_at ON runs(started_at)`)
		if err != nil {
			// ignore if exists
		}
		return nil
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id TEXT NOT NULL,
			triggered_by TEXT,
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
			password_hash TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);
		CREATE TABLE IF NOT EXISTS user_groups (
			user_id INTEGER NOT NULL,
			group_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, group_id)
		);
		CREATE TABLE IF NOT EXISTS app_groups (
			app_id TEXT NOT NULL,
			group_id INTEGER NOT NULL,
			PRIMARY KEY (app_id, group_id)
		);
	`)
	if err == nil {
		_, _ = db.Exec(`ALTER TABLE runs ADD COLUMN triggered_by TEXT`)
		_, _ = db.Exec(`ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`)
	}
	return err
}

// CreateRun inserts a new run and returns its ID.
func (s *Store) CreateRun(appID, commitSHA, triggeredBy string) (int64, error) {
	query := fmt.Sprintf(`INSERT INTO runs (app_id, triggered_by, status, commit_sha, started_at) VALUES (?, ?, 'pending', ?, %s)`, s.nowExpr())
	res, err := s.db.Exec(query, appID, triggeredBy, commitSHA)
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
		query := fmt.Sprintf(`UPDATE runs SET status = ?, log = ?, ended_at = %s WHERE id = ?`, s.nowExpr())
		_, err := s.db.Exec(query, status, log, id)
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
		SELECT id, app_id, COALESCE(triggered_by,''), status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
		FROM runs WHERE id = ?
	`, id).Scan(&r.ID, &r.AppID, &r.TriggeredBy, &r.Status, &r.CommitSHA, &r.Log, &r.StartedAt, &endedAt)
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
			SELECT id, app_id, COALESCE(triggered_by,''), status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
			FROM runs WHERE app_id = ? ORDER BY started_at DESC LIMIT ? OFFSET ?
		`, appID, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, app_id, COALESCE(triggered_by,''), status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
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
		if err := rows.Scan(&r.ID, &r.AppID, &r.TriggeredBy, &r.Status, &r.CommitSHA, &r.Log, &r.StartedAt, &endedAt); err != nil {
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

// DeleteRunsByAppID deletes all runs for a given app.
func (s *Store) DeleteRunsByAppID(appID string) error {
	_, err := s.db.Exec(`DELETE FROM runs WHERE app_id = ?`, appID)
	return err
}

// ListRunsByAppIDs returns runs for the allowed app IDs.
func (s *Store) ListRunsByAppIDs(appIDs []string, limit, offset int) ([]Run, error) {
	if len(appIDs) == 0 {
		return []Run{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(appIDs)), ",")
	args := make([]interface{}, 0, len(appIDs)+2)
	for _, appID := range appIDs {
		args = append(args, appID)
	}
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, app_id, COALESCE(triggered_by,''), status, COALESCE(commit_sha,''), COALESCE(log,''), started_at, ended_at
		FROM runs WHERE app_id IN (%s) ORDER BY started_at DESC LIMIT ? OFFSET ?
	`, placeholders)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]Run, 0)
	for rows.Next() {
		var r Run
		var endedAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.AppID, &r.TriggeredBy, &r.Status, &r.CommitSHA, &r.Log, &r.StartedAt, &endedAt); err != nil {
			return nil, err
		}
		if endedAt.Valid {
			r.EndedAt = &endedAt.Time
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// CountRunsByAppIDs returns run count for allowed app IDs.
func (s *Store) CountRunsByAppIDs(appIDs []string) (int64, error) {
	if len(appIDs) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(appIDs)), ",")
	args := make([]interface{}, 0, len(appIDs))
	for _, appID := range appIDs {
		args = append(args, appID)
	}
	var count int64
	query := fmt.Sprintf(`SELECT COUNT(*) FROM runs WHERE app_id IN (%s)`, placeholders)
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// CreateUser inserts a user and returns the generated ID.
func (s *Store) CreateUser(username, passwordHash string, isAdmin bool) (int64, error) {
	admin := 0
	if isAdmin {
		admin = 1
	}
	res, err := s.db.Exec(`INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, ?)`, username, passwordHash, admin)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetUser returns a user by ID including group IDs.
func (s *Store) GetUser(id int64) (*User, error) {
	var u User
	var isAdmin int
	err := s.db.QueryRow(`SELECT id, username, password_hash, is_admin FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &isAdmin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin == 1
	groupIDs, err := s.UserGroupIDs(u.ID)
	if err != nil {
		return nil, err
	}
	u.GroupIDs = groupIDs
	return &u, nil
}

// ListUsers lists all users including their group IDs.
func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, is_admin FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		var isAdmin int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &isAdmin); err != nil {
			return nil, err
		}
		u.IsAdmin = isAdmin == 1
		groupIDs, err := s.UserGroupIDs(u.ID)
		if err != nil {
			return nil, err
		}
		u.GroupIDs = groupIDs
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetUserByUsername returns a user by username.
func (s *Store) GetUserByUsername(username string) (*User, error) {
	var u User
	var isAdmin int
	err := s.db.QueryRow(`SELECT id, username, password_hash, is_admin FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &isAdmin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin == 1
	groupIDs, err := s.UserGroupIDs(u.ID)
	if err != nil {
		return nil, err
	}
	u.GroupIDs = groupIDs
	return &u, nil
}

// EnsureAdminUser creates the admin user if it does not exist.
func (s *Store) EnsureAdminUser(username, passwordHash string) error {
	u, err := s.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if u != nil {
		if !u.IsAdmin {
			if _, err := s.db.Exec(`UPDATE users SET is_admin = 1 WHERE id = ?`, u.ID); err != nil {
				return err
			}
		}
		return nil
	}
	_, err = s.CreateUser(username, passwordHash, true)
	return err
}

// UpdateUserPassword updates the password hash for a user.
func (s *Store) UpdateUserPassword(userID int64, passwordHash string) error {
	res, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteUser removes a user and all user-group relationships.
func (s *Store) DeleteUser(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_groups WHERE user_id = ?`, userID); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

// CreateGroup inserts a group and returns the generated ID.
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

	groups := make([]Group, 0)
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetGroup returns one group by ID.
func (s *Store) GetGroup(groupID int64) (*Group, error) {
	var g Group
	err := s.db.QueryRow(`SELECT id, name FROM groups WHERE id = ?`, groupID).Scan(&g.ID, &g.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// UserGroupIDs returns the group IDs for a user.
func (s *Store) UserGroupIDs(userID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT group_id FROM user_groups WHERE user_id = ? ORDER BY group_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetUserGroups replaces all groups for a user.
func (s *Store) SetUserGroups(userID int64, groupIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_groups WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, groupID := range groupIDs {
		if _, err := tx.Exec(`INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)`, userID, groupID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GroupUserIDs returns all user IDs in a group.
func (s *Store) GroupUserIDs(groupID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT user_id FROM user_groups WHERE group_id = ? ORDER BY user_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetGroupUsers replaces all user assignments for a group.
func (s *Store) SetGroupUsers(groupID int64, userIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_groups WHERE group_id = ?`, groupID); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if _, err := tx.Exec(`INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)`, userID, groupID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AppGroupIDs returns the group IDs that can access an app.
func (s *Store) AppGroupIDs(appID string) ([]int64, error) {
	rows, err := s.db.Query(`SELECT group_id FROM app_groups WHERE app_id = ? ORDER BY group_id`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetAppGroups replaces all groups for an app.
func (s *Store) SetAppGroups(appID string, groupIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM app_groups WHERE app_id = ?`, appID); err != nil {
		return err
	}
	for _, groupID := range groupIDs {
		if _, err := tx.Exec(`INSERT INTO app_groups (app_id, group_id) VALUES (?, ?)`, appID, groupID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GroupAppIDs returns all app IDs assigned to a group.
func (s *Store) GroupAppIDs(groupID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT app_id FROM app_groups WHERE group_id = ? ORDER BY app_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var appID string
		if err := rows.Scan(&appID); err != nil {
			return nil, err
		}
		out = append(out, appID)
	}
	return out, rows.Err()
}

// SetGroupApps replaces all app assignments for a group.
func (s *Store) SetGroupApps(groupID int64, appIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM app_groups WHERE group_id = ?`, groupID); err != nil {
		return err
	}
	for _, appID := range appIDs {
		if _, err := tx.Exec(`INSERT INTO app_groups (app_id, group_id) VALUES (?, ?)`, appID, groupID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AppIDsByUserGroupIDs returns app IDs linked to any of the provided groups.
func (s *Store) AppIDsByUserGroupIDs(groupIDs []int64) ([]string, error) {
	if len(groupIDs) == 0 {
		return []string{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(groupIDs)), ",")
	args := make([]interface{}, 0, len(groupIDs))
	for _, id := range groupIDs {
		args = append(args, id)
	}
	query := fmt.Sprintf(`SELECT DISTINCT app_id FROM app_groups WHERE group_id IN (%s) ORDER BY app_id`, placeholders)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var appID string
		if err := rows.Scan(&appID); err != nil {
			return nil, err
		}
		out = append(out, appID)
	}
	return out, rows.Err()
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
