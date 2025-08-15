package terminal

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/google/uuid"
)

// Manager manages multiple terminal sessions
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	
	// Configuration
	config ManagerConfig
}

// ManagerConfig contains configuration for the terminal manager
type ManagerConfig struct {
	MaxSessions      int    // Maximum number of concurrent sessions
	DefaultRows      int    // Default terminal rows
	DefaultCols      int    // Default terminal columns
	ScrollbackBuffer int    // Lines to keep in scrollback
	AutoRestart      bool   // Auto restart on crash
	RestartDelay     int    // Seconds to wait before restart
}

// DefaultManagerConfig returns default configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxSessions:      10,
		DefaultRows:      40,
		DefaultCols:      120,
		ScrollbackBuffer: 10000,
		AutoRestart:      false,
		RestartDelay:     5,
	}
}

// NewManager creates a new terminal manager
func NewManager(config ...ManagerConfig) *Manager {
	cfg := DefaultManagerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	
	return &Manager{
		sessions: make(map[string]*Session),
		config:   cfg,
	}
}

// CreateSession creates a new terminal session
func (m *Manager) CreateSession(name string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check max sessions limit
	if len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("max sessions limit reached (%d)", m.config.MaxSessions)
	}
	
	// Generate session ID
	sessionID := uuid.New().String()
	if name != "" {
		sessionID = name + "-" + sessionID[:8]
	}
	
	// Check if session already exists
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}
	
	// Create new session
	session := NewSession(sessionID, SessionConfig{
		Rows:             m.config.DefaultRows,
		Cols:             m.config.DefaultCols,
		ScrollbackBuffer: m.config.ScrollbackBuffer,
		AutoRestart:      m.config.AutoRestart,
		RestartDelay:     m.config.RestartDelay,
	})
	
	// Store session
	m.sessions[sessionID] = session
	
	return session, nil
}

// GetSession returns a session by ID
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	
	return session, nil
}

// ListSessions returns all active sessions
func (m *Manager) ListSessions() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	sessions := make([]SessionInfo, 0, len(m.sessions))
	
	for _, session := range m.sessions {
		sessions = append(sessions, session.GetInfo())
	}
	
	return sessions
}

// DestroySession destroys a terminal session
func (m *Manager) DestroySession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}
	
	// Stop the session
	if err := session.Stop(); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}
	
	// Remove from map
	delete(m.sessions, sessionID)
	
	return nil
}

// DestroyAllSessions destroys all terminal sessions
func (m *Manager) DestroyAllSessions() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	var errors []error
	
	for id, session := range m.sessions {
		if err := session.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop session %s: %w", id, err))
		}
	}
	
	// Clear all sessions
	m.sessions = make(map[string]*Session)
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to stop some sessions: %v", errors)
	}
	
	return nil
}

// RestartSession restarts a terminal session
func (m *Manager) RestartSession(sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}
	
	return session.Restart()
}

// GetStats returns statistics about the manager
func (m *Manager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := ManagerStats{
		TotalSessions:  len(m.sessions),
		MaxSessions:    m.config.MaxSessions,
		ActiveSessions: 0,
		StoppedSessions: 0,
	}
	
	for _, session := range m.sessions {
		if session.IsRunning() {
			stats.ActiveSessions++
		} else {
			stats.StoppedSessions++
		}
	}
	
	return stats
}

// ManagerStats contains statistics about the terminal manager
type ManagerStats struct {
	TotalSessions   int `json:"total_sessions"`
	MaxSessions     int `json:"max_sessions"`
	ActiveSessions  int `json:"active_sessions"`
	StoppedSessions int `json:"stopped_sessions"`
}

// Cleanup performs cleanup operations
func (m *Manager) Cleanup() error {
	return m.DestroyAllSessions()
}

// StartWithContext starts the manager with a context
func (m *Manager) StartWithContext(ctx context.Context) {
	// Monitor context cancellation
	go func() {
		<-ctx.Done()
		m.Cleanup()
	}()
}