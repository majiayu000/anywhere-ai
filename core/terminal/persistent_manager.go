package terminal

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/google/uuid"
)

// PersistentManager manages terminal sessions that can be persisted and restored across devices
type PersistentManager struct {
	sessions  map[string]*PersistentSession
	store     SessionStore
	discovery DiscoveryService
	mu        sync.RWMutex
	
	// Local device info
	deviceID   string
	deviceName string
	deviceType string // "ios", "macos", "web", etc.
	
	config PersistentManagerConfig
}

// PersistentManagerConfig configuration for persistent manager
type PersistentManagerConfig struct {
	MaxSessions      int
	SyncInterval     time.Duration // How often to sync state
	HeartbeatInterval time.Duration // How often to send heartbeat
	SessionTimeout   time.Duration // When to consider a session dead
	EnableDiscovery  bool          // Enable network discovery
}

// SessionStore interface for persisting session state
type SessionStore interface {
	// Save session state
	SaveSession(session *SessionState) error
	
	// Load session state
	LoadSession(sessionID string) (*SessionState, error)
	
	// List all sessions
	ListSessions(filter SessionFilter) ([]*SessionState, error)
	
	// Delete session
	DeleteSession(sessionID string) error
	
	// Update session heartbeat
	UpdateHeartbeat(sessionID string, deviceID string) error
}

// DiscoveryService interface for discovering sessions across network
type DiscoveryService interface {
	// Announce this device and its sessions
	Announce(device DeviceInfo, sessions []string) error
	
	// Discover other devices and their sessions
	Discover() ([]DeviceInfo, error)
	
	// Subscribe to discovery events
	Subscribe() <-chan DiscoveryEvent
	
	// Connect to remote device
	ConnectToDevice(deviceID string) (RemoteConnection, error)
}

// Session represents a basic terminal session (placeholder)
type Session struct {
	ID  string
	ctx context.Context
}

// SessionStatus represents session status
type SessionStatus string

const (
	SessionStatusCreated SessionStatus = "created"
	SessionStatusRunning SessionStatus = "running"
	SessionStatusStopped SessionStatus = "stopped"
)

// SessionConfig represents session configuration
type SessionConfig struct {
	Rows             int
	Cols             int
	ScrollbackBuffer int
	AutoRestart      bool
	RestartDelay     int
}

// NewSession creates a new session (placeholder implementation)
func NewSession(id string, config SessionConfig) *Session {
	return &Session{
		ID:  id,
		ctx: context.Background(),
	}
}

// SessionInfo represents session information
type SessionInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// Session methods (placeholder implementations)
func (s *Session) RestoreBuffer(content []byte) {}
func (s *Session) SetCursorPosition(row, col int) {}
func (s *Session) SetEnvironment(env map[string]string) {}
func (s *Session) SetWorkingDirectory(dir string) {}
func (s *Session) SendInput(input string) {}
func (s *Session) GetBuffer() []byte { return []byte{} }
func (s *Session) GetCursorPosition() (int, int) { return 0, 0 }
func (s *Session) GetInputHistory() []InputRecord { return []InputRecord{} }
func (s *Session) GetOutputHistory() []OutputRecord { return []OutputRecord{} }
func (s *Session) GetInfo() SessionInfo { return SessionInfo{ID: s.ID} }
func (s *Session) Stop() error { return nil }
func (s *Session) Restart() error { return nil }
func (s *Session) IsRunning() bool { return true }

// PersistentSession represents a session that can be persisted and restored
type PersistentSession struct {
	*Session // Embed base session
	
	// Persistent state
	state      *SessionState
	store      SessionStore
	lastSync   time.Time
	syncMutex  sync.Mutex
	
	// Recovery info
	recoverable bool
	checkpoint  *SessionCheckpoint
}

// SessionState represents the persistable state of a session
type SessionState struct {
	// Identity
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// Device info
	OwnerDeviceID   string    `json:"owner_device_id"`
	OwnerDeviceName string    `json:"owner_device_name"`
	CurrentDeviceID string    `json:"current_device_id"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`
	
	// Terminal state
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Environment map[string]string `json:"environment"`
	WorkingDir  string            `json:"working_dir"`
	
	// Buffer state
	BufferContent []byte `json:"buffer_content"`
	CursorRow     int    `json:"cursor_row"`
	CursorCol     int    `json:"cursor_col"`
	ScrollOffset  int    `json:"scroll_offset"`
	
	// Tool state
	ToolName    string                 `json:"tool_name"`
	ToolState   map[string]interface{} `json:"tool_state"`
	
	// Session metadata
	Status      SessionStatus          `json:"status"`
	Tags        []string               `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SessionCheckpoint represents a point-in-time snapshot for recovery
type SessionCheckpoint struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Timestamp  time.Time `json:"timestamp"`
	
	// Input/Output history for replay
	InputHistory  []InputRecord  `json:"input_history"`
	OutputHistory []OutputRecord `json:"output_history"`
	
	// State snapshot
	StateSnapshot *SessionState `json:"state_snapshot"`
}

// InputRecord represents a recorded input
type InputRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	Source    string    `json:"source"` // "user", "system", "tool"
}

// OutputRecord represents a recorded output
type OutputRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Content   []byte    `json:"content"`
	Type      string    `json:"type"` // "stdout", "stderr"
}

// DeviceInfo represents information about a device
type DeviceInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	IPAddress   string    `json:"ip_address"`
	Port        int       `json:"port"`
	Sessions    []string  `json:"sessions"`
	LastSeen    time.Time `json:"last_seen"`
	Capabilities []string `json:"capabilities"`
}

// SessionFilter for filtering sessions
type SessionFilter struct {
	DeviceID   string
	ToolName   string
	Status     SessionStatus
	Tags       []string
	CreatedAfter  time.Time
	CreatedBefore time.Time
	IncludeDead   bool
}

// NewPersistentManager creates a new persistent terminal manager
func NewPersistentManager(deviceID, deviceName, deviceType string, store SessionStore, discovery DiscoveryService) *PersistentManager {
	return &PersistentManager{
		sessions:   make(map[string]*PersistentSession),
		store:      store,
		discovery:  discovery,
		deviceID:   deviceID,
		deviceName: deviceName,
		deviceType: deviceType,
		config: PersistentManagerConfig{
			MaxSessions:       10,
			SyncInterval:      5 * time.Second,
			HeartbeatInterval: 10 * time.Second,
			SessionTimeout:    5 * time.Minute,
			EnableDiscovery:   true,
		},
	}
}

// CreatePersistentSession creates a new persistent session
func (pm *PersistentManager) CreatePersistentSession(name string, toolName string) (*PersistentSession, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Check max sessions
	if len(pm.sessions) >= pm.config.MaxSessions {
		return nil, fmt.Errorf("max sessions limit reached")
	}
	
	// Generate session ID
	sessionID := uuid.New().String()
	if name != "" {
		sessionID = fmt.Sprintf("%s-%s", name, sessionID[:8])
	}
	
	// Create base session
	baseSession := NewSession(sessionID, SessionConfig{
		Rows: 40,
		Cols: 120,
	})
	
	// Create session state
	state := &SessionState{
		ID:              sessionID,
		Name:            name,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		OwnerDeviceID:   pm.deviceID,
		OwnerDeviceName: pm.deviceName,
		CurrentDeviceID: pm.deviceID,
		LastHeartbeat:   time.Now(),
		ToolName:        toolName,
		Status:          SessionStatusCreated,
		Tags:            []string{},
		Metadata:        make(map[string]interface{}),
	}
	
	// Create persistent session
	session := &PersistentSession{
		Session:     baseSession,
		state:       state,
		store:       pm.store,
		recoverable: true,
	}
	
	// Save initial state
	if err := pm.store.SaveSession(state); err != nil {
		return nil, fmt.Errorf("failed to save session state: %w", err)
	}
	
	// Store locally
	pm.sessions[sessionID] = session
	
	// Start sync routine
	go pm.startSyncRoutine(session)
	
	return session, nil
}

// DiscoverSessions discovers sessions from other devices
func (pm *PersistentManager) DiscoverSessions() ([]*SessionState, error) {
	if !pm.config.EnableDiscovery {
		// Fallback to store-based discovery
		return pm.store.ListSessions(SessionFilter{
			IncludeDead: false,
		})
	}
	
	// Network discovery
	devices, err := pm.discovery.Discover()
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}
	
	var allSessions []*SessionState
	
	for _, device := range devices {
		// Get sessions from each device
		for _, sessionID := range device.Sessions {
			state, err := pm.store.LoadSession(sessionID)
			if err != nil {
				continue
			}
			
			// Check if session is still alive
			if time.Since(state.LastHeartbeat) < pm.config.SessionTimeout {
				allSessions = append(allSessions, state)
			}
		}
	}
	
	return allSessions, nil
}

// AttachToSession attaches to an existing session (possibly from another device)
func (pm *PersistentManager) AttachToSession(sessionID string) (*PersistentSession, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Check if already attached locally
	if session, exists := pm.sessions[sessionID]; exists {
		return session, nil
	}
	
	// Load session state from store
	state, err := pm.store.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}
	
	// Check if session is alive
	if time.Since(state.LastHeartbeat) > pm.config.SessionTimeout {
		return nil, fmt.Errorf("session %s is dead (last heartbeat: %v ago)", 
			sessionID, time.Since(state.LastHeartbeat))
	}
	
	// Check if session is on another device
	if state.CurrentDeviceID != pm.deviceID && state.CurrentDeviceID != "" {
		// Try to migrate session
		if err := pm.migrateSession(state); err != nil {
			return nil, fmt.Errorf("failed to migrate session: %w", err)
		}
	}
	
	// Create local session instance
	baseSession := NewSession(sessionID, SessionConfig{
		Rows: 40,
		Cols: 120,
	})
	
	session := &PersistentSession{
		Session:     baseSession,
		state:       state,
		store:       pm.store,
		recoverable: true,
	}
	
	// Restore session state
	if err := pm.restoreSession(session); err != nil {
		return nil, fmt.Errorf("failed to restore session: %w", err)
	}
	
	// Update ownership
	state.CurrentDeviceID = pm.deviceID
	state.LastHeartbeat = time.Now()
	pm.store.SaveSession(state)
	
	// Store locally
	pm.sessions[sessionID] = session
	
	// Start sync routine
	go pm.startSyncRoutine(session)
	
	return session, nil
}

// migrateSession migrates a session from another device
func (pm *PersistentManager) migrateSession(state *SessionState) error {
	// Connect to the current owner device
	conn, err := pm.discovery.ConnectToDevice(state.CurrentDeviceID)
	if err != nil {
		// Device might be offline, proceed with recovery
		return nil
	}
	defer conn.Close()
	
	// Request session migration
	req := MigrationRequest{
		SessionID:     state.ID,
		TargetDeviceID: pm.deviceID,
		Timestamp:     time.Now(),
	}
	
	resp, err := conn.RequestMigration(req)
	if err != nil {
		return fmt.Errorf("migration request failed: %w", err)
	}
	
	// Apply migration data
	if resp.Checkpoint != nil {
		// We have a checkpoint, use it for recovery
		state.BufferContent = resp.Checkpoint.StateSnapshot.BufferContent
		// Note: InputHistory and OutputHistory are stored in checkpoint, not session state
	}
	
	return nil
}

// restoreSession restores a session from saved state
func (pm *PersistentManager) restoreSession(session *PersistentSession) error {
	state := session.state
	
	// Restore terminal buffer
	if len(state.BufferContent) > 0 {
		// Restore buffer content
		// This would restore the visual state of the terminal
		session.RestoreBuffer(state.BufferContent)
	}
	
	// Restore cursor position
	session.SetCursorPosition(state.CursorRow, state.CursorCol)
	
	// Restore environment
	if state.Environment != nil {
		session.SetEnvironment(state.Environment)
	}
	
	// Restore working directory
	if state.WorkingDir != "" {
		session.SetWorkingDirectory(state.WorkingDir)
	}
	
	// If we have checkpoint data, replay inputs
	if session.checkpoint != nil {
		for _, input := range session.checkpoint.InputHistory {
			// Replay inputs with small delay to avoid overwhelming
			time.Sleep(10 * time.Millisecond)
			session.SendInput(input.Content)
		}
	}
	
	return nil
}

// startSyncRoutine starts background sync for a session
func (pm *PersistentManager) startSyncRoutine(session *PersistentSession) {
	ticker := time.NewTicker(pm.config.SyncInterval)
	defer ticker.Stop()
	
	heartbeatTicker := time.NewTicker(pm.config.HeartbeatInterval)
	defer heartbeatTicker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Sync session state
			pm.syncSession(session)
			
		case <-heartbeatTicker.C:
			// Update heartbeat
			pm.store.UpdateHeartbeat(session.ID, pm.deviceID)
			
		case <-session.ctx.Done():
			// Session stopped
			return
		}
	}
}

// syncSession syncs session state to store
func (pm *PersistentManager) syncSession(session *PersistentSession) {
	session.syncMutex.Lock()
	defer session.syncMutex.Unlock()
	
	// Update state
	state := session.state
	state.UpdatedAt = time.Now()
	state.LastHeartbeat = time.Now()
	state.BufferContent = session.GetBuffer()
	state.CursorRow, state.CursorCol = session.GetCursorPosition()
	
	// Save to store
	if err := pm.store.SaveSession(state); err != nil {
		// Log error but don't fail
		fmt.Printf("Failed to sync session %s: %v\n", session.ID, err)
	}
	
	session.lastSync = time.Now()
}

// CreateCheckpoint creates a checkpoint for recovery
func (pm *PersistentManager) CreateCheckpoint(sessionID string) (*SessionCheckpoint, error) {
	session, exists := pm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	
	checkpoint := &SessionCheckpoint{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		StateSnapshot: session.state,
		InputHistory:  session.GetInputHistory(),
		OutputHistory: session.GetOutputHistory(),
	}
	
	// Store checkpoint
	session.checkpoint = checkpoint
	
	return checkpoint, nil
}

// Announce announces this device and its sessions for discovery
func (pm *PersistentManager) Announce() error {
	if !pm.config.EnableDiscovery {
		return nil
	}
	
	pm.mu.RLock()
	sessionIDs := make([]string, 0, len(pm.sessions))
	for id := range pm.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	pm.mu.RUnlock()
	
	device := DeviceInfo{
		ID:        pm.deviceID,
		Name:      pm.deviceName,
		Type:      pm.deviceType,
		Sessions:  sessionIDs,
		LastSeen:  time.Now(),
		Capabilities: []string{"pty", "migration", "checkpoint"},
	}
	
	return pm.discovery.Announce(device, sessionIDs)
}

// GetDeviceID returns the device ID
func (pm *PersistentManager) GetDeviceID() string {
	return pm.deviceID
}

// GetDeviceName returns the device name
func (pm *PersistentManager) GetDeviceName() string {
	return pm.deviceName
}