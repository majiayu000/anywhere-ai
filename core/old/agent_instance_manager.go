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

// AgentInstanceManager 管理AgentInstance的完整生命周期
// 基于Omnara的AgentInstance模式，简化版本
type AgentInstanceManager struct {
	db *sql.DB

	// 活跃实例缓存 (内存中)
	activeInstances map[string]*AgentInstance
	mu              sync.RWMutex
}

// AgentInstance 对应数据库模型
type AgentInstance struct {
	ID              string                 `json:"id"`
	UserID          string                 `json:"user_id"`
	ToolName        string                 `json:"tool_name"`
	Status          string                 `json:"status"`
	Name            string                 `json:"name,omitempty"`

	// 设备信息
	OwnerDeviceID   string `json:"owner_device_id"`
	CurrentDeviceID string `json:"current_device_id,omitempty"`

	// 时间戳
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	LastActivityAt  time.Time  `json:"last_activity_at"`

	// 状态数据
	SessionState    map[string]interface{} `json:"session_state"`
	GitDiff         string                 `json:"git_diff,omitempty"`
	InitialGitHash  string                 `json:"initial_git_hash,omitempty"`
	PermissionState map[string]interface{} `json:"permission_state"`

	// 运行时状态 (不持久化到数据库)
	isActive bool `json:"-"`
}

// CreateAgentInstanceRequest 创建请求
type CreateAgentInstanceRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	ToolName string `json:"tool_name" binding:"required"`
	Name     string `json:"name"`
	DeviceID string `json:"device_id" binding:"required"`
}

// RestoreSessionRequest 恢复请求
type RestoreSessionRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	DeviceID   string `json:"device_id" binding:"required"`
}

// NewAgentInstanceManager 创建新的AgentInstanceManager
func NewAgentInstanceManager(db *sql.DB) *AgentInstanceManager {
	aim := &AgentInstanceManager{
		db:              db,
		activeInstances: make(map[string]*AgentInstance),
	}

	// 启动清理协程
	go aim.startCleanupRoutine()

	return aim
}

// CreateAgentInstance 创建新的AgentInstance
func (aim *AgentInstanceManager) CreateAgentInstance(req *CreateAgentInstanceRequest) (*AgentInstance, error) {
	// 1. 创建数据库记录
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
	}

	// 2. 保存到数据库
	if err := aim.saveInstanceToDB(instance); err != nil {
		return nil, fmt.Errorf("failed to save instance to DB: %w", err)
	}

	// 3. 启动工具Wrapper
	wrapper, err := aim.wrapperManager.CreateWrapper(instance)
	if err != nil {
		// 回滚数据库
		aim.deleteInstanceFromDB(instance.ID)
		return nil, fmt.Errorf("failed to create wrapper: %w", err)
	}

	instance.wrapper = wrapper

	// 4. 加入内存缓存
	aim.mu.Lock()
	aim.activeInstances[instance.ID] = instance
	aim.mu.Unlock()

	log.Printf("Created AgentInstance: %s (%s) for user %s", instance.ID, instance.ToolName, instance.UserID)
	return instance, nil
}

// RestoreAgentInstance 恢复AgentInstance (跨设备)
func (aim *AgentInstanceManager) RestoreAgentInstance(instanceID string, newDeviceID string) (*AgentInstance, error) {
	// 1. 从数据库加载
	instance, err := aim.loadInstanceFromDB(instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load instance: %w", err)
	}

	// 2. 更新设备归属
	instance.CurrentDeviceID = newDeviceID
	instance.LastActivityAt = time.Now()

	// 3. 重建工具Wrapper
	wrapper, err := aim.wrapperManager.RestoreWrapper(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to restore wrapper: %w", err)
	}

	instance.wrapper = wrapper

	// 4. 更新数据库
	if err := aim.updateInstanceInDB(instance); err != nil {
		return nil, fmt.Errorf("failed to update instance: %w", err)
	}

	// 5. 加入内存缓存
	aim.mu.Lock()
	aim.activeInstances[instanceID] = instance
	aim.mu.Unlock()

	log.Printf("Restored AgentInstance: %s to device %s", instanceID, newDeviceID)
	return instance, nil
}

// GetAgentInstance 获取AgentInstance
func (aim *AgentInstanceManager) GetAgentInstance(instanceID string) (*AgentInstance, error) {
	// 先从内存缓存查找
	aim.mu.RLock()
	instance, exists := aim.activeInstances[instanceID]
	aim.mu.RUnlock()

	if exists {
		return instance, nil
	}

	// 内存中没有，从数据库加载
	return aim.loadInstanceFromDB(instanceID)
}

// ListUserInstances 列出用户的所有实例
func (aim *AgentInstanceManager) ListUserInstances(userID string) ([]*AgentInstance, error) {
	query := `
        SELECT id, user_id, tool_name, status, name, owner_device_id, current_device_id,
               started_at, ended_at, last_activity_at, session_state, git_diff, 
               initial_git_hash, permission_state
        FROM agent_instances 
        WHERE user_id = $1 
        ORDER BY last_activity_at DESC
    `

	rows, err := aim.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user instances: %w", err)
	}
	defer rows.Close()

	var instances []*AgentInstance
	for rows.Next() {
		instance, err := aim.scanAgentInstance(rows)
		if err != nil {
			log.Printf("Failed to scan instance: %v", err)
			continue
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// KeepAlive 保持AgentInstance活跃状态
func (aim *AgentInstanceManager) KeepAlive(instanceID string) error {
	aim.mu.RLock()
	instance, exists := aim.activeInstances[instanceID]
	aim.mu.RUnlock()

	if !exists {
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	// 更新活跃时间
	instance.LastActivityAt = time.Now()

	// 异步更新数据库
	go func() {
		if err := aim.updateInstanceActivityInDB(instanceID, time.Now()); err != nil {
			log.Printf("Failed to update instance activity: %v", err)
		}
	}()

	return nil
}

// PauseAgentInstance 暂停AgentInstance (保持状态但停止工具)
func (aim *AgentInstanceManager) PauseAgentInstance(instanceID string) error {
	aim.mu.Lock()
	instance, exists := aim.activeInstances[instanceID]
	aim.mu.Unlock()

	if !exists {
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	// 1. 保存当前状态
	if err := aim.saveInstanceState(instance); err != nil {
		return fmt.Errorf("failed to save instance state: %w", err)
	}

	// 2. 停止Wrapper但不销毁
	if instance.wrapper != nil {
		if err := instance.wrapper.Pause(); err != nil {
			return fmt.Errorf("failed to pause wrapper: %w", err)
		}
	}

	// 3. 更新状态
	instance.Status = "paused"

	// 4. 更新数据库
	if err := aim.updateInstanceInDB(instance); err != nil {
		return fmt.Errorf("failed to update instance in DB: %w", err)
	}

	log.Printf("Paused AgentInstance: %s", instanceID)
	return nil
}

// ResumeAgentInstance 恢复暂停的AgentInstance
func (aim *AgentInstanceManager) ResumeAgentInstance(instanceID string, deviceID string) error {
	instance, err := aim.loadInstanceFromDB(instanceID)
	if err != nil {
		return err
	}

	if instance.Status != "paused" {
		return fmt.Errorf("instance is not paused: %s", instance.Status)
	}

	// 1. 恢复Wrapper
	wrapper, err := aim.wrapperManager.ResumeWrapper(instance)
	if err != nil {
		return fmt.Errorf("failed to resume wrapper: %w", err)
	}

	instance.wrapper = wrapper
	instance.Status = "active"
	instance.CurrentDeviceID = deviceID
	instance.LastActivityAt = time.Now()

	// 2. 更新数据库和缓存
	if err := aim.updateInstanceInDB(instance); err != nil {
		return err
	}

	aim.mu.Lock()
	aim.activeInstances[instanceID] = instance
	aim.mu.Unlock()

	log.Printf("Resumed AgentInstance: %s on device %s", instanceID, deviceID)
	return nil
}

// EndAgentInstance 结束AgentInstance
func (aim *AgentInstanceManager) EndAgentInstance(instanceID string) error {
	aim.mu.Lock()
	instance, exists := aim.activeInstances[instanceID]
	if exists {
		delete(aim.activeInstances, instanceID)
	}
	aim.mu.Unlock()

	if !exists {
		// 从数据库加载
		var err error
		instance, err = aim.loadInstanceFromDB(instanceID)
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

	if err := aim.updateInstanceInDB(instance); err != nil {
		return fmt.Errorf("failed to update instance status: %w", err)
	}

	log.Printf("Ended AgentInstance: %s", instanceID)
	return nil
}

// 数据库操作方法

// saveInstanceToDB 保存实例到数据库
func (aim *AgentInstanceManager) saveInstanceToDB(instance *AgentInstance) error {
	sessionStateJSON, _ := json.Marshal(instance.SessionState)
	permissionStateJSON, _ := json.Marshal(instance.PermissionState)

	query := `
        INSERT INTO agent_instances (
            id, user_id, tool_name, status, name, owner_device_id, current_device_id,
            started_at, last_activity_at, session_state, git_diff, initial_git_hash,
            permission_state
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
    `

	_, err := aim.db.Exec(query,
		instance.ID, instance.UserID, instance.ToolName, instance.Status, instance.Name,
		instance.OwnerDeviceID, instance.CurrentDeviceID, instance.StartedAt,
		instance.LastActivityAt, sessionStateJSON, instance.GitDiff,
		instance.InitialGitHash, permissionStateJSON,
	)

	return err
}

// loadInstanceFromDB 从数据库加载实例
func (aim *AgentInstanceManager) loadInstanceFromDB(instanceID string) (*AgentInstance, error) {
	query := `
        SELECT id, user_id, tool_name, status, name, owner_device_id, current_device_id,
               started_at, ended_at, last_activity_at, session_state, git_diff,
               initial_git_hash, permission_state
        FROM agent_instances WHERE id = $1
    `

	row := aim.db.QueryRow(query, instanceID)
	return aim.scanAgentInstance(row)
}

// scanAgentInstance 扫描数据库行到AgentInstance结构
func (aim *AgentInstanceManager) scanAgentInstance(scanner interface{}) (*AgentInstance, error) {
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
func (aim *AgentInstanceManager) updateInstanceInDB(instance *AgentInstance) error {
	sessionStateJSON, _ := json.Marshal(instance.SessionState)
	permissionStateJSON, _ := json.Marshal(instance.PermissionState)

	query := `
        UPDATE agent_instances SET
            status = $2, name = $3, current_device_id = $4, ended_at = $5,
            last_activity_at = $6, session_state = $7, git_diff = $8,
            initial_git_hash = $9, permission_state = $10
        WHERE id = $1
    `

	_, err := aim.db.Exec(query,
		instance.ID, instance.Status, instance.Name, instance.CurrentDeviceID,
		instance.EndedAt, instance.LastActivityAt, sessionStateJSON,
		instance.GitDiff, instance.InitialGitHash, permissionStateJSON,
	)

	return err
}

// updateInstanceActivityInDB 更新实例活跃时间
func (aim *AgentInstanceManager) updateInstanceActivityInDB(instanceID string, activityTime time.Time) error {
	query := `UPDATE agent_instances SET last_activity_at = $2 WHERE id = $1`
	_, err := aim.db.Exec(query, instanceID, activityTime)
	return err
}

// deleteInstanceFromDB 从数据库删除实例
func (aim *AgentInstanceManager) deleteInstanceFromDB(instanceID string) error {
	query := `DELETE FROM agent_instances WHERE id = $1`
	_, err := aim.db.Exec(query, instanceID)
	return err
}

// saveInstanceState 保存实例状态
func (aim *AgentInstanceManager) saveInstanceState(instance *AgentInstance) error {
	// 从wrapper收集当前状态
	if instance.wrapper != nil {
		state, err := instance.wrapper.GetCurrentState()
		if err != nil {
			log.Printf("Failed to get wrapper state: %v", err)
		} else {
			// 合并状态
			for k, v := range state {
				instance.SessionState[k] = v
			}
		}
	}

	// 保存到数据库
	return aim.updateInstanceInDB(instance)
}

// startCleanupRoutine 启动清理协程
func (aim *AgentInstanceManager) startCleanupRoutine() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		aim.cleanupInactiveInstances()
	}
}

// cleanupInactiveInstances 清理不活跃的实例
func (aim *AgentInstanceManager) cleanupInactiveInstances() {
	// 查找超过24小时不活跃的实例
	inactiveThreshold := time.Now().Add(-24 * time.Hour)

	query := `
        SELECT id FROM agent_instances 
        WHERE status = 'active' 
        AND last_activity_at < $1
    `

	rows, err := aim.db.Query(query, inactiveThreshold)
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

	// 暂停不活跃的实例
	for _, instanceID := range inactiveIDs {
		if err := aim.PauseAgentInstance(instanceID); err != nil {
			log.Printf("Failed to pause inactive instance %s: %v", instanceID, err)
		} else {
			log.Printf("Paused inactive instance: %s", instanceID)
		}
	}

	if len(inactiveIDs) > 0 {
		log.Printf("Cleaned up %d inactive instances", len(inactiveIDs))
	}
}