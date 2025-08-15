package terminal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	
	"github.com/hashicorp/mdns"
)

// MDNSDiscoveryService implements device discovery using mDNS (Bonjour/Zeroconf)
type MDNSDiscoveryService struct {
	deviceID   string
	deviceName string
	deviceType string
	port       int
	
	// mDNS server
	server *mdns.Server
	
	// Discovered devices
	devices     map[string]*DeviceInfo
	devicesMutex sync.RWMutex
	
	// Event channel
	eventChan chan DiscoveryEvent
	
	// HTTP client for inter-device communication
	httpClient *http.Client
}

// DiscoveryEvent represents a discovery event
type DiscoveryEvent struct {
	Type     string     `json:"type"`     // "device_found", "device_lost", "session_announced"
	DeviceID string     `json:"device_id"`
	Device   DeviceInfo `json:"device,omitempty"`
	Sessions []string   `json:"sessions,omitempty"`
}

// RemoteConnection represents a connection to a remote device
type RemoteConnection struct {
	deviceID   string
	baseURL    string
	httpClient *http.Client
}

// MigrationRequest represents a session migration request
type MigrationRequest struct {
	SessionID      string    `json:"session_id"`
	TargetDeviceID string    `json:"target_device_id"`
	Timestamp      time.Time `json:"timestamp"`
}

// MigrationResponse represents a session migration response
type MigrationResponse struct {
	Success    bool               `json:"success"`
	Error      string             `json:"error,omitempty"`
	Checkpoint *SessionCheckpoint `json:"checkpoint,omitempty"`
}

// NewMDNSDiscoveryService creates a new mDNS discovery service
func NewMDNSDiscoveryService(deviceID, deviceName, deviceType string, port int) *MDNSDiscoveryService {
	return &MDNSDiscoveryService{
		deviceID:   deviceID,
		deviceName: deviceName,
		deviceType: deviceType,
		port:       port,
		devices:    make(map[string]*DeviceInfo),
		eventChan:  make(chan DiscoveryEvent, 100),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Announce announces this device and its sessions
func (d *MDNSDiscoveryService) Announce(device DeviceInfo, sessions []string) error {
	// Create mDNS service info
	info := []string{
		fmt.Sprintf("device_id=%s", device.ID),
		fmt.Sprintf("device_name=%s", device.Name),
		fmt.Sprintf("device_type=%s", device.Type),
		fmt.Sprintf("sessions=%s", formatSessions(sessions)),
		fmt.Sprintf("timestamp=%d", time.Now().Unix()),
	}
	
	// Register mDNS service
	service, err := mdns.NewMDNSService(
		device.ID,                    // Instance name
		"_terminal-manager._tcp",     // Service name
		"",                          // Domain (empty = local)
		"",                          // Host name (empty = auto)
		d.port,                      // Port
		nil,                         // IPs (auto-detect)
		info,                        // TXT records
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}
	
	// Start mDNS server
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}
	
	d.server = server
	
	// Start HTTP server for inter-device communication
	go d.startHTTPServer()
	
	return nil
}

// Discover discovers other devices
func (d *MDNSDiscoveryService) Discover() ([]DeviceInfo, error) {
	// Create entries channel
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	
	// Start discovery
	go func() {
		defer close(entriesCh)
		
		params := mdns.DefaultParams("_terminal-manager._tcp")
		params.Entries = entriesCh
		params.Timeout = 3 * time.Second
		
		mdns.Query(params)
	}()
	
	// Collect results
	devices := []DeviceInfo{}
	
	for entry := range entriesCh {
		device := d.parseServiceEntry(entry)
		if device != nil && device.ID != d.deviceID {
			d.devicesMutex.Lock()
			d.devices[device.ID] = device
			d.devicesMutex.Unlock()
			
			devices = append(devices, *device)
			
			// Send discovery event
			select {
			case d.eventChan <- DiscoveryEvent{
				Type:     "device_found",
				DeviceID: device.ID,
				Device:   *device,
				Sessions: device.Sessions,
			}:
			default:
				// Channel full, skip event
			}
		}
	}
	
	return devices, nil
}

// Subscribe returns the event channel
func (d *MDNSDiscoveryService) Subscribe() <-chan DiscoveryEvent {
	return d.eventChan
}

// ConnectToDevice connects to a remote device
func (d *MDNSDiscoveryService) ConnectToDevice(deviceID string) (RemoteConnection, error) {
	d.devicesMutex.RLock()
	device, exists := d.devices[deviceID]
	d.devicesMutex.RUnlock()
	
	if !exists {
		return RemoteConnection{}, fmt.Errorf("device %s not found", deviceID)
	}
	
	baseURL := fmt.Sprintf("http://%s:%d", device.IPAddress, device.Port)
	
	return RemoteConnection{
		deviceID:   deviceID,
		baseURL:    baseURL,
		httpClient: d.httpClient,
	}, nil
}

// parseServiceEntry parses mDNS service entry into DeviceInfo
func (d *MDNSDiscoveryService) parseServiceEntry(entry *mdns.ServiceEntry) *DeviceInfo {
	if entry == nil {
		return nil
	}
	
	device := &DeviceInfo{
		IPAddress: entry.AddrV4.String(),
		Port:      entry.Port,
		LastSeen:  time.Now(),
	}
	
	// Parse TXT records
	for _, txt := range entry.InfoFields {
		if len(txt) >= 10 && txt[:10] == "device_id=" {
			device.ID = txt[10:]
		} else if len(txt) >= 12 && txt[:12] == "device_name=" {
			device.Name = txt[12:]
		} else if len(txt) >= 12 && txt[:12] == "device_type=" {
			device.Type = txt[12:]
		} else if len(txt) >= 9 && txt[:9] == "sessions=" {
			device.Sessions = parseSessions(txt[9:])
		}
	}
	
	return device
}

// startHTTPServer starts HTTP server for inter-device communication
func (d *MDNSDiscoveryService) startHTTPServer() {
	mux := http.NewServeMux()
	
	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":     "ok",
			"device_id":  d.deviceID,
			"timestamp":  time.Now().Format(time.RFC3339),
		})
	})
	
	// Session migration endpoint
	mux.HandleFunc("/migrate", d.handleMigrationRequest)
	
	// Session info endpoint
	mux.HandleFunc("/sessions", d.handleSessionInfo)
	
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", d.port),
		Handler: mux,
	}
	
	// Start server
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		fmt.Printf("Failed to start HTTP server: %v\n", err)
		return
	}
	
	server.Serve(ln)
}

// handleMigrationRequest handles session migration requests
func (d *MDNSDiscoveryService) handleMigrationRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req MigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// TODO: Implement actual migration logic
	// This would involve:
	// 1. Creating a checkpoint of the session
	// 2. Stopping the local session
	// 3. Returning the checkpoint data
	
	resp := MigrationResponse{
		Success: true,
		Checkpoint: &SessionCheckpoint{
			ID:        fmt.Sprintf("checkpoint-%d", time.Now().Unix()),
			SessionID: req.SessionID,
			Timestamp: time.Now(),
			// TODO: Add actual checkpoint data
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSessionInfo handles session info requests
func (d *MDNSDiscoveryService) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// TODO: Return list of active sessions on this device
	sessions := []string{} // Get from session manager
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"device_id": d.deviceID,
		"sessions":  sessions,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Close closes the discovery service
func (d *MDNSDiscoveryService) Close() error {
	if d.server != nil {
		d.server.Shutdown()
	}
	close(d.eventChan)
	return nil
}

// RequestMigration requests session migration
func (conn RemoteConnection) RequestMigration(req MigrationRequest) (*MigrationResponse, error) {
	url := fmt.Sprintf("%s/migrate", conn.baseURL)
	
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	
	resp, err := conn.httpClient.Post(url, "application/json", 
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("migration failed with status %d", resp.StatusCode)
	}
	
	var migrationResp MigrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&migrationResp); err != nil {
		return nil, err
	}
	
	return &migrationResp, nil
}

// Close closes the remote connection
func (conn RemoteConnection) Close() error {
	// No resources to clean up for HTTP connections
	return nil
}

// Helper functions

func formatSessions(sessions []string) string {
	if len(sessions) == 0 {
		return ""
	}
	
	// Join sessions with comma
	result := ""
	for i, session := range sessions {
		if i > 0 {
			result += ","
		}
		result += session
	}
	return result
}

func parseSessions(sessionsStr string) []string {
	if sessionsStr == "" {
		return []string{}
	}
	
	// Split by comma
	return strings.Split(sessionsStr, ",")
}