package database

import (
	"database/sql"
	"fmt"
	"time"
	
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteDB represents the SQLite database connection
type SQLiteDB struct {
	conn *sql.DB
}

// Session represents a stored session
type Session struct {
	ID           string
	Tool         string
	DeviceID     string
	DeviceName   string
	Status       string
	CreatedAt    time.Time
	LastActivity time.Time
	Metadata     string // JSON string
}

// NewSQLiteDB creates a new SQLite database connection
func NewSQLiteDB(path string) (*SQLiteDB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	db := &SQLiteDB{conn: conn}
	if err := db.init(); err != nil {
		conn.Close()
		return nil, err
	}
	
	return db, nil
}

// init creates the necessary tables
func (db *SQLiteDB) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		tool TEXT NOT NULL,
		device_id TEXT NOT NULL,
		device_name TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		last_activity DATETIME NOT NULL,
		metadata TEXT
	);
	
	CREATE INDEX IF NOT EXISTS idx_sessions_device ON sessions(device_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
	`
	
	_, err := db.conn.Exec(schema)
	return err
}

// SaveSession saves or updates a session
func (db *SQLiteDB) SaveSession(session *Session) error {
	query := `
	INSERT OR REPLACE INTO sessions (
		id, tool, device_id, device_name, status, 
		created_at, last_activity, metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	_, err := db.conn.Exec(query,
		session.ID,
		session.Tool,
		session.DeviceID,
		session.DeviceName,
		session.Status,
		session.CreatedAt,
		session.LastActivity,
		session.Metadata,
	)
	
	return err
}

// GetSession retrieves a session by ID
func (db *SQLiteDB) GetSession(id string) (*Session, error) {
	query := `
	SELECT id, tool, device_id, device_name, status, 
	       created_at, last_activity, metadata
	FROM sessions WHERE id = ?
	`
	
	var session Session
	err := db.conn.QueryRow(query, id).Scan(
		&session.ID,
		&session.Tool,
		&session.DeviceID,
		&session.DeviceName,
		&session.Status,
		&session.CreatedAt,
		&session.LastActivity,
		&session.Metadata,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found")
	}
	
	return &session, err
}

// ListSessions lists sessions with optional filters
func (db *SQLiteDB) ListSessions(deviceID, status string) ([]*Session, error) {
	query := `SELECT id, tool, device_id, device_name, status, 
	                 created_at, last_activity, metadata FROM sessions WHERE 1=1`
	
	var args []interface{}
	if deviceID != "" {
		query += " AND device_id = ?"
		args = append(args, deviceID)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY last_activity DESC"
	
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var sessions []*Session
	for rows.Next() {
		var s Session
		err := rows.Scan(
			&s.ID,
			&s.Tool,
			&s.DeviceID,
			&s.DeviceName,
			&s.Status,
			&s.CreatedAt,
			&s.LastActivity,
			&s.Metadata,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}
	
	return sessions, nil
}

// DeleteSession deletes a session
func (db *SQLiteDB) DeleteSession(id string) error {
	_, err := db.conn.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

// Close closes the database connection
func (db *SQLiteDB) Close() error {
	return db.conn.Close()
}