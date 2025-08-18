package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// JSONLMonitor monitors Claude's JSONL log files for precise message extraction
type JSONLMonitor struct {
	messageService *MessageService
	wsService      *TerminalWebSocketService
	sessions       map[string]*JSONLSessionState
	mu             sync.RWMutex
}

// JSONLSessionState tracks JSONL monitoring for a session
type JSONLSessionState struct {
	SessionID    string
	LogFilePath  string
	LastPosition int64
	Context      context.Context
	Cancel       context.CancelFunc
}

// ClaudeLogEntry represents a Claude JSONL log entry
type ClaudeLogEntry struct {
	Type      string    `json:"type"`
	Message   Message   `json:"message"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
}

// Message represents the message content in JSONL
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []ContentBlock
}

// ContentBlock represents a content block (text, tool_use, etc.)
type ContentBlock struct {
	Type string      `json:"type"`
	Text string      `json:"text,omitempty"`
	Name string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
}

// NewJSONLMonitor creates a new JSONL monitor
func NewJSONLMonitor(messageService *MessageService, wsService *TerminalWebSocketService) *JSONLMonitor {
	return &JSONLMonitor{
		messageService: messageService,
		wsService:      wsService,
		sessions:       make(map[string]*JSONLSessionState),
	}
}

// StartMonitoring starts monitoring JSONL for a session
func (m *JSONLMonitor) StartMonitoring(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing monitor if any
	if state, exists := m.sessions[sessionID]; exists {
		state.Cancel()
	}

	// Wait for the new Claude session to create its own JSONL file
	// Instead of finding the latest file, we'll wait for a new one
	logPath, err := m.waitForNewClaudeLogFile(sessionID)
	if err != nil {
		return fmt.Errorf("failed to find new Claude log file: %w", err)
	}

	// Create monitoring context
	ctx, cancel := context.WithCancel(context.Background())
	state := &JSONLSessionState{
		SessionID:    sessionID,
		LogFilePath:  logPath,
		LastPosition: 0, // Start from beginning of the new file
		Context:      ctx,
		Cancel:       cancel,
	}
	m.sessions[sessionID] = state

	// Start monitoring in goroutine
	go m.monitorJSONLFile(state)

	return nil
}

// StopMonitoring stops monitoring JSONL for a session
func (m *JSONLMonitor) StopMonitoring(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, exists := m.sessions[sessionID]; exists {
		state.Cancel()
		delete(m.sessions, sessionID)
	}
}

// waitForNewClaudeLogFile waits for a new Claude JSONL file to be created
func (m *JSONLMonitor) waitForNewClaudeLogFile(sessionID string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Get current working directory and convert to Claude's project name format
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Convert path to Claude's format
	projectName := strings.ReplaceAll(cwd, "/", "-")
	projectName = strings.ReplaceAll(projectName, ".", "-")
	projectName = strings.ReplaceAll(projectName, " ", "-")

	// Claude logs are stored in ~/.claude/projects/{project-name}/
	projectDir := filepath.Join(homeDir, ".claude", "projects", projectName)

	// Record existing files before starting
	existingFiles := make(map[string]bool)
	if files, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl")); err == nil {
		for _, file := range files {
			existingFiles[file] = true
		}
	}

	// Wait up to 10 seconds for a new JSONL file to appear
	startTime := time.Now()
	for time.Since(startTime) < 10*time.Second {
		files, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Look for new files that weren't there before
		for _, file := range files {
			if !existingFiles[file] {
				// Found a new file! Wait a bit more for it to have content
				time.Sleep(1 * time.Second)
				log.Printf("Found new Claude JSONL file for session %s: %s", sessionID, file)
				return file, nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Fallback: if no new file found, use the most recent one but from a specific position
	return m.findClaudeLogFile()
}

// findClaudeLogFile finds the most recent Claude JSONL log file (fallback)
func (m *JSONLMonitor) findClaudeLogFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Get current working directory and convert to Claude's project name format
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Convert path to Claude's format
	projectName := strings.ReplaceAll(cwd, "/", "-")
	projectName = strings.ReplaceAll(projectName, ".", "-")
	projectName = strings.ReplaceAll(projectName, " ", "-")

	// Claude logs are stored in ~/.claude/projects/{project-name}/
	projectDir := filepath.Join(homeDir, ".claude", "projects", projectName)

	// Find the most recent JSONL file
	files, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no JSONL files found in %s", projectDir)
	}

	// Return the most recently modified file
	var latestFile string
	var latestTime time.Time

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = file
		}
	}

	return latestFile, nil
}

// monitorJSONLFile monitors a JSONL file for new entries
func (m *JSONLMonitor) monitorJSONLFile(state *JSONLSessionState) {
	log.Printf("Starting JSONL monitoring for session %s, file: %s", state.SessionID, state.LogFilePath)

	for {
		select {
		case <-state.Context.Done():
			return
		default:
			m.processNewEntries(state)
			time.Sleep(100 * time.Millisecond) // Check every 100ms for high responsiveness
		}
	}
}

// processNewEntries processes new entries in the JSONL file
func (m *JSONLMonitor) processNewEntries(state *JSONLSessionState) {
	file, err := os.Open(state.LogFilePath)
	if err != nil {
		return
	}
	defer file.Close()

	// Seek to last position
	file.Seek(state.LastPosition, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse JSONL entry
		var entry ClaudeLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Printf("Failed to parse JSONL entry: %v", err)
			continue
		}

		// Process the entry
		m.processLogEntry(state.SessionID, &entry)
	}

	// Update position
	pos, _ := file.Seek(0, 2) // Seek to end
	state.LastPosition = pos
}

// processLogEntry processes a single JSONL log entry
func (m *JSONLMonitor) processLogEntry(sessionID string, entry *ClaudeLogEntry) {
	ctx := context.Background()

	switch entry.Type {
	case "user":
		// Skip creating duplicate user messages, but show typing indicator
		// This means Claude has received the user message and is about to process it
		log.Printf("User message detected in JSONL - Claude is about to respond: %s", sessionID)
		m.wsService.BroadcastTypingIndicator(sessionID, true)

	case "assistant":
		// Extract Claude's response
		content := m.extractTextContent(entry.Message.Content)
		if content != "" {
			// Stop typing indicator when Claude responds
			m.wsService.BroadcastTypingIndicator(sessionID, false)
			
			// Check if this contains tool usage
			requiresInput := m.containsToolUsage(entry.Message.Content)
			
			message, err := m.messageService.CreateAgentMessage(ctx, sessionID, content, requiresInput)
			if err != nil {
				log.Printf("Failed to create agent message: %v", err)
				return
			}
			m.wsService.BroadcastMessage(sessionID, message)
		}

	case "thinking":
		// Claude is processing/thinking - show typing indicator
		log.Printf("Claude is thinking for session: %s", sessionID)
		m.wsService.BroadcastTypingIndicator(sessionID, true)
		
	case "tool_use":
		// Claude is using tools - show typing indicator with tool info
		log.Printf("Claude is using tools for session: %s", sessionID)
		m.wsService.BroadcastTypingIndicator(sessionID, true)
		
	case "processing":
		// Claude is processing - show typing indicator
		log.Printf("Claude is processing for session: %s", sessionID)
		m.wsService.BroadcastTypingIndicator(sessionID, true)

	case "summary":
		// Session started - could create a welcome message
		log.Printf("Claude session started for %s", sessionID)
	}
}

// extractTextContent extracts text content from content (string or []ContentBlock)
func (m *JSONLMonitor) extractTextContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		// Simple string content
		return v
	case []interface{}:
		// Array of content blocks
		var parts []string
		for _, item := range v {
			if block, ok := item.(map[string]interface{}); ok {
				if blockType, exists := block["type"]; exists && blockType == "text" {
					if text, exists := block["text"]; exists {
						if textStr, ok := text.(string); ok {
							parts = append(parts, textStr)
						}
					}
				} else if blockType, exists := block["type"]; exists && blockType == "tool_use" {
					if name, exists := block["name"]; exists {
						if nameStr, ok := name.(string); ok {
							toolInfo := fmt.Sprintf("🔧 Using tool: %s", nameStr)
							parts = append(parts, toolInfo)
						}
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

// containsToolUsage checks if the content contains tool usage
func (m *JSONLMonitor) containsToolUsage(content interface{}) bool {
	if contentArray, ok := content.([]interface{}); ok {
		for _, item := range contentArray {
			if block, ok := item.(map[string]interface{}); ok {
				if blockType, exists := block["type"]; exists && blockType == "tool_use" {
					return true
				}
			}
		}
	}
	return false
}