# 跨设备终端会话系统实现设计文档

## 🎯 项目概述

**目标**: 实现一个基于tmux/screen的跨设备AI工具终端会话管理系统

**核心功能**:
- iOS通过API创建tmux/screen会话
- Mac通过原生终端命令直接恢复会话  
- 支持Claude、Gemini、Cursor等AI工具
- 基于Omnara的AgentInstance模式

## 📁 项目结构

```
anywhere/core/
├── cmd/
│   └── server/
│       └── main.go                    # 服务器入口
├── internal/
│   ├── config/
│   │   ├── config.go                  # 配置管理
│   │   └── config.yaml               # 配置文件
│   ├── database/
│   │   ├── connection.go             # 数据库连接
│   │   ├── migrations/               # 数据库迁移
│   │   │   ├── 001_init.sql
│   │   │   └── 002_terminal_sessions.sql
│   │   └── models.go                 # 数据模型
│   ├── session/
│   │   ├── manager.go                # 会话管理器
│   │   ├── tmux.go                   # tmux会话管理
│   │   ├── screen.go                 # screen会话管理
│   │   └── status.go                 # 状态检查
│   ├── api/
│   │   ├── handlers.go               # API处理器
│   │   ├── middleware.go             # 中间件
│   │   └── routes.go                 # 路由定义
│   └── tools/
│       ├── claude.go                 # Claude适配器
│       ├── gemini.go                 # Gemini适配器
│       └── cursor.go                 # Cursor适配器
├── pkg/
│   ├── types/
│   │   └── session.go                # 公共类型定义
│   └── utils/
│       └── helpers.go                # 工具函数
├── scripts/
│   ├── setup.sh                     # 环境设置脚本
│   └── deploy.sh                    # 部署脚本
├── docker/
│   ├── Dockerfile                   # Docker构建文件
│   └── docker-compose.yml          # Docker编排
├── docs/
│   ├── api.md                       # API文档
│   └── deployment.md               # 部署文档
├── go.mod
├── go.sum
├── Makefile                         # 构建脚本
└── README.md
```

## 🗄️ 数据库设计

### 表结构

```sql
-- Agent实例表 (继承Omnara)
CREATE TABLE agent_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL,
    tool_name VARCHAR(50) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    name VARCHAR(255),
    
    -- 设备信息
    owner_device_id VARCHAR(100) NOT NULL,
    current_device_id VARCHAR(100),
    
    -- 会话类型
    session_type VARCHAR(20),              -- pty, tmux, screen
    server_host VARCHAR(255),
    
    -- 时间戳
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 状态数据
    session_state JSONB DEFAULT '{}',
    permission_state JSONB DEFAULT '{}',
    git_diff TEXT,
    initial_git_hash VARCHAR(40)
);

-- 终端会话表
CREATE TABLE terminal_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_instance_id UUID NOT NULL REFERENCES agent_instances(id),
    
    -- 会话标识
    session_type VARCHAR(20) NOT NULL,     -- tmux, screen
    native_session_id VARCHAR(100) NOT NULL,
    
    -- 工具信息
    tool_name VARCHAR(50) NOT NULL,
    tool_command TEXT NOT NULL,
    working_directory TEXT,
    
    -- 服务器信息
    server_host VARCHAR(255) NOT NULL,
    server_port INTEGER DEFAULT 22,
    server_user VARCHAR(100),
    
    -- 恢复信息
    attach_command TEXT NOT NULL,
    ssh_command TEXT,
    
    -- 状态
    status VARCHAR(20) DEFAULT 'running',
    pid INTEGER,
    
    -- 时间戳
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_attached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP
);

-- 消息表 (继承Omnara)
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_instance_id UUID NOT NULL REFERENCES agent_instances(id),
    sender_type VARCHAR(10) NOT NULL,      -- USER, AGENT
    content TEXT NOT NULL,
    requires_user_input BOOLEAN DEFAULT FALSE,
    git_diff TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 🔧 核心组件设计

### 1. 配置管理 (config/config.go)

```go
type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
    Tools    ToolsConfig    `yaml:"tools"`
    Session  SessionConfig  `yaml:"session"`
}

type ServerConfig struct {
    Host       string `yaml:"host"`
    Port       int    `yaml:"port"`
    SSHPort    int    `yaml:"ssh_port"`
    SSHUser    string `yaml:"ssh_user"`
    WorkingDir string `yaml:"working_dir"`
}

type DatabaseConfig struct {
    URL            string `yaml:"url"`
    MaxConnections int    `yaml:"max_connections"`
}

type ToolsConfig struct {
    Claude ToolConfig `yaml:"claude"`
    Gemini ToolConfig `yaml:"gemini"`
    Cursor ToolConfig `yaml:"cursor"`
}

type ToolConfig struct {
    Command    string `yaml:"command"`
    WorkingDir string `yaml:"working_dir"`
}

type SessionConfig struct {
    CleanupInterval   string `yaml:"cleanup_interval"`
    InactiveTimeout   string `yaml:"inactive_timeout"`
    MaxSessionsPerUser int   `yaml:"max_sessions_per_user"`
}
```

### 2. 数据模型 (types/session.go)

```go
// TerminalSession 终端会话
type TerminalSession struct {
    ID               string    `json:"id" db:"id"`
    AgentInstanceID  string    `json:"agent_instance_id" db:"agent_instance_id"`
    SessionType      string    `json:"session_type" db:"session_type"`
    NativeSessionID  string    `json:"native_session_id" db:"native_session_id"`
    ToolName         string    `json:"tool_name" db:"tool_name"`
    ToolCommand      string    `json:"tool_command" db:"tool_command"`
    WorkingDirectory string    `json:"working_directory" db:"working_directory"`
    ServerHost       string    `json:"server_host" db:"server_host"`
    ServerPort       int       `json:"server_port" db:"server_port"`
    ServerUser       string    `json:"server_user" db:"server_user"`
    AttachCommand    string    `json:"attach_command" db:"attach_command"`
    SSHCommand       string    `json:"ssh_command" db:"ssh_command"`
    Status           string    `json:"status" db:"status"`
    PID              int       `json:"pid" db:"pid"`
    CreatedAt        time.Time `json:"created_at" db:"created_at"`
    LastAttachedAt   time.Time `json:"last_attached_at" db:"last_attached_at"`
    EndedAt          *time.Time `json:"ended_at" db:"ended_at"`
}

// AgentInstance Agent实例
type AgentInstance struct {
    ID              string                 `json:"id" db:"id"`
    UserID          string                 `json:"user_id" db:"user_id"`
    ToolName        string                 `json:"tool_name" db:"tool_name"`
    Status          string                 `json:"status" db:"status"`
    Name            string                 `json:"name" db:"name"`
    OwnerDeviceID   string                 `json:"owner_device_id" db:"owner_device_id"`
    CurrentDeviceID string                 `json:"current_device_id" db:"current_device_id"`
    SessionType     string                 `json:"session_type" db:"session_type"`
    ServerHost      string                 `json:"server_host" db:"server_host"`
    StartedAt       time.Time              `json:"started_at" db:"started_at"`
    EndedAt         *time.Time             `json:"ended_at" db:"ended_at"`
    LastActivityAt  time.Time              `json:"last_activity_at" db:"last_activity_at"`
    SessionState    map[string]interface{} `json:"session_state" db:"session_state"`
    PermissionState map[string]interface{} `json:"permission_state" db:"permission_state"`
    GitDiff         string                 `json:"git_diff" db:"git_diff"`
    InitialGitHash  string                 `json:"initial_git_hash" db:"initial_git_hash"`
}

// Message 消息
type Message struct {
    ID              string                 `json:"id" db:"id"`
    AgentInstanceID string                 `json:"agent_instance_id" db:"agent_instance_id"`
    SenderType      string                 `json:"sender_type" db:"sender_type"`
    Content         string                 `json:"content" db:"content"`
    RequiresInput   bool                   `json:"requires_user_input" db:"requires_user_input"`
    GitDiff         string                 `json:"git_diff" db:"git_diff"`
    Metadata        map[string]interface{} `json:"metadata" db:"metadata"`
    CreatedAt       time.Time              `json:"created_at" db:"created_at"`
}
```

### 3. 会话管理器接口 (session/manager.go)

```go
// SessionManager 会话管理器接口
type SessionManager interface {
    // 会话创建和管理
    CreateSession(req *CreateSessionRequest) (*TerminalSession, error)
    GetSession(sessionID string) (*TerminalSession, error)
    ListSessions(userID string) ([]*TerminalSession, error)
    DeleteSession(sessionID string) error
    
    // 会话状态
    CheckSessionStatus(sessionID string) (string, error)
    UpdateSessionStatus(sessionID string, status string) error
    
    // 会话清理
    CleanupInactiveSessions() error
    
    // 原生会话操作
    CreateNativeSession(sessionType, toolName, agentInstanceID string) (string, string, error)
    KillNativeSession(session *TerminalSession) error
}

// NativeSessionManager 原生会话管理器接口  
type NativeSessionManager interface {
    Create(toolName, agentInstanceID string) (nativeID, attachCommand string, err error)
    CheckStatus(nativeID string) string
    Kill(nativeID string) error
    List() ([]string, error)
}
```

### 4. API 请求/响应结构

```go
// 请求结构
type CreateSessionRequest struct {
    ToolName    string `json:"tool_name" binding:"required"`
    SessionType string `json:"session_type" binding:"required"`
    Name        string `json:"name"`
    DeviceID    string `json:"device_id" binding:"required"`
    UserID      string `json:"user_id" binding:"required"`
}

type AttachSessionRequest struct {
    SessionID string `json:"session_id" binding:"required"`
    DeviceID  string `json:"device_id" binding:"required"`
}

// 响应结构
type CreateSessionResponse struct {
    Session      *TerminalSession  `json:"session"`
    Instructions map[string]string `json:"instructions"`
}

type SessionListResponse struct {
    Sessions []*TerminalSession `json:"sessions"`
    Count    int                `json:"count"`
}

type SessionStatusResponse struct {
    SessionID string    `json:"session_id"`
    Status    string    `json:"status"`
    NativeID  string    `json:"native_id"`
    CheckedAt time.Time `json:"checked_at"`
}
```

## 🚀 API 接口设计

### 基础路径: `/api/v1/terminal`

```
POST   /sessions                # 创建会话
GET    /sessions                # 列出会话 (query: user_id)  
GET    /sessions/:id            # 获取会话详情
DELETE /sessions/:id            # 删除会话

POST   /sessions/:id/attach     # 记录连接 (更新last_attached_at)
POST   /sessions/:id/detach     # 记录分离
GET    /sessions/:id/status     # 检查状态

GET    /sessions/:id/ws         # WebSocket连接 (Web客户端备选)
```

### 示例请求/响应

```bash
# 创建会话
POST /api/v1/terminal/sessions
{
  "tool_name": "claude",
  "session_type": "tmux",
  "name": "coding-session",
  "device_id": "ios-device-123", 
  "user_id": "user-456"
}

# 响应
{
  "session": {
    "id": "session-abc123",
    "native_session_id": "ai-claude-abc12345",
    "attach_command": "tmux attach -t ai-claude-abc12345",
    "ssh_command": "ssh -t user@server 'tmux attach -t ai-claude-abc12345'",
    "status": "running"
  },
  "instructions": {
    "attach_command": "tmux attach -t ai-claude-abc12345",
    "ssh_command": "ssh -t user@server 'tmux attach -t ai-claude-abc12345'",
    "usage_example": "ssh -t user@server 'tmux attach -t ai-claude-abc12345'"
  }
}
```

## 🔨 实现步骤

### 阶段1: 基础设施 (1-2天)
1. ✅ 项目结构搭建
2. ✅ 配置管理实现  
3. ✅ 数据库连接和模型
4. ✅ 基础中间件和路由

### 阶段2: 核心功能 (2-3天)  
1. ✅ tmux会话管理器实现
2. ✅ screen会话管理器实现
3. ✅ 统一会话管理器接口
4. ✅ 会话状态检查机制

### 阶段3: API接口 (1-2天)
1. ✅ 会话CRUD接口
2. ✅ 状态检查接口  
3. ✅ 错误处理和验证
4. ✅ API文档

### 阶段4: 工具适配 (1天)
1. ✅ Claude工具适配器
2. ✅ Gemini工具适配器  
3. ✅ Cursor工具适配器
4. ✅ 通用工具接口

### 阶段5: 部署和测试 (1天)
1. ✅ Docker容器化
2. ✅ 部署脚本
3. ✅ 集成测试
4. ✅ 文档完善

## 🛠️ 开发环境要求

### 服务器环境
- Linux/macOS (支持tmux/screen)
- Go 1.21+
- PostgreSQL 13+
- tmux 3.0+ / screen 4.0+
- SSH服务

### 开发工具
- Go开发环境
- PostgreSQL客户端
- Docker (可选)
- 支持SSH的终端

## 📋 配置示例

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 8080
  ssh_port: 22
  ssh_user: "ai"
  working_dir: "/home/ai/workspace"

database:
  url: "postgresql://user:pass@localhost:5432/ai_terminal"
  max_connections: 10

tools:
  claude:
    command: "claude"
    working_dir: "/home/ai/workspace"
  gemini:
    command: "gemini"  
    working_dir: "/home/ai/workspace"
  cursor:
    command: "cursor"
    working_dir: "/home/ai/workspace"

session:
  cleanup_interval: "30m"
  inactive_timeout: "24h"
  max_sessions_per_user: 10
```

## 🧪 测试计划

### 单元测试
- [ ] 会话管理器核心逻辑
- [ ] tmux/screen操作
- [ ] 数据库操作
- [ ] API处理器

### 集成测试  
- [ ] 完整的会话创建流程
- [ ] 跨设备会话恢复
- [ ] 会话状态检查
- [ ] 错误处理场景

### 端到端测试
- [ ] iOS创建 + Mac恢复流程
- [ ] 多用户并发使用
- [ ] 长期会话稳定性
- [ ] 异常情况恢复

## 📝 部署指南

### 快速部署
```bash
# 1. 克隆项目
git clone <repo>
cd anywhere/core

# 2. 构建
make build

# 3. 配置
cp config.example.yaml config.yaml
# 编辑配置文件

# 4. 初始化数据库
make migrate-up

# 5. 启动服务
./bin/server
```

### Docker部署
```bash
# 1. 构建镜像
docker build -t ai-terminal .

# 2. 启动服务
docker-compose up -d
```

## 🔍 监控和日志

### 日志级别
- ERROR: 系统错误
- WARN: 警告信息  
- INFO: 关键操作记录
- DEBUG: 详细调试信息

### 监控指标
- 活跃会话数量
- 会话创建/删除速率
- API响应时间
- 数据库连接状态
- 系统资源使用

## 🚨 错误处理

### 常见错误场景
1. **会话创建失败**: tmux/screen命令执行失败
2. **工具启动失败**: AI工具不可用或配置错误
3. **会话不存在**: 恢复一个已经结束的会话
4. **权限问题**: SSH访问或目录权限
5. **资源限制**: 达到最大会话数限制

### 错误响应格式
```json
{
  "error": "session_not_found",
  "message": "Session abc123 not found or has ended",
  "code": 404,
  "timestamp": "2024-01-01T12:00:00Z"
}
```

这个设计文档提供了完整的实现指南，包含了项目结构、数据库设计、核心组件、API接口、实现步骤和部署指南。可以作为开发的详细参考。