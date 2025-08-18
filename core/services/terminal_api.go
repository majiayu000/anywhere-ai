package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/majiayu000/anywhere-ai/core/tmux"
)

// TerminalAPIService provides REST API for terminal management
type TerminalAPIService struct {
	tmuxManager   *tmux.Manager
	wsService     *TerminalWebSocketService
	claudeMonitor *ClaudeMonitor
	jsonlMonitor  *JSONLMonitor
}

// NewTerminalAPIService creates a new terminal API service
func NewTerminalAPIService(tmuxManager *tmux.Manager, wsService *TerminalWebSocketService, claudeMonitor *ClaudeMonitor, jsonlMonitor *JSONLMonitor) *TerminalAPIService {
	return &TerminalAPIService{
		tmuxManager:   tmuxManager,
		wsService:     wsService,
		claudeMonitor: claudeMonitor,
		jsonlMonitor:  jsonlMonitor,
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

// RegisterRoutes registers API routes
func (s *TerminalAPIService) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/terminal")
	{
		api.POST("/sessions", s.CreateSession)
		api.GET("/sessions", s.ListSessions)
		api.GET("/sessions/:id/output", s.GetSessionOutput)
		api.POST("/sessions/:id/input", s.SendSessionInput)
		api.DELETE("/sessions/:id", s.DeleteSession)
		api.POST("/sessions/:id/attach", s.AttachSession)
	}
}