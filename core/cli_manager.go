package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CLIManager 管理所有AI CLI工具的完整生命周期
// 基于Omnara的AgentInstance模式 + 我们的跨设备终端恢复创新
type CLIManager struct {
	db *sql.DB

	// 活跃实例缓存 (内存中)
	activeInstances map[string]*AgentInstance
	mu              sync.RWMutex

	// 工具适配器
	toolAdapters map[string]ToolAdapter

	// 消息管理器
	messageManager *MessageManager

	// 工具包装器管理
	wrapperManager *ToolWrapperManager
}

// AgentInstance 对应Omnara的agent_instances表 + 我们的扩展
type AgentInstance struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	ToolName        string                 `json:"tool_name"`
	Status          string                 `json:"status"`
	Name            string                 `json:"name,omitempty"`

	// 设备信息 (我们的创新)
	OwnerDeviceID   string `json:"owner_device_id"`
	CurrentDeviceID string `json:"current_device_id,omitempty"`

	// 时间戳
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	LastActivityAt  time.Time  `json:"last_activity_at"`

	// 状态数据 (我们的扩展 - 存储完整终端状态)
	SessionState    map[string]interface{} `json:"session_state"`
	GitDiff         string                 `json:"git_diff,omitempty"`
	InitialGitHash  string                 `json:"initial_git_hash,omitempty"`
	PermissionState map[string]interface{} `json:"permission_state"`

	// 运行时引用 (不持久化)
	wrapper    *ToolWrapper `json:"-"`
	isActive   bool         `json:"-"`
	lastReadMessageID string `json:"-"`
}

// MessageManager 管理所有消息 (学习Omnara)
type MessageManager struct {
	db *sql.DB
}

// Message 对应Omnara的messages表
type Message struct {
	ID              string                 `json:"id"`
	AgentInstanceID string                 `json:"agent_instance_id"`
	SenderType      string                 `json:"sender_type"` // USER or AGENT
	Content         string                 `json:"content"`
	RequiresInput   bool                   `json:"requires_user_input"`
	GitDiff         string                 `json:"git_diff,omitempty"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       time.Time              `json:"created_at"`
}

// ToolWrapperManager 管理工具包装器
type ToolWrapperManager struct {
	wrappers map[string]*ToolWrapper
	mu       sync.RWMutex
}

// ToolWrapper 工具包装器 (类似Omnara的claude_wrapper_v3.py)
type ToolWrapper struct {
	ID          string
	SessionID   string
	ToolName    string
	ProcessID   int
	
	// PTY管理 (我们的创新)
	ptyInstance *PTYInstance
	
	// 状态管理
	Running     bool
	State       map[string]interface{}
	
	// 权限状态 (学习Omnara)
	PermissionState map[string]interface{}
	
	// 消息处理 (学习Omnara)
	messageProcessor *MessageProcessor
}

// PTYInstance PTY实例管理
type PTYInstance struct {
	SessionID      string
	MasterFD       int
	ChildPID       int
	TerminalBuffer []byte
	InputBuffer    []byte
	LastEscSeen    time.Time
	IsIdle         bool
	InputStream    chan []byte
	OutputStream   chan []byte
}

// MessageProcessor 消息处理器 (学习Omnara)
type MessageProcessor struct {
	sessionID        string
	lastMessageID    string
	lastMessageTime  time.Time
	webUIMessages    map[string]bool
	permissionState  map[string]interface{}
	inputQueue       []string
}

// 请求和响应类型
type CreateAgentInstanceRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	ToolName string `json:"tool_name" binding:"required"`
	Name     string `json:"name"`
	DeviceID string `json:"device_id" binding:"required"`
}

type RestoreSessionRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	DeviceID   string `json:"device_id" binding:"required"`
}

// NewCLIManager 创建新的CLIManager
func NewCLIManager(db *sql.DB) *CLIManager {
	manager := &CLIManager{
		db:              db,
		activeInstances: make(map[string]*AgentInstance),
		toolAdapters:    make(map[string]ToolAdapter),
		messageManager:  NewMessageManager(db),
		wrapperManager:  NewToolWrapperManager(),
	}

	// 注册工具适配器
	manager.registerToolAdapters()

	// 启动清理协程
	go manager.startCleanupRoutine()

	return manager
}

// 注册所有工具适配器
func (cm *CLIManager) registerToolAdapters() {
	// 这里会注册Claude、Gemini、Cursor等适配器
	// 暂时用占位符
	cm.toolAdapters["claude"] = &ClaudeAdapter{}
	cm.toolAdapters["gemini"] = &GeminiAdapter{}
	cm.toolAdapters["cursor"] = &CursorAdapter{}
}

// CreateAgentInstance 创建新的AgentInstance
func (cm *CLIManager) CreateAgentInstance(req *CreateAgentInstanceRequest) (*AgentInstance, error) {
	// 1. 验证工具适配器是否存在
	adapter, exists := cm.toolAdapters[req.ToolName]
	if !exists {
		return nil, fmt.Errorf("unsupported tool: %s", req.ToolName)
	}

	// 2. 创建AgentInstance
	instance := &AgentInstance{
		ID:              uuid.New().String(),
		UserID:          req.UserID,
		ToolName:        req.ToolName,
		Status:          "active",
		Name:            req.Name,
		OwnerDeviceID:   req.DeviceID,
		CurrentDeviceID: req.DeviceID,
		StartedAt:       time.Now(),
		LastActivityAt:  time.Now(),
		SessionState:    make(map[string]interface{}),
		PermissionState: make(map[string]interface{}),
		isActive:        true,
	}

	// 3. 保存到数据库
	if err := cm.saveInstanceToDB(instance); err != nil {
		return nil, fmt.Errorf("failed to save instance to DB: %w", err)
	}

	// 4. 创建工具包装器
	wrapper, err := cm.wrapperManager.CreateWrapper(instance, adapter)
	if err != nil {
		// 回滚数据库
		cm.deleteInstanceFromDB(instance.ID)
		return nil, fmt.Errorf("failed to create wrapper: %w", err)
	}

	instance.wrapper = wrapper

	// 5. 加入内存缓存
	cm.mu.Lock()
	cm.activeInstances[instance.ID] = instance
	cm.mu.Unlock()

	// 6. 发送初始消息
	cm.messageManager.SendMessage(&Message{
		ID:              uuid.New().String(),
		AgentInstanceID: instance.ID,
		SenderType:      "AGENT",
		Content:         fmt.Sprintf("Started %s session: %s", req.ToolName, instance.ID),
		RequiresInput:   false,
		Metadata:        map[string]interface{}{"type": "session_start"},
		CreatedAt:       time.Now(),
	})

	log.Printf("Created AgentInstance: %s (%s) for user %s", instance.ID, instance.ToolName, instance.UserID)
	return instance, nil
}

// RestoreAgentInstance 恢复AgentInstance (跨设备)
func (cm *CLIManager) RestoreAgentInstance(instanceID string, newDeviceID string) (*AgentInstance, error) {
	// 1. 从数据库加载
	instance, err := cm.loadInstanceFromDB(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load instance: %w", err)
	}

	// 2. 获取工具适配器
	adapter, exists := cm.toolAdapters[instance.ToolName]
	if !exists {
		return nil, fmt.Errorf("tool adapter not found: %s", instance.ToolName)
	}

	// 3. 更新设备归属
	instance.CurrentDeviceID = newDeviceID
	instance.LastActivityAt = time.Now()
	instance.isActive = true

	// 4. 重建工具包装器 (恢复状态)
	wrapper, err := cm.wrapperManager.RestoreWrapper(instance, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to restore wrapper: %w", err)
	}

	instance.wrapper = wrapper

	// 5. 更新数据库
	if err := cm.updateInstanceInDB(instance); err != nil {
		return nil, fmt.Errorf("failed to update instance: %w", err)
	}

	// 6. 加入内存缓存
	cm.mu.Lock()
	cm.activeInstances[instanceID] = instance
	cm.mu.Unlock()

	// 7. 获取最近消息历史
	messages, err := cm.messageManager.GetRecentMessages(instanceID, 50)
	if err != nil {
		log.Printf("Failed to get recent messages: %v", err)
	} else {
		// 重放消息到新设备 (如果需要)
		log.Printf("Restored %d messages for session %s", len(messages), instanceID)
	}

	// 8. 发送恢复消息
	cm.messageManager.SendMessage(&Message{
		ID:              uuid.New().String(),
		AgentInstanceID: instanceID,
		SenderType:      "AGENT",
		Content:         fmt.Sprintf("Session restored on device: %s", newDeviceID),
		RequiresInput:   false,
		Metadata:        map[string]interface{}{"type": "session_restore", "device_id": newDeviceID},
		CreatedAt:       time.Now(),
	})

	log.Printf("Restored AgentInstance: %s to device %s", instanceID, newDeviceID)
	return instance, nil
}

// GetAgentInstance 获取AgentInstance
func (cm *CLIManager) GetAgentInstance(instanceID string) (*AgentInstance, error) {
	// 先从内存缓存查找
	cm.mu.RLock()
	instance, exists := cm.activeInstances[instanceID]
	cm.mu.RUnlock()

	if exists {
		return instance, nil
	}

	// 内存中没有，从数据库加载
	return cm.loadInstanceFromDB(instanceID)
}

// ListUserInstances 列出用户的所有实例
func (cm *CLIManager) ListUserInstances(userID string) ([]*AgentInstance, error) {
	query := `
        SELECT id, user_id, tool_name, status, name, owner_device_id, current_device_id,
               started_at, ended_at, last_activity_at, session_state, git_diff, 
               initial_git_hash, permission_state
        FROM agent_instances 
        WHERE user_id = $1 
        ORDER BY last_activity_at DESC
    `

	rows, err := cm.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user instances: %w", err)
	}
	defer rows.Close()

	var instances []*AgentInstance
	for rows.Next() {
		instance, err := cm.scanAgentInstance(rows)
		if err != nil {
			log.Printf("Failed to scan instance: %v", err)
			continue
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// SendMessage 发送消息到AgentInstance
func (cm *CLIManager) SendMessage(instanceID string, message *Message) error {
	// 1. 获取实例
	instance, err := cm.GetAgentInstance(instanceID)
	if err != nil {
		return err
	}

	// 2. 更新活跃时间
	cm.KeepAlive(instanceID)

	// 3. 处理消息
	if instance.wrapper != nil && instance.wrapper.messageProcessor != nil {
		instance.wrapper.messageProcessor.ProcessMessage(message)
	}

	// 4. 发送到工具
	if instance.wrapper != nil && instance.wrapper.Running {
		if err := instance.wrapper.SendInput(message.Content); err != nil {
			log.Printf("Failed to send input to wrapper: %v", err)
		}
	}

	// 5. 保存消息到数据库
	return cm.messageManager.SendMessage(message)
}

// KeepAlive 保持AgentInstance活跃状态
func (cm *CLIManager) KeepAlive(instanceID string) error {
	cm.mu.RLock()
	instance, exists := cm.activeInstances[instanceID]
	cm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	// 更新活跃时间
	instance.LastActivityAt = time.Now()

	// 异步更新数据库
	go func() {
		if err := cm.updateInstanceActivityInDB(instanceID, time.Now()); err != nil {
			log.Printf("Failed to update instance activity: %v", err)
		}
	}()

	return nil
}

// EndAgentInstance 结束AgentInstance
func (cm *CLIManager) EndAgentInstance(instanceID string) error {
	cm.mu.Lock()
	instance, exists := cm.activeInstances[instanceID]
	if exists {
		delete(cm.activeInstances, instanceID)
	}
	cm.mu.Unlock()

	if !exists {
		// 从数据库加载
		var err error
		instance, err = cm.loadInstanceFromDB(instanceID)
		if err != nil {
			return fmt.Errorf("instance not found: %s", instanceID)
		}
	}

	// 1. 停止Wrapper
	if instance.wrapper != nil {
		if err := instance.wrapper.Stop(); err != nil {
			log.Printf("Failed to stop wrapper for instance %s: %v", instanceID, err)
		}
	}

	// 2. 更新状态并保存
	now := time.Now()
	instance.Status = "ended"
	instance.EndedAt = &now

	if err := cm.updateInstanceInDB(instance); err != nil {
		return fmt.Errorf("failed to update instance status: %w", err)
	}

	// 3. 发送结束消息
	cm.messageManager.SendMessage(&Message{
		ID:              uuid.New().String(),
		AgentInstanceID: instanceID,
		SenderType:      "AGENT",
		Content:         "Session ended",
		RequiresInput:   false,
		Metadata:        map[string]interface{}{"type": "session_end"},
		CreatedAt:       time.Now(),
	})

	log.Printf("Ended AgentInstance: %s", instanceID)
	return nil
}

// 数据库操作方法... (保持原有的实现)

// saveInstanceToDB 保存实例到数据库
func (cm *CLIManager) saveInstanceToDB(instance *AgentInstance) error {
	sessionStateJSON, _ := json.Marshal(instance.SessionState)
	permissionStateJSON, _ := json.Marshal(instance.PermissionState)

	query := `
        INSERT INTO agent_instances (
            id, user_id, tool_name, status, name, owner_device_id, current_device_id,
            started_at, last_activity_at, session_state, git_diff, initial_git_hash,
            permission_state
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `

	_, err := cm.db.Exec(query,
		instance.ID, instance.UserID, instance.ToolName, instance.Status, instance.Name,
		instance.OwnerDeviceID, instance.CurrentDeviceID, instance.StartedAt,
		instance.LastActivityAt, sessionStateJSON, instance.GitDiff,
		instance.InitialGitHash, permissionStateJSON,
	)

	return err
}

// loadInstanceFromDB 从数据库加载实例
func (cm *CLIManager) loadInstanceFromDB(instanceID string) (*AgentInstance, error) {
	query := `
        SELECT id, user_id, tool_name, status, name, owner_device_id, current_device_id,
               started_at, ended_at, last_activity_at, session_state, git_diff,
               initial_git_hash, permission_state
        FROM agent_instances WHERE id = $1
    `

	row := cm.db.QueryRow(query, instanceID)
	return cm.scanAgentInstance(row)
}

// scanAgentInstance 扫描数据库行到AgentInstance结构
func (cm *CLIManager) scanAgentInstance(scanner interface{}) (*AgentInstance, error) {
	var instance AgentInstance
	var sessionStateJSON, permissionStateJSON []byte
	var currentDeviceID sql.NullString
	var endedAt sql.NullTime
	var name sql.NullString

	var err error

	// 根据scanner类型处理
	switch s := scanner.(type) {
	case *sql.Row:
		err = s.Scan(&instance.ID, &instance.UserID, &instance.ToolName, &instance.Status,
			&name, &instance.OwnerDeviceID, &currentDeviceID, &instance.StartedAt,
			&endedAt, &instance.LastActivityAt, &sessionStateJSON, &instance.GitDiff,
			&instance.InitialGitHash, &permissionStateJSON)
	case *sql.Rows:
		err = s.Scan(&instance.ID, &instance.UserID, &instance.ToolName, &instance.Status,
			&name, &instance.OwnerDeviceID, &currentDeviceID, &instance.StartedAt,
			&endedAt, &instance.LastActivityAt, &sessionStateJSON, &instance.GitDiff,
			&instance.InitialGitHash, &permissionStateJSON)
	default:
		return nil, fmt.Errorf("unsupported scanner type")
	}

	if err != nil {
		return nil, err
	}

	// 处理可空字段
	if name.Valid {
		instance.Name = name.String
	}
	if currentDeviceID.Valid {
		instance.CurrentDeviceID = currentDeviceID.String
	}
	if endedAt.Valid {
		instance.EndedAt = &endedAt.Time
	}

	// 解析JSON字段
	if sessionStateJSON != nil {
		if err := json.Unmarshal(sessionStateJSON, &instance.SessionState); err != nil {
			instance.SessionState = make(map[string]interface{})
		}
	} else {
		instance.SessionState = make(map[string]interface{})
	}

	if permissionStateJSON != nil {
		if err := json.Unmarshal(permissionStateJSON, &instance.PermissionState); err != nil {
			instance.PermissionState = make(map[string]interface{})
		}
	} else {
		instance.PermissionState = make(map[string]interface{})
	}

	return &instance, nil
}

// updateInstanceInDB 更新实例到数据库
func (cm *CLIManager) updateInstanceInDB(instance *AgentInstance) error {
	sessionStateJSON, _ := json.Marshal(instance.SessionState)
	permissionStateJSON, _ := json.Marshal(instance.PermissionState)

	query := `
        UPDATE agent_instances SET
            status = $2, name = $3, current_device_id = $4, ended_at = $5,
            last_activity_at = $6, session_state = $7, git_diff = $8,
            initial_git_hash = $9, permission_state = $10
        WHERE id = $1
    `

	_, err := cm.db.Exec(query,
		instance.ID, instance.Status, instance.Name, instance.CurrentDeviceID,
		instance.EndedAt, instance.LastActivityAt, sessionStateJSON,
		instance.GitDiff, instance.InitialGitHash, permissionStateJSON,
	)

	return err
}

// updateInstanceActivityInDB 更新实例活跃时间
func (cm *CLIManager) updateInstanceActivityInDB(instanceID string, activityTime time.Time) error {
	query := `UPDATE agent_instances SET last_activity_at = $2 WHERE id = $1`
	_, err := cm.db.Exec(query, instanceID, activityTime)
	return err
}

// deleteInstanceFromDB 从数据库删除实例
func (cm *CLIManager) deleteInstanceFromDB(instanceID string) error {
	query := `DELETE FROM agent_instances WHERE id = $1`
	_, err := cm.db.Exec(query, instanceID)
	return err
}

// startCleanupRoutine 启动清理协程
func (cm *CLIManager) startCleanupRoutine() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cm.cleanupInactiveInstances()
	}
}

// cleanupInactiveInstances 清理不活跃的实例
func (cm *CLIManager) cleanupInactiveInstances() {
	// 查找超过24小时不活跃的实例
	inactiveThreshold := time.Now().Add(-24 * time.Hour)

	query := `
        SELECT id FROM agent_instances 
        WHERE status = 'active' 
        AND last_activity_at < $1
    `

	rows, err := cm.db.Query(query, inactiveThreshold)
	if err != nil {
		log.Printf("Failed to query inactive instances: %v", err)
		return
	}
	defer rows.Close()

	var inactiveIDs []string
	for rows.Next() {
		var instanceID string
		if err := rows.Scan(&instanceID); err != nil {
			continue
		}
		inactiveIDs = append(inactiveIDs, instanceID)
	}

	// 结束不活跃的实例
	for _, instanceID := range inactiveIDs {
		if err := cm.EndAgentInstance(instanceID); err != nil {
			log.Printf("Failed to end inactive instance %s: %v", instanceID, err)
		} else {
			log.Printf("Ended inactive instance: %s", instanceID)
		}
	}

	if len(inactiveIDs) > 0 {
		log.Printf("Cleaned up %d inactive instances", len(inactiveIDs))
	}
}

// 辅助组件实现

// NewMessageManager 创建消息管理器
func NewMessageManager(db *sql.DB) *MessageManager {
	return &MessageManager{db: db}
}

// SendMessage 发送消息
func (mm *MessageManager) SendMessage(message *Message) error {
	metadataJSON, _ := json.Marshal(message.Metadata)

	query := `
        INSERT INTO messages (
            id, agent_instance_id, sender_type, content, requires_user_input,
            git_diff, metadata, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `

	_, err := mm.db.Exec(query,
		message.ID, message.AgentInstanceID, message.SenderType, message.Content,
		message.RequiresInput, message.GitDiff, metadataJSON, message.CreatedAt,
	)

	return err
}

// GetRecentMessages 获取最近的消息
func (mm *MessageManager) GetRecentMessages(instanceID string, limit int) ([]*Message, error) {
	query := `
        SELECT id, agent_instance_id, sender_type, content, requires_user_input,
               git_diff, metadata, created_at
        FROM messages 
        WHERE agent_instance_id = $1 
        ORDER BY created_at DESC 
        LIMIT $2
    `

	rows, err := mm.db.Query(query, instanceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var message Message
		var metadataJSON []byte

		err := rows.Scan(&message.ID, &message.AgentInstanceID, &message.SenderType,
			&message.Content, &message.RequiresInput, &message.GitDiff,
			&metadataJSON, &message.CreatedAt)
		if err != nil {
			continue
		}

		// 解析metadata
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &message.Metadata)
		} else {
			message.Metadata = make(map[string]interface{})
		}

		messages = append(messages, &message)
	}

	return messages, nil
}

// NewToolWrapperManager 创建工具包装器管理器
func NewToolWrapperManager() *ToolWrapperManager {
	return &ToolWrapperManager{
		wrappers: make(map[string]*ToolWrapper),
	}
}

// CreateWrapper 创建工具包装器
func (twm *ToolWrapperManager) CreateWrapper(instance *AgentInstance, adapter ToolAdapter) (*ToolWrapper, error) {
	wrapper := &ToolWrapper{
		ID:               uuid.New().String(),
		SessionID:        instance.ID,
		ToolName:         instance.ToolName,
		Running:          false,
		State:            make(map[string]interface{}),
		PermissionState:  instance.PermissionState,
		messageProcessor: &MessageProcessor{
			sessionID:       instance.ID,
			webUIMessages:   make(map[string]bool),
			permissionState: instance.PermissionState,
		},
	}

	// 启动工具进程 (简化实现)
	if err := wrapper.Start(); err != nil {
		return nil, err
	}

	twm.mu.Lock()
	twm.wrappers[instance.ID] = wrapper
	twm.mu.Unlock()

	return wrapper, nil
}

// RestoreWrapper 恢复工具包装器
func (twm *ToolWrapperManager) RestoreWrapper(instance *AgentInstance, adapter ToolAdapter) (*ToolWrapper, error) {
	// 基本上和CreateWrapper相同，但恢复状态
	wrapper, err := twm.CreateWrapper(instance, adapter)
	if err != nil {
		return nil, err
	}

	// 恢复状态
	wrapper.State = instance.SessionState
	wrapper.PermissionState = instance.PermissionState

	return wrapper, nil
}

// ToolWrapper方法

// Start 启动工具
func (tw *ToolWrapper) Start() error {
	// 这里实现具体的工具启动逻辑
	tw.Running = true
	log.Printf("Started tool wrapper: %s (%s)", tw.ToolName, tw.ID)
	return nil
}

// Stop 停止工具
func (tw *ToolWrapper) Stop() error {
	tw.Running = false
	log.Printf("Stopped tool wrapper: %s", tw.ID)
	return nil
}

// SendInput 发送输入到工具
func (tw *ToolWrapper) SendInput(input string) error {
	if !tw.Running {
		return fmt.Errorf("tool is not running")
	}
	
	// 这里实现具体的输入发送逻辑
	log.Printf("Sending input to %s: %s", tw.ToolName, input)
	return nil
}

// MessageProcessor方法

// ProcessMessage 处理消息
func (mp *MessageProcessor) ProcessMessage(message *Message) {
	// 处理消息逻辑
	log.Printf("Processing message for session %s: %s", mp.sessionID, message.Content)
}

// 占位符适配器
type ClaudeAdapter struct{}
type GeminiAdapter struct{}
type CursorAdapter struct{}

func (c *ClaudeAdapter) GetName() string { return "Claude Code" }
func (g *GeminiAdapter) GetName() string { return "Gemini CLI" }  
func (cu *CursorAdapter) GetName() string { return "Cursor" }