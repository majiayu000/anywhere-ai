package terminal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	
	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresSessionStore implements SessionStore using PostgreSQL
type PostgresSessionStore struct {
	db *sql.DB
}

// NewPostgresSessionStore creates a new PostgreSQL session store
func NewPostgresSessionStore(connectionString string) (*PostgresSessionStore, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}
	
	// Create tables if not exists
	if err := createTables(db); err != nil {
		return nil, err
	}
	
	return &PostgresSessionStore{db: db}, nil
}

func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS terminal_sessions (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255),
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		
		-- Device info
		owner_device_id VARCHAR(255) NOT NULL,
		owner_device_name VARCHAR(255),
		current_device_id VARCHAR(255),
		last_heartbeat TIMESTAMP,
		
		-- Terminal state
		command TEXT,
		args JSONB,
		environment JSONB,
		working_dir TEXT,
		
		-- Buffer state
		buffer_content BYTEA,
		cursor_row INT,
		cursor_col INT,
		scroll_offset INT,
		
		-- Tool state
		tool_name VARCHAR(255),
		tool_state JSONB,
		
		-- Metadata
		status VARCHAR(50),
		tags JSONB,
		metadata JSONB,
		
		-- Indexes
		INDEX idx_owner_device (owner_device_id),
		INDEX idx_current_device (current_device_id),
		INDEX idx_last_heartbeat (last_heartbeat),
		INDEX idx_status (status)
	);
	
	CREATE TABLE IF NOT EXISTS session_checkpoints (
		id VARCHAR(255) PRIMARY KEY,
		session_id VARCHAR(255) REFERENCES terminal_sessions(id) ON DELETE CASCADE,
		timestamp TIMESTAMP NOT NULL,
		
		-- History
		input_history JSONB,
		output_history JSONB,
		
		-- State snapshot
		state_snapshot JSONB,
		
		INDEX idx_session_id (session_id),
		INDEX idx_timestamp (timestamp)
	);
	
	CREATE TABLE IF NOT EXISTS session_logs (
		id SERIAL PRIMARY KEY,
		session_id VARCHAR(255) REFERENCES terminal_sessions(id) ON DELETE CASCADE,
		timestamp TIMESTAMP NOT NULL,
		type VARCHAR(50), -- 'input', 'output', 'error', 'event'
		content TEXT,
		metadata JSONB,
		
		INDEX idx_session_logs (session_id, timestamp)
	);
	`
	
	_, err := db.Exec(schema)
	return err
}

// SaveSession saves a session state
func (s *PostgresSessionStore) SaveSession(session *SessionState) error {
	query := `
	INSERT INTO terminal_sessions (
		id, name, created_at, updated_at,
		owner_device_id, owner_device_name, current_device_id, last_heartbeat,
		command, args, environment, working_dir,
		buffer_content, cursor_row, cursor_col, scroll_offset,
		tool_name, tool_state,
		status, tags, metadata
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
	ON CONFLICT (id) DO UPDATE SET
		updated_at = $4,
		current_device_id = $7,
		last_heartbeat = $8,
		buffer_content = $13,
		cursor_row = $14,
		cursor_col = $15,
		scroll_offset = $16,
		tool_state = $18,
		status = $19,
		metadata = $21
	`
	
	argsJSON, _ := json.Marshal(session.Args)
	envJSON, _ := json.Marshal(session.Environment)
	toolStateJSON, _ := json.Marshal(session.ToolState)
	tagsJSON, _ := json.Marshal(session.Tags)
	metadataJSON, _ := json.Marshal(session.Metadata)
	
	_, err := s.db.Exec(query,
		session.ID, session.Name, session.CreatedAt, session.UpdatedAt,
		session.OwnerDeviceID, session.OwnerDeviceName, session.CurrentDeviceID, session.LastHeartbeat,
		session.Command, argsJSON, envJSON, session.WorkingDir,
		session.BufferContent, session.CursorRow, session.CursorCol, session.ScrollOffset,
		session.ToolName, toolStateJSON,
		session.Status, tagsJSON, metadataJSON,
	)
	
	return err
}

// LoadSession loads a session state
func (s *PostgresSessionStore) LoadSession(sessionID string) (*SessionState, error) {
	query := `
	SELECT 
		id, name, created_at, updated_at,
		owner_device_id, owner_device_name, current_device_id, last_heartbeat,
		command, args, environment, working_dir,
		buffer_content, cursor_row, cursor_col, scroll_offset,
		tool_name, tool_state,
		status, tags, metadata
	FROM terminal_sessions
	WHERE id = $1
	`
	
	var session SessionState
	var argsJSON, envJSON, toolStateJSON, tagsJSON, metadataJSON []byte
	
	err := s.db.QueryRow(query, sessionID).Scan(
		&session.ID, &session.Name, &session.CreatedAt, &session.UpdatedAt,
		&session.OwnerDeviceID, &session.OwnerDeviceName, &session.CurrentDeviceID, &session.LastHeartbeat,
		&session.Command, &argsJSON, &envJSON, &session.WorkingDir,
		&session.BufferContent, &session.CursorRow, &session.CursorCol, &session.ScrollOffset,
		&session.ToolName, &toolStateJSON,
		&session.Status, &tagsJSON, &metadataJSON,
	)
	
	if err != nil {
		return nil, err
	}
	
	// Unmarshal JSON fields
	json.Unmarshal(argsJSON, &session.Args)
	json.Unmarshal(envJSON, &session.Environment)
	json.Unmarshal(toolStateJSON, &session.ToolState)
	json.Unmarshal(tagsJSON, &session.Tags)
	json.Unmarshal(metadataJSON, &session.Metadata)
	
	return &session, nil
}

// ListSessions lists sessions based on filter
func (s *PostgresSessionStore) ListSessions(filter SessionFilter) ([]*SessionState, error) {
	query := `
	SELECT 
		id, name, created_at, updated_at,
		owner_device_id, owner_device_name, current_device_id, last_heartbeat,
		tool_name, status
	FROM terminal_sessions
	WHERE 1=1
	`
	
	args := []interface{}{}
	argCount := 0
	
	// Build dynamic query based on filter
	if filter.DeviceID != "" {
		argCount++
		query += fmt.Sprintf(" AND (owner_device_id = $%d OR current_device_id = $%d)", argCount, argCount)
		args = append(args, filter.DeviceID)
	}
	
	if filter.ToolName != "" {
		argCount++
		query += fmt.Sprintf(" AND tool_name = $%d", argCount)
		args = append(args, filter.ToolName)
	}
	
	if filter.Status != "" {
		argCount++
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, filter.Status)
	}
	
	if !filter.CreatedAfter.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND created_at > $%d", argCount)
		args = append(args, filter.CreatedAfter)
	}
	
	if !filter.CreatedBefore.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND created_at < $%d", argCount)
		args = append(args, filter.CreatedBefore)
	}
	
	if !filter.IncludeDead {
		query += fmt.Sprintf(" AND last_heartbeat > NOW() - INTERVAL '5 minutes'")
	}
	
	query += " ORDER BY updated_at DESC"
	
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var sessions []*SessionState
	
	for rows.Next() {
		var session SessionState
		err := rows.Scan(
			&session.ID, &session.Name, &session.CreatedAt, &session.UpdatedAt,
			&session.OwnerDeviceID, &session.OwnerDeviceName, 
			&session.CurrentDeviceID, &session.LastHeartbeat,
			&session.ToolName, &session.Status,
		)
		if err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}
	
	return sessions, nil
}

// DeleteSession deletes a session
func (s *PostgresSessionStore) DeleteSession(sessionID string) error {
	_, err := s.db.Exec("DELETE FROM terminal_sessions WHERE id = $1", sessionID)
	return err
}

// UpdateHeartbeat updates session heartbeat
func (s *PostgresSessionStore) UpdateHeartbeat(sessionID string, deviceID string) error {
	query := `
	UPDATE terminal_sessions 
	SET last_heartbeat = NOW(), 
	    current_device_id = $2,
	    updated_at = NOW()
	WHERE id = $1
	`
	_, err := s.db.Exec(query, sessionID, deviceID)
	return err
}

// SaveCheckpoint saves a session checkpoint
func (s *PostgresSessionStore) SaveCheckpoint(checkpoint *SessionCheckpoint) error {
	inputHistoryJSON, _ := json.Marshal(checkpoint.InputHistory)
	outputHistoryJSON, _ := json.Marshal(checkpoint.OutputHistory)
	stateSnapshotJSON, _ := json.Marshal(checkpoint.StateSnapshot)
	
	query := `
	INSERT INTO session_checkpoints (
		id, session_id, timestamp,
		input_history, output_history, state_snapshot
	) VALUES ($1, $2, $3, $4, $5, $6)
	`
	
	_, err := s.db.Exec(query,
		checkpoint.ID, checkpoint.SessionID, checkpoint.Timestamp,
		inputHistoryJSON, outputHistoryJSON, stateSnapshotJSON,
	)
	
	return err
}

// LoadCheckpoint loads a session checkpoint
func (s *PostgresSessionStore) LoadCheckpoint(checkpointID string) (*SessionCheckpoint, error) {
	query := `
	SELECT id, session_id, timestamp,
	       input_history, output_history, state_snapshot
	FROM session_checkpoints
	WHERE id = $1
	`
	
	var checkpoint SessionCheckpoint
	var inputHistoryJSON, outputHistoryJSON, stateSnapshotJSON []byte
	
	err := s.db.QueryRow(query, checkpointID).Scan(
		&checkpoint.ID, &checkpoint.SessionID, &checkpoint.Timestamp,
		&inputHistoryJSON, &outputHistoryJSON, &stateSnapshotJSON,
	)
	
	if err != nil {
		return nil, err
	}
	
	json.Unmarshal(inputHistoryJSON, &checkpoint.InputHistory)
	json.Unmarshal(outputHistoryJSON, &checkpoint.OutputHistory)
	json.Unmarshal(stateSnapshotJSON, &checkpoint.StateSnapshot)
	
	return &checkpoint, nil
}

// LogSessionEvent logs a session event
func (s *PostgresSessionStore) LogSessionEvent(sessionID string, eventType string, content string, metadata map[string]interface{}) error {
	metadataJSON, _ := json.Marshal(metadata)
	
	query := `
	INSERT INTO session_logs (session_id, timestamp, type, content, metadata)
	VALUES ($1, NOW(), $2, $3, $4)
	`
	
	_, err := s.db.Exec(query, sessionID, eventType, content, metadataJSON)
	return err
}

// GetSessionLogs gets session logs
func (s *PostgresSessionStore) GetSessionLogs(sessionID string, limit int) ([]SessionLog, error) {
	query := `
	SELECT timestamp, type, content, metadata
	FROM session_logs
	WHERE session_id = $1
	ORDER BY timestamp DESC
	LIMIT $2
	`
	
	rows, err := s.db.Query(query, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var logs []SessionLog
	
	for rows.Next() {
		var log SessionLog
		var metadataJSON []byte
		
		err := rows.Scan(&log.Timestamp, &log.Type, &log.Content, &metadataJSON)
		if err != nil {
			continue
		}
		
		json.Unmarshal(metadataJSON, &log.Metadata)
		logs = append(logs, log)
	}
	
	return logs, nil
}

// SessionLog represents a session log entry
type SessionLog struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
}