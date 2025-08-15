# Anywhere AI CLI Manager

一个统一的AI CLI工具管理平台，支持Claude Code、Gemini CLI、Cursor等多种AI工具的跨设备终端会话管理。

## 🎯 核心特性

- **多工具支持**: 统一管理Claude、Gemini、Cursor、GitHub Copilot等AI CLI工具
- **跨设备恢复**: 在iOS创建会话，在Mac上恢复继续使用
- **tmux集成**: 利用tmux的原生终端管理能力
- **权限检测**: 智能检测并处理工具的权限请求
- **持久化存储**: SQLite轻量级数据库存储会话状态
- **实时监控**: 实时捕获和处理工具输出

## 📦 项目结构

```
anywhere/
├── core/                      # 核心功能模块
│   ├── tmux/                 # tmux会话管理
│   │   └── manager.go        # tmux管理器
│   ├── tools/                # AI工具管理
│   │   ├── session_manager.go # 工具会话管理
│   │   └── adapters.go       # 工具适配器
│   ├── output/               # 输出处理
│   │   └── processor.go      # 输出处理器和权限检测
│   ├── database/             # 数据持久化
│   │   └── sqlite.go         # SQLite存储层
│   └── core/                 # 核心接口定义
│       ├── interface.go      # 工具适配器接口
│       ├── types.go          # 类型定义
│       └── pty.go           # PTY管理
├── server/                   # 后端服务器模块
│   ├── cmd/                 # 服务入口
│   ├── internal/            # 内部实现
│   ├── pkg/                 # 公共包
│   └── configs/             # 配置文件
├── cli/                      # 命令行客户端
│   └── main.go              # CLI入口
├── pkg/sdk/                  # Go SDK
│   ├── client.go            # SDK客户端
│   └── models.go            # 数据模型
└── examples/                 # 使用示例
    └── basic_usage.go       # 基础使用示例
```

## 🚀 快速开始

### 安装依赖

```bash
# 安装tmux
brew install tmux  # macOS
apt-get install tmux  # Ubuntu

# 获取Go依赖
cd anywhere/core
go mod tidy
```

### 基础使用

```go
package main

import (
    "context"
    "github.com/majiayu000/anywhere-ai/core/tmux"
    "github.com/majiayu000/anywhere-ai/core/tools"
)

func main() {
    // 创建管理器
    tmuxManager := tmux.NewManager()
    sessionManager := tools.NewSessionManager(tmuxManager)
    
    // 创建Claude会话
    ctx := context.Background()
    session, err := sessionManager.CreateSession(ctx, tools.ToolClaude, "my-claude")
    
    // 发送命令
    sessionManager.SendInput(ctx, session.ID, "Hello Claude!")
    
    // 监控输出
    sessionManager.MonitorSession(ctx, session.ID, func(s *tools.ToolSession, output string) {
        fmt.Println("Output:", output)
    })
}
```

## 🔧 核心组件

### tmux管理器
- 创建、附加、分离tmux会话
- 发送命令和捕获输出
- 跨设备会话恢复

### 工具适配器
- Claude Adapter: Claude Code集成
- Gemini Adapter: Gemini CLI集成  
- Cursor Adapter: Cursor IDE CLI集成
- Copilot Adapter: GitHub Copilot CLI集成

### 输出处理器
- 实时输出缓冲和分析
- 权限请求智能检测
- 文件操作、命令执行、网络请求权限识别

### 数据持久化
- SQLite轻量级存储
- 会话状态持久化
- 跨设备会话发现

## 📱 跨设备使用

### 在iOS上创建会话
```go
// iOS设备上
session := createSession("claude", "work-session")
saveToDatabase(session)
```

### 在Mac上恢复会话
```go
// Mac设备上
sessions := listRemoteSessions()
session := findSession("work-session")
attachToSession(session)
```

## 🔐 权限处理

系统会自动检测AI工具的权限请求：

```go
processor := output.NewOutputProcessor()
processor.ProcessOutput(toolOutput)

if permission := processor.GetLastPermission(); permission != nil {
    switch permission.Type {
    case "file_write":
        // 处理文件写入权限
    case "command_execute":
        // 处理命令执行权限
    case "network":
        // 处理网络请求权限
    }
}
```

## 🎨 架构优势

- **混合架构**: 结合Omnara的消息系统和tmux的终端管理
- **模块化设计**: 各组件独立，易于扩展
- **统一接口**: 所有AI工具使用相同的管理接口
- **轻量级**: SQLite存储，无需复杂数据库配置

## 🤝 贡献

欢迎提交Issue和Pull Request！

## 📄 许可

MIT License