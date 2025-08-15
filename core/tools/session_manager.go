package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/majiayu000/anywhere-ai/core/tmux"
)

// ToolSession represents a tool-specific session
type ToolSession struct {
	ID           string
	Tool         ToolType
	TmuxSession  *tmux.Session
	State        SessionState
	StartedAt    time.Time
	LastActivity time.Time
	Metadata     map[string]interface{}
	OutputBuffer []string
	mu           sync.RWMutex
}

// ToolType represents different AI tools
type ToolType string

const (
	ToolClaude  ToolType = "claude"
	ToolGemini  ToolType = "gemini"
	ToolCursor  ToolType = "cursor"
	ToolCopilot ToolType = "copilot"
)

// SessionState represents the state of a tool session
type SessionState string

const (
	StateStarting     SessionState = "starting"
	StateReady        SessionState = "ready"
	StateWaitingInput SessionState = "waiting_input"
	StateProcessing   SessionState = "processing"
	StateError        SessionState = "error"
	StateStopped      SessionState = "stopped"
)

// SessionManager manages tool sessions
type SessionManager struct {
	tmuxManager *tmux.Manager
	sessions    map[string]*ToolSession
	adapters    map[ToolType]ToolAdapter
	mu          sync.RWMutex
}

// ToolAdapter interface for tool-specific implementations
type ToolAdapter interface {
	// GetCommand returns the command to start the tool
	GetCommand() []string
	
	// ParseOutput analyzes output to determine state changes
	ParseOutput(output string) SessionState
	
	// IsPermissionPrompt checks if output contains permission prompt
	IsPermissionPrompt(output string) bool
	
	// FormatInput formats user input for the tool
	FormatInput(input string) string
	
	// GetInitCommands returns commands to run after tool starts
	GetInitCommands() []string
}

// NewSessionManager creates a new session manager
func NewSessionManager(tmuxManager *tmux.Manager) *SessionManager {
	sm := &SessionManager{
		tmuxManager: tmuxManager,
		sessions:    make(map[string]*ToolSession),
		adapters:    make(map[ToolType]ToolAdapter),
	}
	
	// Register default adapters
	sm.RegisterAdapter(ToolClaude, &ClaudeAdapter{})
	sm.RegisterAdapter(ToolGemini, &GeminiAdapter{})
	sm.RegisterAdapter(ToolCursor, &CursorAdapter{})
	
	return sm
}

// RegisterAdapter registers a tool adapter
func (sm *SessionManager) RegisterAdapter(tool ToolType, adapter ToolAdapter) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.adapters[tool] = adapter
}

// CreateSession creates a new tool session
func (sm *SessionManager) CreateSession(ctx context.Context, tool ToolType, sessionName string) (*ToolSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	adapter, exists := sm.adapters[tool]
	if !exists {
		return nil, fmt.Errorf("no adapter for tool: %s", tool)
	}
	
	// Create tmux session
	tmuxSession, err := sm.tmuxManager.CreateSession(ctx, string(tool), sessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to create tmux session: %w", err)
	}
	
	// Start the tool in tmux
	command := adapter.GetCommand()
	if len(command) > 0 {
		cmdStr := ""
		for i, part := range command {
			if i > 0 {
				cmdStr += " "
			}
			cmdStr += part
		}
		if err := sm.tmuxManager.SendCommand(ctx, tmuxSession.ID, cmdStr); err != nil {
			sm.tmuxManager.KillSession(ctx, tmuxSession.ID)
			return nil, fmt.Errorf("failed to start tool: %w", err)
		}
	}
	
	// Create tool session
	session := &ToolSession{
		ID:           tmuxSession.ID,
		Tool:         tool,
		TmuxSession:  tmuxSession,
		State:        StateStarting,
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
		Metadata:     make(map[string]interface{}),
		OutputBuffer: []string{},
	}
	
	sm.sessions[session.ID] = session
	
	// Run init commands
	go sm.initializeSession(ctx, session, adapter)
	
	return session, nil
}

// GetSession gets a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*ToolSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	
	return session, nil
}

// SendInput sends input to a tool session
func (sm *SessionManager) SendInput(ctx context.Context, sessionID string, input string) error {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}
	
	adapter := sm.adapters[session.Tool]
	formattedInput := adapter.FormatInput(input)
	
	if err := sm.tmuxManager.SendCommand(ctx, sessionID, formattedInput); err != nil {
		return fmt.Errorf("failed to send input: %w", err)
	}
	
	session.mu.Lock()
	session.LastActivity = time.Now()
	session.State = StateProcessing
	session.mu.Unlock()
	
	return nil
}

// MonitorSession monitors a session for output changes
func (sm *SessionManager) MonitorSession(ctx context.Context, sessionID string, callback func(*ToolSession, string)) error {
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return err
	}
	
	adapter := sm.adapters[session.Tool]
	
	return sm.tmuxManager.MonitorOutput(ctx, sessionID, func(output string) {
		session.mu.Lock()
		defer session.mu.Unlock()
		
		// Update output buffer
		session.OutputBuffer = append(session.OutputBuffer, output)
		if len(session.OutputBuffer) > 1000 {
			session.OutputBuffer = session.OutputBuffer[len(session.OutputBuffer)-1000:]
		}
		
		// Parse state from output
		newState := adapter.ParseOutput(output)
		if newState != session.State {
			session.State = newState
			session.LastActivity = time.Now()
		}
		
		// Check for permission prompts
		if adapter.IsPermissionPrompt(output) {
			session.Metadata["permission_prompt"] = true
		}
		
		callback(session, output)
	})
}

// ListSessions lists all active sessions
func (sm *SessionManager) ListSessions() []*ToolSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	var sessions []*ToolSession
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	
	return sessions
}

// StopSession stops a tool session
func (sm *SessionManager) StopSession(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	
	if err := sm.tmuxManager.KillSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to kill tmux session: %w", err)
	}
	
	session.State = StateStopped
	delete(sm.sessions, sessionID)
	
	return nil
}

// Helper functions

func (sm *SessionManager) initializeSession(ctx context.Context, session *ToolSession, adapter ToolAdapter) {
	// Wait for tool to start
	time.Sleep(2 * time.Second)
	
	// Run init commands
	for _, cmd := range adapter.GetInitCommands() {
		if err := sm.tmuxManager.SendCommand(ctx, session.ID, cmd); err != nil {
			session.mu.Lock()
			session.State = StateError
			session.mu.Unlock()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	
	session.mu.Lock()
	session.State = StateReady
	session.mu.Unlock()
}