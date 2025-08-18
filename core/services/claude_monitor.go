package services

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/majiayu000/anywhere-ai/core/database"
	"github.com/majiayu000/anywhere-ai/core/tmux"
	"github.com/majiayu000/anywhere-ai/core/tools"
)

// ClaudeMonitor monitors Claude sessions and converts output to messages
type ClaudeMonitor struct {
	tmuxManager    *tmux.Manager
	messageService *MessageService
	wsService      *TerminalWebSocketService
	adapter        *tools.ClaudeAdapter
	sessions       map[string]*ClaudeSessionState
	mu             sync.RWMutex
}

// ClaudeSessionState tracks the state of a Claude session
type ClaudeSessionState struct {
	SessionID          string
	LastOutput         string
	LastProcessedLine  int
	LastUserInput      string
	LastClaudeResponse string
	IsWaitingForInput  bool
	LastActivity       time.Time
	Context            context.Context
	Cancel             context.CancelFunc
}

// NewClaudeMonitor creates a new Claude monitor
func NewClaudeMonitor(tmuxManager *tmux.Manager, messageService *MessageService, wsService *TerminalWebSocketService) *ClaudeMonitor {
	return &ClaudeMonitor{
		tmuxManager:    tmuxManager,
		messageService: messageService,
		wsService:      wsService,
		adapter:        tools.NewClaudeAdapter(),
		sessions:       make(map[string]*ClaudeSessionState),
	}
}

// StartMonitoring starts monitoring a Claude session
func (m *ClaudeMonitor) StartMonitoring(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing monitor if any
	if state, exists := m.sessions[sessionID]; exists {
		state.Cancel()
	}

	// Create new monitoring context
	ctx, cancel := context.WithCancel(context.Background())
	state := &ClaudeSessionState{
		SessionID:         sessionID,
		LastOutput:        "",
		LastProcessedLine: 0,
		IsWaitingForInput: false,
		LastActivity:      time.Now(),
		Context:           ctx,
		Cancel:            cancel,
	}
	m.sessions[sessionID] = state

	// Start monitoring goroutine
	go m.monitorSession(state)
}

// StopMonitoring stops monitoring a Claude session
func (m *ClaudeMonitor) StopMonitoring(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.sessions[sessionID]; exists {
		state.Cancel()
		delete(m.sessions, sessionID)
	}
}

// monitorSession monitors a single Claude session
func (m *ClaudeMonitor) monitorSession(state *ClaudeSessionState) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-state.Context.Done():
			return
		case <-ticker.C:
			m.processSessionOutput(state)
		}
	}
}

// processSessionOutput processes new output from a Claude session
func (m *ClaudeMonitor) processSessionOutput(state *ClaudeSessionState) {
	// Capture current output
	output, err := m.tmuxManager.CaptureOutput(state.Context, state.SessionID)
	if err != nil {
		log.Printf("Failed to capture output for session %s: %v", state.SessionID, err)
		return
	}

	// Check if there's new output
	if output == state.LastOutput {
		return
	}

	// Parse session state using adapter
	sessionState := m.adapter.ParseOutput(output)
	
	// Check for permission prompts
	if m.adapter.IsPermissionPrompt(output) {
		m.handlePermissionPrompt(state, output)
	}

	// Extract and process new content
	lines := strings.Split(output, "\n")
	newLines := lines[state.LastProcessedLine:]
	
	if len(newLines) > 0 {
		// Check for user input
		if userInput, found := m.extractUserInput(newLines, state); found {
			m.createUserMessage(state, userInput)
		}

		// Check for Claude response
		if claudeResponse, found := m.extractClaudeResponse(newLines, state, sessionState == tools.StateWaitingInput); found {
			m.createClaudeMessage(state, claudeResponse, sessionState == tools.StateWaitingInput)
		}
	}

	// Update state
	state.LastOutput = output
	state.LastProcessedLine = len(lines)
	state.LastActivity = time.Now()
	state.IsWaitingForInput = (sessionState == tools.StateWaitingInput)
}

// extractUserInput extracts user input from new lines
func (m *ClaudeMonitor) extractUserInput(lines []string, state *ClaudeSessionState) (string, bool) {
	// Look for patterns that indicate user input
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and system output
		if trimmed == "" || strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, ">") {
			continue
		}
		
		// Check if this looks like user input (not Claude output)
		if !m.looksLikeClaudeOutput(trimmed) && trimmed != state.LastUserInput {
			state.LastUserInput = trimmed
			return trimmed, true
		}
	}
	
	return "", false
}

// extractClaudeResponse extracts Claude's response from new lines
func (m *ClaudeMonitor) extractClaudeResponse(lines []string, state *ClaudeSessionState, requiresInput bool) (string, bool) {
	responseBuilder := strings.Builder{}
	foundResponse := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines at the start
		if !foundResponse && trimmed == "" {
			continue
		}
		
		// Check if this looks like Claude output
		if m.looksLikeClaudeOutput(trimmed) {
			foundResponse = true
			responseBuilder.WriteString(trimmed)
			responseBuilder.WriteString("\n")
		} else if foundResponse && m.looksLikeEndOfResponse(trimmed) {
			// End of response
			break
		}
	}
	
	response := strings.TrimSpace(responseBuilder.String())
	if response != "" && response != state.LastClaudeResponse {
		state.LastClaudeResponse = response
		return response, true
	}
	
	return "", false
}

// looksLikeClaudeOutput checks if a line looks like Claude's output
func (m *ClaudeMonitor) looksLikeClaudeOutput(line string) bool {
	// Common Claude response patterns
	claudePatterns := []string{
		"I'll", "I will", "I can", "I'm", "I am",
		"Let me", "Let's", "Here's", "Here is",
		"This", "That", "These", "Those",
		"The", "To", "For", "Yes", "No",
		"Sure", "Certainly", "Of course",
	}
	
	for _, pattern := range claudePatterns {
		if strings.HasPrefix(line, pattern) {
			return true
		}
	}
	
	// Check for markdown or code
	if strings.HasPrefix(line, "```") || 
	   strings.HasPrefix(line, "#") ||
	   strings.HasPrefix(line, "-") ||
	   strings.HasPrefix(line, "*") ||
	   strings.HasPrefix(line, "    ") { // Indented code
		return true
	}
	
	// Check for tool usage patterns
	if strings.Contains(line, "Using tool:") ||
	   strings.Contains(line, "Reading") ||
	   strings.Contains(line, "Writing") ||
	   strings.Contains(line, "Running") {
		return true
	}
	
	return false
}

// looksLikeEndOfResponse checks if a line indicates end of Claude's response
func (m *ClaudeMonitor) looksLikeEndOfResponse(line string) bool {
	endPatterns := []string{
		">", "$", "What would you like",
		"How can I help", "Is there anything else",
		"Let me know if", "Feel free to ask",
	}
	
	for _, pattern := range endPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}
	
	return false
}

// handlePermissionPrompt handles permission prompts from Claude
func (m *ClaudeMonitor) handlePermissionPrompt(state *ClaudeSessionState, output string) {
	// Extract permission prompt details
	prompt := m.extractPermissionPrompt(output)
	if prompt == "" {
		return
	}
	
	// Create a message for the permission prompt
	message, err := m.messageService.CreateAgentMessage(
		state.Context,
		state.SessionID,
		prompt,
		true, // Requires user input
	)
	if err != nil {
		log.Printf("Failed to create permission message: %v", err)
		return
	}
	
	// Broadcast to WebSocket clients
	m.wsService.BroadcastMessage(state.SessionID, message)
}

// extractPermissionPrompt extracts permission prompt text
func (m *ClaudeMonitor) extractPermissionPrompt(output string) string {
	lines := strings.Split(output, "\n")
	promptBuilder := strings.Builder{}
	inPrompt := false
	
	for _, line := range lines {
		if strings.Contains(line, "Do you want") || 
		   strings.Contains(line, "Would you like to proceed") {
			inPrompt = true
		}
		
		if inPrompt {
			promptBuilder.WriteString(line)
			promptBuilder.WriteString("\n")
			
			// Check if we've reached the options
			if strings.Contains(line, "3.") || strings.Contains(line, "No") {
				break
			}
		}
	}
	
	return strings.TrimSpace(promptBuilder.String())
}

// createUserMessage creates a user message in the database
func (m *ClaudeMonitor) createUserMessage(state *ClaudeSessionState, content string) {
	message, err := m.messageService.CreateUserMessage(
		state.Context,
		state.SessionID,
		content,
		true, // Mark as read
	)
	if err != nil {
		log.Printf("Failed to create user message: %v", err)
		return
	}
	
	// Broadcast to WebSocket clients
	m.wsService.BroadcastMessage(state.SessionID, message)
}

// createClaudeMessage creates a Claude message in the database
func (m *ClaudeMonitor) createClaudeMessage(state *ClaudeSessionState, content string, requiresInput bool) {
	message, err := m.messageService.CreateAgentMessage(
		state.Context,
		state.SessionID,
		content,
		requiresInput,
	)
	if err != nil {
		log.Printf("Failed to create Claude message: %v", err)
		return
	}
	
	// Broadcast to WebSocket clients
	m.wsService.BroadcastMessage(state.SessionID, message)
}

// BroadcastMessage is a helper for WebSocket service to broadcast messages
func (s *TerminalWebSocketService) BroadcastMessage(sessionID string, message *database.TerminalMessage) {
	msg := WebSocketMessage{
		Action:    "newMessage",
		SessionID: sessionID,
		Type:      "message",
		Data:      message,
	}
	data, _ := json.Marshal(msg)
	s.hub.broadcast <- data
}