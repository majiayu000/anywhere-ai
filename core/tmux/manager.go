package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Manager manages tmux sessions for AI tools
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// Session represents a tmux session
type Session struct {
	ID         string
	Name       string
	Tool       string // "claude", "gemini", "cursor", etc.
	WindowID   string
	PaneID     string
	Created    time.Time
	LastActive time.Time
	Status     string // "active", "detached", "terminated"
	DeviceID   string
	DeviceName string
}

// NewManager creates a new tmux manager
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new tmux session for an AI tool
func (m *Manager) CreateSession(ctx context.Context, tool string, sessionName string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate unique session name if not provided
	if sessionName == "" {
		sessionName = fmt.Sprintf("%s-%d", tool, time.Now().Unix())
	}

	// Create tmux session
	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", sessionName, "-n", tool)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Get session info
	session := &Session{
		ID:         sessionName,
		Name:       sessionName,
		Tool:       tool,
		Created:    time.Now(),
		LastActive: time.Now(),
		Status:     "active",
	}

	// Get window and pane IDs
	if err := m.updateSessionInfo(ctx, session); err != nil {
		// Clean up on error
		exec.CommandContext(ctx, "tmux", "kill-session", "-t", sessionName).Run()
		return nil, err
	}

	m.sessions[sessionName] = session
	return session, nil
}

// AttachSession attaches to an existing tmux session
func (m *Manager) AttachSession(ctx context.Context, sessionID string) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Check if session still exists in tmux
	if !m.isSessionAlive(ctx, sessionID) {
		m.mu.Lock()
		session.Status = "terminated"
		m.mu.Unlock()
		return fmt.Errorf("session %s is no longer active", sessionID)
	}

	// Attach to session (this will replace current terminal)
	cmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", sessionID)
	return cmd.Run()
}

// DetachSession detaches from a tmux session (keeps it running)
func (m *Manager) DetachSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.Status = "detached"
	session.LastActive = time.Now()
	return nil
}

// SendCommand sends a command to a tmux session
func (m *Manager) SendCommand(ctx context.Context, sessionID string, command string) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Send keys to tmux pane
	var cmd *exec.Cmd
	if command == "" {
		// Just send Enter key
		cmd = exec.CommandContext(ctx, "tmux", "send-keys", "-t", session.PaneID, "Enter")
	} else {
		// Send command followed by Enter
		cmd = exec.CommandContext(ctx, "tmux", "send-keys", "-t", session.PaneID, command, "Enter")
	}
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send command: %w", err)
	}

	// Update last active time
	m.mu.Lock()
	session.LastActive = time.Now()
	m.mu.Unlock()

	return nil
}

// SendLiteralInput sends literal input to a tmux session (for Claude)
func (m *Manager) SendLiteralInput(ctx context.Context, sessionID string, input string) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Use tmux send-keys with -l flag for literal input
	// This is better for Claude which expects actual text input
	cmd := exec.CommandContext(ctx, "tmux", "send-keys", "-l", "-t", session.PaneID, input)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send literal input: %w", err)
	}
	
	// Send Enter key separately
	enterCmd := exec.CommandContext(ctx, "tmux", "send-keys", "-t", session.PaneID, "Enter")
	if err := enterCmd.Run(); err != nil {
		return fmt.Errorf("failed to send Enter key: %w", err)
	}

	// Update last active time
	m.mu.Lock()
	session.LastActive = time.Now()
	m.mu.Unlock()

	return nil
}

// CaptureOutput captures the current output from a tmux session
func (m *Manager) CaptureOutput(ctx context.Context, sessionID string) (string, error) {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("session %s not found", sessionID)
	}

	// Capture pane content
	cmd := exec.CommandContext(ctx, "tmux", "capture-pane", "-t", session.PaneID, "-p")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture output: %w", err)
	}

	return string(output), nil
}

// ListSessions lists all active tmux sessions
func (m *Manager) ListSessions(ctx context.Context) ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Update session status
	for _, session := range m.sessions {
		if !m.isSessionAlive(ctx, session.ID) {
			session.Status = "terminated"
		}
	}

	// Return active sessions
	var sessions []*Session
	for _, session := range m.sessions {
		if session.Status != "terminated" {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// KillSession terminates a tmux session
func (m *Manager) KillSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Kill tmux session
	cmd := exec.CommandContext(ctx, "tmux", "kill-session", "-t", sessionID)
	if err := cmd.Run(); err != nil {
		// Session might already be dead
		if !strings.Contains(err.Error(), "can't find session") {
			return fmt.Errorf("failed to kill session: %w", err)
		}
	}

	session.Status = "terminated"
	delete(m.sessions, sessionID)
	return nil
}

// RestoreSession restores a session from stored state
func (m *Manager) RestoreSession(ctx context.Context, session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already exists
	if m.isSessionAlive(ctx, session.ID) {
		// Session already exists, just update our records
		m.sessions[session.ID] = session
		return m.updateSessionInfo(ctx, session)
	}

	// Recreate session
	cmd := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", session.ID, "-n", session.Tool)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore session: %w", err)
	}

	// Update session info
	if err := m.updateSessionInfo(ctx, session); err != nil {
		return err
	}

	session.Status = "detached"
	m.sessions[session.ID] = session
	return nil
}

// MonitorOutput monitors a tmux session for output changes
func (m *Manager) MonitorOutput(ctx context.Context, sessionID string, callback func(string)) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastOutput string
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			output, err := m.CaptureOutput(ctx, sessionID)
			if err != nil {
				return err
			}
			if output != lastOutput {
				callback(output)
				lastOutput = output
			}
		}
	}
}

// Helper functions

func (m *Manager) isSessionAlive(ctx context.Context, sessionID string) bool {
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", sessionID)
	return cmd.Run() == nil
}

func (m *Manager) updateSessionInfo(ctx context.Context, session *Session) error {
	// Get window ID
	cmd := exec.CommandContext(ctx, "tmux", "list-windows", "-t", session.ID, "-F", "#{window_id}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get window ID: %w", err)
	}
	session.WindowID = strings.TrimSpace(string(output))

	// Get pane ID
	cmd = exec.CommandContext(ctx, "tmux", "list-panes", "-t", session.ID, "-F", "#{pane_id}")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get pane ID: %w", err)
	}
	session.PaneID = strings.TrimSpace(string(output))

	return nil
}