package database

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SenderType represents who sent the message
type SenderType string

const (
	SenderTypeAgent SenderType = "AGENT"
	SenderTypeUser  SenderType = "USER"
)

// TerminalMessage represents a message in terminal conversation
// This extends the base Message model with terminal-specific fields
type TerminalMessage struct {
	ID                uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	SessionID         string         `gorm:"not null;index:idx_terminal_messages_session_created" json:"session_id"`
	SenderType        SenderType     `gorm:"type:varchar(10);not null" json:"sender_type"`
	Content           string         `gorm:"type:text;not null" json:"content"`
	RequiresUserInput bool           `gorm:"default:false" json:"requires_user_input"`
	Metadata          string         `gorm:"type:text" json:"metadata,omitempty"` // Store as JSON string for SQLite
	CreatedAt         time.Time      `gorm:"default:current_timestamp" json:"created_at"`
	
	// Foreign key to TerminalSession
	Session           *TerminalSession `gorm:"foreignKey:SessionID;references:ID" json:"-"`
}

// MessageSession tracks the reading progress for a session
type MessageSession struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	SessionID          string     `gorm:"unique;not null" json:"session_id"`
	LastReadMessageID  *uuid.UUID `gorm:"type:uuid" json:"last_read_message_id,omitempty"`
	UnreadCount        int        `gorm:"default:0" json:"unread_count"`
	UpdatedAt          time.Time  `gorm:"default:current_timestamp" json:"updated_at"`
	
	// Foreign keys
	Session            *TerminalSession  `gorm:"foreignKey:SessionID;references:ID" json:"-"`
	LastReadMessage    *TerminalMessage  `gorm:"foreignKey:LastReadMessageID" json:"-"`
}

// JSON type for JSONB fields
type JSON map[string]interface{}

// TableName sets the table name for TerminalMessage
func (TerminalMessage) TableName() string {
	return "terminal_messages"
}

// TableName sets the table name for MessageSession
func (MessageSession) TableName() string {
	return "message_sessions"
}

// BeforeCreate hook for TerminalMessage
func (m *TerminalMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	return nil
}

// BeforeCreate hook for MessageSession
func (ms *MessageSession) BeforeCreate(tx *gorm.DB) error {
	if ms.ID == uuid.Nil {
		ms.ID = uuid.New()
	}
	if ms.UpdatedAt.IsZero() {
		ms.UpdatedAt = time.Now()
	}
	return nil
}

// MessageStatus represents the status of a message session
type MessageStatus struct {
	SessionID         string     `json:"session_id"`
	TotalMessages     int64      `json:"total_messages"`
	UnreadMessages    int        `json:"unread_messages"`
	LastMessageTime   *time.Time `json:"last_message_time,omitempty"`
	RequiresUserInput bool       `json:"requires_user_input"`
}