package core

import (
	"context"
	"time"
)

// ToolAdapter defines the interface that all AI CLI tools must implement
type ToolAdapter interface {
	// Basic information
	GetName() string
	GetVersion() string
	GetDescription() string
	GetIcon() string

	// Installation and configuration
	IsInstalled() bool
	GetExecutablePath() string
	GetDefaultArgs() []string
	ValidateConfig() error

	// Command building
	BuildCommand(args []string) *Command

	// Output processing
	ParseOutput(data []byte) (*ParsedOutput, error)
	IsPromptReady(output string) bool
	IsWaitingForInput(output string) bool
	DetectError(output string) *ToolError

	// Input processing
	TransformInput(input string) string
	HandleSpecialCommand(cmd string) (handled bool, response string)
}

// Session represents an active AI tool session
type Session interface {
	GetID() string
	GetTool() ToolAdapter
	GetStartTime() time.Time
	GetStatus() SessionStatus

	// Lifecycle management
	Start(ctx context.Context) error
	Stop() error
	Restart() error

	// IO operations
	SendInput(input string) error
	GetOutputStream() <-chan *OutputMessage

	// Status and statistics
	IsRunning() bool
	GetStats() *SessionStats
}

// ProcessManager handles the underlying process execution
type ProcessManager interface {
	Start(ctx context.Context, cmd *Command) error
	Stop() error

	GetPID() int
	IsRunning() bool

	Write(data []byte) (int, error)
	Read() <-chan []byte
}

// StreamProcessor handles parsing and processing of output streams
type StreamProcessor interface {
	ProcessData(data []byte)
	GetOutput() <-chan *OutputMessage
	Reset()
}