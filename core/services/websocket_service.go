package services

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/majiayu000/anywhere-ai/core/tmux"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		return true
	},
}

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan []byte
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
}

// WebSocketClient represents a WebSocket client
type WebSocketClient struct {
	hub       *WebSocketHub
	conn      *websocket.Conn
	send      chan []byte
	sessionID string
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Action    string `json:"action"`
	SessionID string `json:"sessionId"`
	Output    string `json:"output,omitempty"`
	Input     string `json:"input,omitempty"`
}

// TerminalWebSocketService handles WebSocket connections for terminal sessions
type TerminalWebSocketService struct {
	hub         *WebSocketHub
	tmuxManager *tmux.Manager
	monitors    map[string]context.CancelFunc
	mu          sync.RWMutex
}

// NewTerminalWebSocketService creates a new WebSocket service
func NewTerminalWebSocketService(tmuxManager *tmux.Manager) *TerminalWebSocketService {
	hub := &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}

	service := &TerminalWebSocketService{
		hub:         hub,
		tmuxManager: tmuxManager,
		monitors:    make(map[string]context.CancelFunc),
	}

	// Start the hub
	go hub.run()

	return service
}

// run runs the WebSocket hub
func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client registered: %v", client.sessionID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("Client unregistered: %v", client.sessionID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// HandleWebSocket handles WebSocket connections
func (s *TerminalWebSocketService) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &WebSocketClient{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	client.hub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump(s)
}

// readPump reads messages from the WebSocket connection
func (c *WebSocketClient) readPump(s *TerminalWebSocketService) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
		// Stop monitoring if active
		if c.sessionID != "" {
			s.stopMonitoring(c.sessionID)
		}
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg WebSocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		switch msg.Action {
		case "subscribe":
			// Stop previous monitoring if any
			if c.sessionID != "" {
				s.stopMonitoring(c.sessionID)
			}
			c.sessionID = msg.SessionID
			s.startMonitoring(c, msg.SessionID)

		case "unsubscribe":
			s.stopMonitoring(msg.SessionID)
			c.sessionID = ""

		case "input":
			if msg.SessionID != "" && msg.Input != "" {
				s.sendInput(msg.SessionID, msg.Input)
			}
		}
	}
}

// writePump writes messages to the WebSocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// startMonitoring starts monitoring a session for output
func (s *TerminalWebSocketService) startMonitoring(client *WebSocketClient, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing monitor if any
	if cancel, exists := s.monitors[sessionID]; exists {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.monitors[sessionID] = cancel

	go func() {
		lastOutput := ""
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				output, err := s.tmuxManager.CaptureOutput(context.Background(), sessionID)
				if err != nil {
					log.Printf("Failed to capture output for session %s: %v", sessionID, err)
					continue
				}

				if output != lastOutput {
					msg := WebSocketMessage{
						Action:    "output",
						SessionID: sessionID,
						Output:    output,
					}
					data, _ := json.Marshal(msg)
					
					select {
					case client.send <- data:
					default:
						// Client buffer full, skip
					}
					
					lastOutput = output
				}
			}
		}
	}()
}

// stopMonitoring stops monitoring a session
func (s *TerminalWebSocketService) stopMonitoring(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, exists := s.monitors[sessionID]; exists {
		cancel()
		delete(s.monitors, sessionID)
	}
}

// sendInput sends input to a session
func (s *TerminalWebSocketService) sendInput(sessionID string, input string) {
	ctx := context.Background()
	if err := s.tmuxManager.SendCommand(ctx, sessionID, input); err != nil {
		log.Printf("Failed to send input to session %s: %v", sessionID, err)
	}
}

// BroadcastToSession broadcasts a message to all clients watching a session
func (s *TerminalWebSocketService) BroadcastToSession(sessionID string, output string) {
	msg := WebSocketMessage{
		Action:    "output",
		SessionID: sessionID,
		Output:    output,
	}
	data, _ := json.Marshal(msg)
	s.hub.broadcast <- data
}