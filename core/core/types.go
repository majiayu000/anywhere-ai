package core

import (
	"time"
)

// Command represents a command to be executed
type Command struct {
	Path string   // Executable path
	Args []string // Command arguments
	Env  []string // Environment variables
	Dir  string   // Working directory
}

// SessionStatus represents the current state of a session
type SessionStatus string

const (
	SessionStatusStarting SessionStatus = "starting"
	SessionStatusRunning  SessionStatus = "running"
	SessionStatusStopping SessionStatus = "stopping"
	SessionStatusStopped  SessionStatus = "stopped"
	SessionStatusError    SessionStatus = "error"
)

// OutputType represents different types of output from the tool
type OutputType string

const (
	OutputTypeStandard     OutputType = "standard"
	OutputTypeError        OutputType = "error"
	OutputTypePrompt       OutputType = "prompt"
	OutputTypeWaitingInput OutputType = "waiting_input"
	OutputTypeSystem       OutputType = "system"
	OutputTypeProgress     OutputType = "progress"
	OutputTypeToolUse      OutputType = "tool_use"
)

// ParsedOutput represents processed output from a tool
type ParsedOutput struct {
	Raw       string                 // Original output
	Content   string                 // Cleaned content (no ANSI codes, etc.)
	Type      OutputType             // Type of output
	Timestamp time.Time              // When the output was generated
	Metadata  map[string]interface{} // Tool-specific metadata
}

// OutputMessage represents a message to be sent to clients
type OutputMessage struct {
	ID        string                 `json:"id"`
	Type      OutputType             `json:"type"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	ToolName  string                 `json:"tool_name"`
	SessionID string                 `json:"session_id"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToolError represents an error from a tool
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Fatal   bool   `json:"fatal"`
}

// SessionStats contains statistics about a session
type SessionStats struct {
	StartTime      time.Time     `json:"start_time"`
	Duration       time.Duration `json:"duration"`
	MessagesIn     int           `json:"messages_in"`
	MessagesOut    int           `json:"messages_out"`
	BytesIn        int64         `json:"bytes_in"`
	BytesOut       int64         `json:"bytes_out"`
	LastActivity   time.Time     `json:"last_activity"`
	Cost           float64       `json:"cost,omitempty"`
	TokensUsed     int           `json:"tokens_used,omitempty"`
	ToolsUsed      []string      `json:"tools_used,omitempty"`
	ErrorCount     int           `json:"error_count"`
	RestartCount   int           `json:"restart_count"`
	CurrentMemory  int64         `json:"current_memory,omitempty"`
	PeakMemory     int64         `json:"peak_memory,omitempty"`
	CPUUsage       float64       `json:"cpu_usage,omitempty"`
}

// ToolInfo provides information about an available tool
type ToolInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Installed   bool   `json:"installed"`
	Available   bool   `json:"available"`
	Path        string `json:"path,omitempty"`
}