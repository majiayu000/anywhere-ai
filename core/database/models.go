package database

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user in the system (from Omnara)
type User struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Email       string    `gorm:"unique;not null" json:"email"`
	DisplayName *string   `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Notification preferences
	PushNotificationsEnabled  bool    `gorm:"default:true" json:"push_notifications_enabled"`
	EmailNotificationsEnabled bool    `gorm:"default:false" json:"email_notifications_enabled"`
	SMSNotificationsEnabled   bool    `gorm:"default:false" json:"sms_notifications_enabled"`
	PhoneNumber              *string  `json:"phone_number"`
	NotificationEmail        *string  `json:"notification_email"`

	// Relationships
	UserAgents      []UserAgent      `gorm:"foreignKey:UserID" json:"user_agents,omitempty"`
	APIKeys         []APIKey         `gorm:"foreignKey:UserID" json:"api_keys,omitempty"`
	AgentInstances  []AgentInstance  `gorm:"foreignKey:UserID" json:"agent_instances,omitempty"`
	PushTokens      []PushToken      `gorm:"foreignKey:UserID" json:"push_tokens,omitempty"`
	TerminalSessions []TerminalSession `gorm:"foreignKey:UserID" json:"terminal_sessions,omitempty"`
}

// UserAgent represents a configured AI agent (from Omnara)
type UserAgent struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	Name           string     `gorm:"not null" json:"name"`
	WebhookURL     *string    `json:"webhook_url"`
	WebhookAPIKey  *string    `json:"webhook_api_key"` // Encrypted
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	IsDeleted      bool       `gorm:"default:false" json:"is_deleted"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Relationships
	User      User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Instances []AgentInstance `gorm:"foreignKey:UserAgentID" json:"instances,omitempty"`
}

// AgentInstance represents a running AI agent session (from Omnara)
type AgentInstance struct {
	ID               uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	UserAgentID      uuid.UUID  `gorm:"type:uuid;not null" json:"user_agent_id"`
	UserID           uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	Status           string     `gorm:"default:'active'" json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
	GitDiff          *string    `json:"git_diff"`
	Name             *string    `json:"name"`
	LastReadMessageID *uuid.UUID `gorm:"type:uuid" json:"last_read_message_id"`

	// Relationships
	UserAgent       UserAgent `gorm:"foreignKey:UserAgentID" json:"user_agent,omitempty"`
	User           User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Messages       []Message `gorm:"foreignKey:AgentInstanceID" json:"messages,omitempty"`
	LastReadMessage *Message  `gorm:"foreignKey:LastReadMessageID" json:"last_read_message,omitempty"`
}

// Message represents a message in agent communication (from Omnara)
type Message struct {
	ID               uuid.UUID              `gorm:"type:uuid;primary_key" json:"id"`
	AgentInstanceID  uuid.UUID              `gorm:"type:uuid;not null" json:"agent_instance_id"`
	SenderType       string                 `gorm:"not null" json:"sender_type"` // "AGENT" or "USER"
	Content          string                 `gorm:"type:text;not null" json:"content"`
	CreatedAt        time.Time              `json:"created_at"`
	RequiresUserInput bool                  `gorm:"default:false" json:"requires_user_input"`
	MessageMetadata  map[string]interface{} `gorm:"type:jsonb" json:"message_metadata"`

	// Relationships
	Instance AgentInstance `gorm:"foreignKey:AgentInstanceID" json:"instance,omitempty"`
}

// APIKey represents API keys for agent authentication (from Omnara)
type APIKey struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	Name       string     `gorm:"not null" json:"name"`
	APIKeyHash string     `gorm:"not null" json:"api_key_hash"`
	APIKey     string     `gorm:"type:text" json:"api_key"` // Store JWT for viewing
	IsActive   bool       `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// PushToken represents push notification tokens (from Omnara)
type PushToken struct {
	ID         uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	UserID     uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	Token      string     `gorm:"unique;not null" json:"token"`
	Platform   string     `gorm:"not null" json:"platform"` // 'ios' or 'android'
	IsActive   bool       `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at"`

	// Relationships
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TerminalSession represents a persistent terminal session (our new addition)
type TerminalSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	Name      string    `gorm:"not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Device info
	OwnerDeviceID   string    `gorm:"not null" json:"owner_device_id"`
	OwnerDeviceName string    `json:"owner_device_name"`
	CurrentDeviceID string    `json:"current_device_id"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`

	// Terminal state
	Command     string                 `json:"command"`
	Args        []string               `gorm:"type:jsonb" json:"args"`
	Environment map[string]string      `gorm:"type:jsonb" json:"environment"`
	WorkingDir  string                 `json:"working_dir"`

	// Buffer state
	BufferContent []byte `json:"buffer_content"`
	CursorRow     int    `json:"cursor_row"`
	CursorCol     int    `json:"cursor_col"`
	ScrollOffset  int    `json:"scroll_offset"`

	// Tool state
	ToolName  string                 `json:"tool_name"`
	ToolState map[string]interface{} `gorm:"type:jsonb" json:"tool_state"`

	// Metadata
	Status   string                 `gorm:"default:'created'" json:"status"`
	Tags     []string               `gorm:"type:jsonb" json:"tags"`
	Metadata map[string]interface{} `gorm:"type:jsonb" json:"metadata"`

	// Relationships
	User        User                 `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Checkpoints []SessionCheckpoint  `gorm:"foreignKey:SessionID" json:"checkpoints,omitempty"`
	Logs        []SessionLog         `gorm:"foreignKey:SessionID" json:"logs,omitempty"`
}

// SessionCheckpoint represents a session checkpoint for recovery
type SessionCheckpoint struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	SessionID uuid.UUID `gorm:"type:uuid;not null" json:"session_id"`
	Timestamp time.Time `json:"timestamp"`

	// History
	InputHistory  []map[string]interface{} `gorm:"type:jsonb" json:"input_history"`
	OutputHistory []map[string]interface{} `gorm:"type:jsonb" json:"output_history"`

	// State snapshot
	StateSnapshot map[string]interface{} `gorm:"type:jsonb" json:"state_snapshot"`

	// Relationships
	Session TerminalSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

// SessionLog represents session event logs
type SessionLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID uuid.UUID `gorm:"type:uuid;not null" json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // 'input', 'output', 'error', 'event'
	Content   string    `gorm:"type:text" json:"content"`
	Metadata  map[string]interface{} `gorm:"type:jsonb" json:"metadata"`

	// Relationships
	Session TerminalSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

// BeforeCreate hooks
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (ua *UserAgent) BeforeCreate(tx *gorm.DB) error {
	if ua.ID == uuid.Nil {
		ua.ID = uuid.New()
	}
	return nil
}

func (ai *AgentInstance) BeforeCreate(tx *gorm.DB) error {
	if ai.ID == uuid.Nil {
		ai.ID = uuid.New()
	}
	return nil
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (ak *APIKey) BeforeCreate(tx *gorm.DB) error {
	if ak.ID == uuid.Nil {
		ak.ID = uuid.New()
	}
	return nil
}

func (pt *PushToken) BeforeCreate(tx *gorm.DB) error {
	if pt.ID == uuid.Nil {
		pt.ID = uuid.New()
	}
	return nil
}

func (ts *TerminalSession) BeforeCreate(tx *gorm.DB) error {
	if ts.ID == uuid.Nil {
		ts.ID = uuid.New()
	}
	return nil
}

func (sc *SessionCheckpoint) BeforeCreate(tx *gorm.DB) error {
	if sc.ID == uuid.Nil {
		sc.ID = uuid.New()
	}
	return nil
}