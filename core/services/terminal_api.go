package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/majiayu000/anywhere-ai/core/tmux"
)

// TerminalAPIService provides REST API for terminal management
type TerminalAPIService struct {
	tmuxManager    *tmux.Manager
	wsService      *TerminalWebSocketService
	claudeMonitor  *ClaudeMonitor
	jsonlMonitor   *JSONLMonitor
	messageService *MessageService
}

// NewTerminalAPIService creates a new terminal API service
func NewTerminalAPIService(tmuxManager *tmux.Manager, wsService *TerminalWebSocketService, claudeMonitor *ClaudeMonitor, jsonlMonitor *JSONLMonitor, messageService *MessageService) *TerminalAPIService {
	return &TerminalAPIService{
		tmuxManager:    tmuxManager,
		wsService:      wsService,
		claudeMonitor:  claudeMonitor,
		jsonlMonitor:   jsonlMonitor,
		messageService: messageService,
	}
}

// CreateSessionRequest represents a session creation request
type CreateSessionAPIRequest struct {
	Tool string `json:"tool" binding:"required"`
	Name string `json:"name"`
}

// SessionResponse represents a session in API responses
type SessionResponse struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Tool   string    `json:"tool"`
	Status string    `json:"status"`
	Created time.Time `json:"created"`
}

// CreateSession creates a new terminal session
func (s *TerminalAPIService) CreateSession(c *gin.Context) {
	var req CreateSessionAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Validate tool
	validTools := map[string]bool{
		"claude":  true,
		"gemini":  true,
		"cursor":  true,
		"copilot": true,
	}

	if !validTools[req.Tool] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tool"})
		return
	}

	// Generate unique session name if not provided or if name exists
	sessionName := req.Name
	if sessionName == "" {
		sessionName = fmt.Sprintf("%s-%d", req.Tool, time.Now().Unix())
	}

	// Create tmux session
	ctx := context.Background()
	session, err := s.tmuxManager.CreateSession(ctx, req.Tool, sessionName)
	if err != nil {
		// If session name exists, try with timestamp
		sessionName = fmt.Sprintf("%s-%d", req.Tool, time.Now().Unix())
		session, err = s.tmuxManager.CreateSession(ctx, req.Tool, sessionName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create session: %v", err)})
			return
		}
	}

	// Start the actual tool in the tmux session
	toolCmd := ""
	switch req.Tool {
	case "claude":
		// Start Claude Code CLI directly without proxy
		toolCmd = `claude`
	case "gemini":
		// Try to start Gemini CLI
		toolCmd = "gemini || echo 'Gemini not installed. Install Gemini CLI first.'"
	case "cursor":
		// Try to start Cursor CLI
		toolCmd = "cursor --cli || echo 'Cursor not installed. Install Cursor IDE first.'"
	case "copilot":
		// Try to start GitHub Copilot CLI
		toolCmd = "gh copilot || echo 'GitHub Copilot CLI not installed. Install with: gh extension install github/gh-copilot'"
	default:
		// Start a plain shell
		toolCmd = "echo 'Starting shell session...'"
	}
	
	// Send initial command to tmux session
	if toolCmd != "" {
		s.tmuxManager.SendCommand(ctx, session.ID, toolCmd)
		
		// For Claude, start JSONL monitoring only (more precise)
		if req.Tool == "claude" {
			// JSONL monitoring provides precise message extraction
			if s.jsonlMonitor != nil {
				go func() {
					// Wait for Claude to create JSONL file
					time.Sleep(2 * time.Second)
					if err := s.jsonlMonitor.StartMonitoring(session.ID); err != nil {
						log.Printf("Failed to start JSONL monitoring: %v", err)
						// Fallback to tmux monitoring
						if s.claudeMonitor != nil {
							s.claudeMonitor.StartMonitoring(session.ID)
							log.Printf("Started fallback tmux monitoring for session %s", session.ID)
						}
					} else {
						log.Printf("Started JSONL monitoring for session %s", session.ID)
					}
				}()
			}
			
			go func() {
				time.Sleep(3 * time.Second) // Wait for Claude to start
				// Send Tab key to bypass permissions
				exec.Command("tmux", "send-keys", "-t", session.ID, "Tab").Run()
			}()
		}
	}

	c.JSON(http.StatusOK, SessionResponse{
		ID:      session.ID,
		Name:    session.Name,
		Tool:    req.Tool,
		Status:  session.Status,
		Created: session.Created,
	})
}

// ListSessions lists all active sessions
func (s *TerminalAPIService) ListSessions(c *gin.Context) {
	ctx := context.Background()
	sessions, err := s.tmuxManager.ListSessions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}

	response := []SessionResponse{}
	for _, session := range sessions {
		response = append(response, SessionResponse{
			ID:      session.ID,
			Name:    session.Name,
			Tool:    session.Tool,
			Status:  session.Status,
			Created: session.Created,
		})
	}

	c.JSON(http.StatusOK, response)
}

// GetSessionOutput gets the current output of a session
func (s *TerminalAPIService) GetSessionOutput(c *gin.Context) {
	sessionID := c.Param("id")
	
	ctx := context.Background()
	output, err := s.tmuxManager.CaptureOutput(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"output": output})
}

// SendSessionInput sends input to a session
func (s *TerminalAPIService) SendSessionInput(c *gin.Context) {
	sessionID := c.Param("id")
	
	var req struct {
		Input string `json:"input" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	ctx := context.Background()
	if err := s.tmuxManager.SendCommand(ctx, sessionID, req.Input); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteSession terminates a session
func (s *TerminalAPIService) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	ctx := context.Background()
	
	// Kill tmux session
	if err := s.tmuxManager.KillSession(ctx, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AttachSession attaches to an existing session
func (s *TerminalAPIService) AttachSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	ctx := context.Background()
	if err := s.tmuxManager.AttachSession(ctx, sessionID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetSessionMessages gets all messages for a session
func (s *TerminalAPIService) GetSessionMessages(c *gin.Context) {
	sessionID := c.Param("id")
	
	ctx := context.Background()
	messages, err := s.messageService.GetMessages(ctx, sessionID, 100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get messages: %v", err)})
		return
	}

	c.JSON(http.StatusOK, messages)
}

// SendSessionMessage sends a message to a session
func (s *TerminalAPIService) SendSessionMessage(c *gin.Context) {
	sessionID := c.Param("id")
	
	var req struct {
		Content string `json:"content" binding:"required"`
		Type    string `json:"type"` // "user" or "agent"
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	ctx := context.Background()
	
	// Create user message by default
	if req.Type == "" || req.Type == "user" {
		message, err := s.messageService.CreateUserMessage(ctx, sessionID, req.Content, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send message: %v", err)})
			return
		}
		
		// Send to tmux session as well
		if err := s.tmuxManager.SendLiteralInput(ctx, sessionID, req.Content); err != nil {
			log.Printf("Failed to send input to tmux: %v", err)
		}
		
		c.JSON(http.StatusOK, message)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only user messages can be sent via API"})
	}
}

// GetSessionMessageStatus gets message status for a session
func (s *TerminalAPIService) GetSessionMessageStatus(c *gin.Context) {
	sessionID := c.Param("id")
	
	ctx := context.Background()
	status, err := s.messageService.GetMessageStatus(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get message status: %v", err)})
		return
	}

	c.JSON(http.StatusOK, status)
}

// CommandInfo represents a Claude command with description
type CommandInfo struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// GetClaudeCommands fetches the latest Claude Code commands from official docs
func (s *TerminalAPIService) GetClaudeCommands(c *gin.Context) {
	// Try to fetch from official documentation
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get("https://docs.anthropic.com/en/docs/claude-code/slash-commands")
	if err != nil {
		log.Printf("Failed to fetch Claude commands from docs: %v", err)
		// Return hardcoded commands as fallback
		s.returnFallbackCommands(c)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to fetch Claude commands, status: %d", resp.StatusCode)
		s.returnFallbackCommands(c)
		return
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		s.returnFallbackCommands(c)
		return
	}
	
	// Parse HTML content to extract commands
	commands := s.parseClaudeCommands(string(body))
	if len(commands) == 0 {
		log.Printf("No commands parsed from documentation, using fallback")
		s.returnFallbackCommands(c)
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"commands": commands,
		"source":   "documentation",
		"updated":  time.Now(),
	})
}

// parseClaudeCommands extracts commands from HTML content
func (s *TerminalAPIService) parseClaudeCommands(html string) []CommandInfo {
	commands := []CommandInfo{}
	
	// Look for patterns like "/command: description" or "/command - description"
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		// Clean HTML tags (basic)
		line = strings.ReplaceAll(line, "<", "&lt;")
		line = strings.ReplaceAll(line, ">", "&gt;")
		line = strings.TrimSpace(line)
		
		if strings.Contains(line, "/") {
			// Try to match command patterns
			if idx := strings.Index(line, "/"); idx != -1 {
				rest := line[idx:]
				// Look for patterns like "/add-dir: Add additional working directories"
				parts := strings.SplitN(rest, ":", 2)
				if len(parts) == 2 {
					cmd := strings.TrimSpace(parts[0])
					desc := strings.TrimSpace(parts[1])
					if strings.HasPrefix(cmd, "/") && len(cmd) > 1 && len(desc) > 0 {
						commands = append(commands, CommandInfo{
							Command:     cmd,
							Description: desc,
						})
						continue
					}
				}
				
				// Try alternative pattern "/add-dir - Add additional working directories"
				parts = strings.SplitN(rest, " - ", 2)
				if len(parts) == 2 {
					cmd := strings.TrimSpace(parts[0])
					desc := strings.TrimSpace(parts[1])
					if strings.HasPrefix(cmd, "/") && len(cmd) > 1 && len(desc) > 0 {
						commands = append(commands, CommandInfo{
							Command:     cmd,
							Description: desc,
						})
					}
				}
			}
		}
	}
	
	return commands
}

// returnFallbackCommands returns hardcoded Claude commands
func (s *TerminalAPIService) returnFallbackCommands(c *gin.Context) {
	commands := []CommandInfo{
		{Command: "/add-dir", Description: "Add additional working directories"},
		{Command: "/agents", Description: "Manage custom AI subagents for specialized tasks"},
		{Command: "/bug", Description: "Report bugs (sends conversation to Anthropic)"},
		{Command: "/clear", Description: "Clear conversation history"},
		{Command: "/compact", Description: "Compact conversation with optional focus instructions"},
		{Command: "/config", Description: "View/modify configuration"},
		{Command: "/cost", Description: "Show token usage statistics"},
		{Command: "/doctor", Description: "Checks the health of your Claude Code installation"},
		{Command: "/help", Description: "Get usage help"},
		{Command: "/init", Description: "Initialize project with CLAUDE.md guide"},
		{Command: "/login", Description: "Switch Anthropic accounts"},
		{Command: "/logout", Description: "Sign out from your Anthropic account"},
		{Command: "/mcp", Description: "Manage MCP server connections and OAuth authentication"},
		{Command: "/memory", Description: "Edit CLAUDE.md memory files"},
		{Command: "/model", Description: "Select or change the AI model"},
		{Command: "/permissions", Description: "View or update permissions"},
		{Command: "/pr_comments", Description: "View pull request comments"},
		{Command: "/review", Description: "Request code review"},
		{Command: "/status", Description: "View account and system statuses"},
		{Command: "/terminal-setup", Description: "Install Shift+Enter key binding for newlines"},
		{Command: "/vim", Description: "Enter vim mode for alternating insert and command modes"},
	}
	
	c.JSON(http.StatusOK, gin.H{
		"commands": commands,
		"source":   "fallback",
		"updated":  time.Now(),
	})
}

// RegisterRoutes registers API routes
func (s *TerminalAPIService) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		// Claude commands endpoint
		api.GET("/claude-commands", s.GetClaudeCommands)
	}
	
	terminal := router.Group("/api/v1/terminal")
	{
		terminal.POST("/sessions", s.CreateSession)
		terminal.GET("/sessions", s.ListSessions)
		terminal.GET("/sessions/:id/output", s.GetSessionOutput)
		terminal.POST("/sessions/:id/input", s.SendSessionInput)
		terminal.DELETE("/sessions/:id", s.DeleteSession)
		terminal.POST("/sessions/:id/attach", s.AttachSession)
		
		// Message endpoints
		terminal.GET("/sessions/:id/messages", s.GetSessionMessages)
		terminal.POST("/sessions/:id/messages", s.SendSessionMessage)
		terminal.GET("/sessions/:id/messages/status", s.GetSessionMessageStatus)
	}
}