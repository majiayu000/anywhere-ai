package services

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	
	"github.com/majiayu000/ai-cli-core/database"
)

// AgentCommunicationService handles AI agent communication (like Omnara)
type AgentCommunicationService struct {
	upgrader websocket.Upgrader
	clients  map[string]*websocket.Conn // agentInstanceID -> connection
}

// MessageRequest represents an incoming message from agent
type MessageRequest struct {
	Content           string                 `json:"content"`
	SenderType        string                 `json:"sender_type"`
	RequiresUserInput bool                   `json:"requires_user_input"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// MessageResponse represents response to agent
type MessageResponse struct {
	Success        bool                   `json:"success"`
	MessageID      uuid.UUID              `json:"message_id"`
	QueuedMessages []database.Message     `json:"queued_messages"`
	Error          string                 `json:"error,omitempty"`
}

// UserFeedbackRequest represents user feedback
type UserFeedbackRequest struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

// NewAgentCommunicationService creates new agent communication service
func NewAgentCommunicationService() *AgentCommunicationService {
	return &AgentCommunicationService{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// In production, check against allowed origins
				return true
			},
		},
		clients: make(map[string]*websocket.Conn),
	}
}

// SendMessage handles agent sending messages (like Omnara's MCP endpoint)
func (acs *AgentCommunicationService) SendMessage(c *gin.Context) {
	// Get agent instance ID from auth context (similar to Omnara's auth)
	agentInstanceID, exists := c.Get("agent_instance_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Agent instance not found"})
		return
	}

	var req MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Create message record
	message := database.Message{
		ID:               uuid.New(),
		AgentInstanceID:  agentInstanceID.(uuid.UUID),
		SenderType:       req.SenderType,
		Content:          req.Content,
		CreatedAt:        time.Now(),
		RequiresUserInput: req.RequiresUserInput,
		MessageMetadata:  req.Metadata,
	}

	// Save to database
	if err := database.DB.Create(&message).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
		return
	}

	// Get queued user messages (unread messages for this agent)
	var queuedMessages []database.Message
	database.DB.Where("agent_instance_id = ? AND sender_type = 'USER'", agentInstanceID).
		Order("created_at ASC").Find(&queuedMessages)

	// Update last read message (mark user messages as read)
	if len(queuedMessages) > 0 {
		lastMessage := queuedMessages[len(queuedMessages)-1]
		database.DB.Model(&database.AgentInstance{}).
			Where("id = ?", agentInstanceID).
			Update("last_read_message_id", lastMessage.ID)
	}

	// Send real-time notification if user is connected via WebSocket
	acs.notifyUser(agentInstanceID.(uuid.UUID), message)

	// Response
	resp := MessageResponse{
		Success:        true,
		MessageID:      message.ID,
		QueuedMessages: queuedMessages,
	}

	c.JSON(http.StatusOK, resp)
}

// SendUserFeedback handles user sending feedback to agent
func (acs *AgentCommunicationService) SendUserFeedback(c *gin.Context) {
	agentInstanceIDStr := c.Param("instance_id")
	agentInstanceID, err := uuid.Parse(agentInstanceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid instance ID"})
		return
	}

	var req UserFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Verify user owns this agent instance (add proper auth here)
	var instance database.AgentInstance
	if err := database.DB.First(&instance, agentInstanceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent instance not found"})
		return
	}

	// Create user message
	message := database.Message{
		ID:               uuid.New(),
		AgentInstanceID:  agentInstanceID,
		SenderType:       "USER",
		Content:          req.Content,
		CreatedAt:        time.Now(),
		RequiresUserInput: false,
		MessageMetadata:  req.Metadata,
	}

	// Save to database
	if err := database.DB.Create(&message).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message_id": message.ID,
	})
}

// GetMessages gets conversation history
func (acs *AgentCommunicationService) GetMessages(c *gin.Context) {
	agentInstanceIDStr := c.Param("instance_id")
	agentInstanceID, err := uuid.Parse(agentInstanceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid instance ID"})
		return
	}

	// Get messages for this agent instance
	var messages []database.Message
	database.DB.Where("agent_instance_id = ?", agentInstanceID).
		Order("created_at ASC").Find(&messages)

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
	})
}

// WebSocketHandler handles WebSocket connections for real-time communication
func (acs *AgentCommunicationService) WebSocketHandler(c *gin.Context) {
	agentInstanceIDStr := c.Param("instance_id")
	agentInstanceID, err := uuid.Parse(agentInstanceIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid instance ID"})
		return
	}

	// Upgrade connection to WebSocket
	conn, err := acs.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Store connection
	acs.clients[agentInstanceID.String()] = conn

	// Remove connection when done
	defer func() {
		delete(acs.clients, agentInstanceID.String())
	}()

	// Keep connection alive and handle messages
	for {
		// Read message from client
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		// Parse message
		var wsMessage map[string]interface{}
		if err := json.Unmarshal(msg, &wsMessage); err != nil {
			continue
		}

		// Handle different message types
		switch wsMessage["type"] {
		case "ping":
			// Send pong
			conn.WriteJSON(map[string]interface{}{
				"type":      "pong",
				"timestamp": time.Now(),
			})
		case "user_message":
			// Handle user message
			content, ok := wsMessage["content"].(string)
			if ok {
				acs.handleUserMessage(agentInstanceID, content, conn)
			}
		}
	}
}

// notifyUser sends real-time notification to user via WebSocket
func (acs *AgentCommunicationService) notifyUser(agentInstanceID uuid.UUID, message database.Message) {
	conn, exists := acs.clients[agentInstanceID.String()]
	if !exists {
		return
	}

	notification := map[string]interface{}{
		"type":               "new_message",
		"message":            message,
		"requires_user_input": message.RequiresUserInput,
		"timestamp":          time.Now(),
	}

	if err := conn.WriteJSON(notification); err != nil {
		log.Printf("Failed to send WebSocket message: %v", err)
		// Clean up broken connection
		delete(acs.clients, agentInstanceID.String())
	}
}

// handleUserMessage handles user message from WebSocket
func (acs *AgentCommunicationService) handleUserMessage(agentInstanceID uuid.UUID, content string, conn *websocket.Conn) {
	// Create user message
	message := database.Message{
		ID:               uuid.New(),
		AgentInstanceID:  agentInstanceID,
		SenderType:       "USER",
		Content:          content,
		CreatedAt:        time.Now(),
		RequiresUserInput: false,
		MessageMetadata:  make(map[string]interface{}),
	}

	// Save to database
	if err := database.DB.Create(&message).Error; err != nil {
		log.Printf("Failed to save user message: %v", err)
		return
	}

	// Send confirmation
	conn.WriteJSON(map[string]interface{}{
		"type":       "message_sent",
		"message_id": message.ID,
		"timestamp":  time.Now(),
	})
}

// RegisterRoutes registers agent communication routes
func (acs *AgentCommunicationService) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		// Agent endpoints (require agent authentication)
		agent := api.Group("/agent")
		{
			agent.POST("/messages", acs.SendMessage)
		}

		// User endpoints
		user := api.Group("/user")
		{
			user.POST("/agents/:instance_id/feedback", acs.SendUserFeedback)
			user.GET("/agents/:instance_id/messages", acs.GetMessages)
		}

		// WebSocket endpoint
		api.GET("/ws/:instance_id", acs.WebSocketHandler)
	}
}