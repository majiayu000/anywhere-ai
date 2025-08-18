package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/majiayu000/anywhere-ai/core/database"
	"gorm.io/gorm"
)

// MessageService handles message operations
type MessageService struct {
	db *gorm.DB
}

// NewMessageService creates a new message service
func NewMessageService(db *gorm.DB) *MessageService {
	return &MessageService{db: db}
}

// CreateAgentMessage creates a new agent message
func (s *MessageService) CreateAgentMessage(ctx context.Context, sessionID string, content string, requiresInput bool) (*database.TerminalMessage, error) {
	message := &database.TerminalMessage{
		ID:                uuid.New(),
		SessionID:         sessionID,
		SenderType:        database.SenderTypeAgent,
		Content:           content,
		RequiresUserInput: requiresInput,
		CreatedAt:         time.Now(),
		Metadata:          "", // Empty JSON string for SQLite
	}

	if err := s.db.WithContext(ctx).Create(message).Error; err != nil {
		return nil, fmt.Errorf("failed to create agent message: %w", err)
	}

	// Update message session unread count
	if err := s.incrementUnreadCount(ctx, sessionID); err != nil {
		return nil, err
	}

	return message, nil
}

// CreateUserMessage creates a new user message
func (s *MessageService) CreateUserMessage(ctx context.Context, sessionID string, content string, markAsRead bool) (*database.TerminalMessage, error) {
	message := &database.TerminalMessage{
		ID:                uuid.New(),
		SessionID:         sessionID,
		SenderType:        database.SenderTypeUser,
		Content:           content,
		RequiresUserInput: false,
		CreatedAt:         time.Now(),
		Metadata:          "", // Empty JSON string for SQLite
	}

	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(message).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create user message: %w", err)
	}

	// Mark as read if requested
	if markAsRead {
		if err := s.markAsReadTx(ctx, tx, sessionID, message.ID); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return message, nil
}

// GetMessages retrieves messages for a session
func (s *MessageService) GetMessages(ctx context.Context, sessionID string, limit int, offset int) ([]database.TerminalMessage, error) {
	var messages []database.TerminalMessage
	
	query := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	
	if err := query.Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	return messages, nil
}

// GetUnreadMessages retrieves unread messages for a session
func (s *MessageService) GetUnreadMessages(ctx context.Context, sessionID string) ([]database.TerminalMessage, error) {
	// Get or create message session
	messageSession, err := s.getOrCreateMessageSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var messages []database.TerminalMessage
	query := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at ASC")

	// If there's a last read message, get messages after it
	if messageSession.LastReadMessageID != nil {
		var lastReadMessage database.TerminalMessage
		if err := s.db.WithContext(ctx).
			Where("id = ?", messageSession.LastReadMessageID).
			First(&lastReadMessage).Error; err == nil {
			query = query.Where("created_at > ?", lastReadMessage.CreatedAt)
		}
	}

	if err := query.Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("failed to get unread messages: %w", err)
	}

	return messages, nil
}

// MarkAsRead marks messages as read up to a specific message ID
func (s *MessageService) MarkAsRead(ctx context.Context, sessionID string, messageID uuid.UUID) error {
	return s.markAsReadTx(ctx, s.db.WithContext(ctx), sessionID, messageID)
}

// markAsReadTx marks messages as read within a transaction
func (s *MessageService) markAsReadTx(ctx context.Context, tx *gorm.DB, sessionID string, messageID uuid.UUID) error {
	messageSession, err := s.getOrCreateMessageSessionTx(ctx, tx, sessionID)
	if err != nil {
		return err
	}

	// Update last read message ID
	messageSession.LastReadMessageID = &messageID
	messageSession.UnreadCount = 0
	messageSession.UpdatedAt = time.Now()

	if err := tx.Save(messageSession).Error; err != nil {
		return fmt.Errorf("failed to update message session: %w", err)
	}

	return nil
}

// GetMessageStatus retrieves the status of a message session
func (s *MessageService) GetMessageStatus(ctx context.Context, sessionID string) (*database.MessageStatus, error) {
	var totalMessages int64
	if err := s.db.WithContext(ctx).
		Model(&database.TerminalMessage{}).
		Where("session_id = ?", sessionID).
		Count(&totalMessages).Error; err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get last message
	var lastMessage database.TerminalMessage
	var lastMessageTime *time.Time
	var requiresInput bool
	
	if err := s.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("created_at DESC").
		First(&lastMessage).Error; err == nil {
		lastMessageTime = &lastMessage.CreatedAt
		requiresInput = lastMessage.RequiresUserInput
	}

	// Get unread count
	messageSession, err := s.getOrCreateMessageSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &database.MessageStatus{
		SessionID:         sessionID,
		TotalMessages:     totalMessages,
		UnreadMessages:    messageSession.UnreadCount,
		LastMessageTime:   lastMessageTime,
		RequiresUserInput: requiresInput,
	}, nil
}

// getOrCreateMessageSession gets or creates a message session
func (s *MessageService) getOrCreateMessageSession(ctx context.Context, sessionID string) (*database.MessageSession, error) {
	return s.getOrCreateMessageSessionTx(ctx, s.db.WithContext(ctx), sessionID)
}

// getOrCreateMessageSessionTx gets or creates a message session within a transaction
func (s *MessageService) getOrCreateMessageSessionTx(ctx context.Context, tx *gorm.DB, sessionID string) (*database.MessageSession, error) {
	var messageSession database.MessageSession
	
	err := tx.Where("session_id = ?", sessionID).First(&messageSession).Error
	if err == gorm.ErrRecordNotFound {
		// Create new message session
		messageSession = database.MessageSession{
			SessionID: sessionID,
			UpdatedAt: time.Now(),
		}
		if err := tx.Create(&messageSession).Error; err != nil {
			return nil, fmt.Errorf("failed to create message session: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get message session: %w", err)
	}

	return &messageSession, nil
}

// incrementUnreadCount increments the unread count for a session
func (s *MessageService) incrementUnreadCount(ctx context.Context, sessionID string) error {
	messageSession, err := s.getOrCreateMessageSession(ctx, sessionID)
	if err != nil {
		return err
	}

	messageSession.UnreadCount++
	messageSession.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Save(messageSession).Error; err != nil {
		return fmt.Errorf("failed to update unread count: %w", err)
	}

	return nil
}

// GetQueuedUserMessages retrieves queued user messages since last read
func (s *MessageService) GetQueuedUserMessages(ctx context.Context, sessionID string) ([]database.TerminalMessage, error) {
	messageSession, err := s.getOrCreateMessageSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	var messages []database.TerminalMessage
	query := s.db.WithContext(ctx).
		Where("session_id = ? AND sender_type = ?", sessionID, database.SenderTypeUser).
		Order("created_at ASC")

	// Get messages after last read
	if messageSession.LastReadMessageID != nil {
		var lastReadMessage database.TerminalMessage
		if err := s.db.WithContext(ctx).
			Where("id = ?", messageSession.LastReadMessageID).
			First(&lastReadMessage).Error; err == nil {
			query = query.Where("created_at > ?", lastReadMessage.CreatedAt)
		}
	}

	if err := query.Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("failed to get queued user messages: %w", err)
	}

	return messages, nil
}

// SendAgentMessageWithQueue sends an agent message and returns queued user messages
func (s *MessageService) SendAgentMessageWithQueue(ctx context.Context, sessionID string, content string, requiresInput bool) (*database.TerminalMessage, []database.TerminalMessage, error) {
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create agent message
	message := &database.TerminalMessage{
		ID:                uuid.New(),
		SessionID:         sessionID,
		SenderType:        database.SenderTypeAgent,
		Content:           content,
		RequiresUserInput: requiresInput,
		CreatedAt:         time.Now(),
		Metadata:          "", // Empty JSON string for SQLite
	}

	if err := tx.Create(message).Error; err != nil {
		tx.Rollback()
		return nil, nil, fmt.Errorf("failed to create agent message: %w", err)
	}

	// Get queued user messages
	messageSession, err := s.getOrCreateMessageSessionTx(ctx, tx, sessionID)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	var queuedMessages []database.TerminalMessage
	query := tx.Where("session_id = ? AND sender_type = ?", sessionID, database.SenderTypeUser).
		Order("created_at ASC")

	if messageSession.LastReadMessageID != nil {
		var lastReadMessage database.TerminalMessage
		if err := tx.Where("id = ?", messageSession.LastReadMessageID).
			First(&lastReadMessage).Error; err == nil {
			query = query.Where("created_at > ?", lastReadMessage.CreatedAt)
		}
	}

	if err := query.Find(&queuedMessages).Error; err != nil {
		tx.Rollback()
		return nil, nil, fmt.Errorf("failed to get queued messages: %w", err)
	}

	// Mark this message as the last read
	messageSession.LastReadMessageID = &message.ID
	messageSession.UnreadCount = 0
	messageSession.UpdatedAt = time.Now()
	
	if err := tx.Save(messageSession).Error; err != nil {
		tx.Rollback()
		return nil, nil, fmt.Errorf("failed to update message session: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return message, queuedMessages, nil
}