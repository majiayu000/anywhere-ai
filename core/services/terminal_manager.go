package services

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	
	"github.com/majiayu000/ai-cli-core/database"
	"github.com/majiayu000/ai-cli-core/terminal"
)

// TerminalManagerService handles terminal session management
type TerminalManagerService struct {
	persistentManager *terminal.PersistentManager
	store            *terminal.PostgresSessionStore
	discovery        *terminal.MDNSDiscoveryService
}

// CreateSessionRequest represents terminal session creation request
type CreateSessionRequest struct {
	Name     string `json:"name" binding:"required"`
	ToolName string `json:"tool_name" binding:"required"`
}

// CreateSessionResponse represents terminal session creation response
type CreateSessionResponse struct {
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
	ToolName  string `json:"tool_name"`
	Status    string `json:"status"`
}

// SessionInfo represents session information
type SessionInfo struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	ToolName        string    `json:"tool_name"`
	Status          string    `json:"status"`
	OwnerDeviceID   string    `json:"owner_device_id"`
	OwnerDeviceName string    `json:"owner_device_name"`
	CurrentDeviceID string    `json:"current_device_id"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AttachSessionRequest represents session attachment request
type AttachSessionRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

// NewTerminalManagerService creates new terminal manager service
func NewTerminalManagerService() (*TerminalManagerService, error) {
	// Initialize PostgreSQL store
	store, err := terminal.NewPostgresSessionStore(getDatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("failed to create session store: %w", err)
	}

	// Initialize discovery service
	deviceID := uuid.New().String()
	discovery := terminal.NewMDNSDiscoveryService(deviceID, "Go Server", "server", 8080)

	// Initialize persistent manager
	persistentManager := terminal.NewPersistentManager(deviceID, "Go Server", "server", store, discovery)

	return &TerminalManagerService{
		persistentManager: persistentManager,
		store:            store,
		discovery:        discovery,
	}, nil
}

// CreateSession creates a new terminal session
func (tms *TerminalManagerService) CreateSession(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Create persistent session
	session, err := tms.persistentManager.CreatePersistentSession(req.Name, req.ToolName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create session: %v", err)})
		return
	}

	// Create database record for user association
	terminalSession := database.TerminalSession{
		ID:              uuid.MustParse(session.ID),
		UserID:          userID.(uuid.UUID),
		Name:            req.Name,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		OwnerDeviceID:   tms.persistentManager.GetDeviceID(),
		OwnerDeviceName: tms.persistentManager.GetDeviceName(),
		CurrentDeviceID: tms.persistentManager.GetDeviceID(),
		LastHeartbeat:   time.Now(),
		ToolName:        req.ToolName,
		Status:          "created",
		Tags:            []string{},
		Metadata:        make(map[string]interface{}),
	}

	if err := database.DB.Create(&terminalSession).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session to database"})
		return
	}

	response := CreateSessionResponse{
		SessionID: session.ID,
		Name:      req.Name,
		ToolName:  req.ToolName,
		Status:    "created",
	}

	c.JSON(http.StatusOK, response)
}

// ListSessions lists all sessions for the user
func (tms *TerminalManagerService) ListSessions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get sessions from database
	var sessions []database.TerminalSession
	if err := database.DB.Where("user_id = ?", userID).Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sessions"})
		return
	}

	// Convert to response format
	var sessionInfos []SessionInfo
	for _, session := range sessions {
		sessionInfos = append(sessionInfos, SessionInfo{
			ID:              session.ID.String(),
			Name:            session.Name,
			ToolName:        session.ToolName,
			Status:          session.Status,
			OwnerDeviceID:   session.OwnerDeviceID,
			OwnerDeviceName: session.OwnerDeviceName,
			CurrentDeviceID: session.CurrentDeviceID,
			LastHeartbeat:   session.LastHeartbeat,
			CreatedAt:       session.CreatedAt,
			UpdatedAt:       session.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessionInfos})
}

// DiscoverSessions discovers sessions from other devices
func (tms *TerminalManagerService) DiscoverSessions(c *gin.Context) {
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Discover sessions
	discoveredSessions, err := tms.persistentManager.DiscoverSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to discover sessions: %v", err)})
		return
	}

	// Filter sessions by user (add proper filtering logic)
	var userSessions []SessionInfo
	for _, session := range discoveredSessions {
		// Convert session state to session info
		sessionInfo := SessionInfo{
			ID:              session.ID,
			Name:            session.Name,
			ToolName:        session.ToolName,
			Status:          string(session.Status),
			OwnerDeviceID:   session.OwnerDeviceID,
			OwnerDeviceName: session.OwnerDeviceName,
			CurrentDeviceID: session.CurrentDeviceID,
			LastHeartbeat:   session.LastHeartbeat,
			CreatedAt:       session.CreatedAt,
			UpdatedAt:       session.UpdatedAt,
		}
		userSessions = append(userSessions, sessionInfo)
	}

	c.JSON(http.StatusOK, gin.H{"discovered_sessions": userSessions})
}

// AttachToSession attaches to an existing session
func (tms *TerminalManagerService) AttachToSession(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req AttachSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Verify user owns this session
	var terminalSession database.TerminalSession
	if err := database.DB.Where("id = ? AND user_id = ?", req.SessionID, userID).First(&terminalSession).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found or access denied"})
		return
	}

	// Attach to session using persistent manager
	session, err := tms.persistentManager.AttachToSession(req.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to attach to session: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"session_id": session.ID,
		"message":    "Successfully attached to session",
	})
}

// GetSessionInfo gets detailed session information
func (tms *TerminalManagerService) GetSessionInfo(c *gin.Context) {
	sessionID := c.Param("session_id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get session from database
	var terminalSession database.TerminalSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&terminalSession).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	sessionInfo := SessionInfo{
		ID:              terminalSession.ID.String(),
		Name:            terminalSession.Name,
		ToolName:        terminalSession.ToolName,
		Status:          terminalSession.Status,
		OwnerDeviceID:   terminalSession.OwnerDeviceID,
		OwnerDeviceName: terminalSession.OwnerDeviceName,
		CurrentDeviceID: terminalSession.CurrentDeviceID,
		LastHeartbeat:   terminalSession.LastHeartbeat,
		CreatedAt:       terminalSession.CreatedAt,
		UpdatedAt:       terminalSession.UpdatedAt,
	}

	c.JSON(http.StatusOK, sessionInfo)
}

// DeleteSession deletes a session
func (tms *TerminalManagerService) DeleteSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Verify ownership and delete from database
	result := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&database.TerminalSession{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Delete from persistent store
	if err := tms.store.DeleteSession(sessionID); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to delete session from store: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted successfully"})
}

// RegisterRoutes registers terminal manager routes
func (tms *TerminalManagerService) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/terminal")
	{
		api.POST("/sessions", tms.CreateSession)
		api.GET("/sessions", tms.ListSessions)
		api.GET("/sessions/discover", tms.DiscoverSessions)
		api.POST("/sessions/attach", tms.AttachToSession)
		api.GET("/sessions/:session_id", tms.GetSessionInfo)
		api.DELETE("/sessions/:session_id", tms.DeleteSession)
	}
}

// getDatabaseURL constructs database URL from environment or uses default
func getDatabaseURL() string {
	// This should match your database configuration
	// In production, read from environment variables
	return "postgresql://user:password@localhost:5432/anywhere_core"
}