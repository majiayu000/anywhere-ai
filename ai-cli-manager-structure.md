# AI CLI Manager - 项目结构设计

## 📂 项目结构（全部在 anywhere 下）

```bash
anywhere/
├── core/                         # 🔥 核心模块 - CLI 交互引擎（独立的 Go 模块）
│   ├── go.mod                    # module github.com/majiayu000/ai-cli-core
│   ├── go.sum
│   ├── interface.go              # 核心接口定义
│   ├── process.go                # 进程管理器
│   ├── pty.go                    # PTY 伪终端管理
│   ├── stream.go                 # 流处理器
│   ├── parser.go                 # 输出解析器
│   ├── session.go                # 会话管理
│   ├── types.go                  # 通用类型定义
│   └── adapters/                 # 工具适配器
│       ├── adapter.go            # 适配器接口
│       ├── claude/               # Claude Code 适配器
│       │   ├── claude.go
│       │   ├── parser.go
│       │   └── commands.go
│       ├── gemini/               # Gemini CLI 适配器
│       │   ├── gemini.go
│       │   └── parser.go
│       ├── cursor/               # Cursor 适配器
│       │   ├── cursor.go
│       │   └── parser.go
│       └── copilot/              # GitHub Copilot 适配器
│           ├── copilot.go
│           └── parser.go
│
└── go-web-starter/               # Web 服务层（基于现有 Gin 框架）
    ├── go.mod                    # 添加对 core 的依赖
    ├── internal/
    │   ├── handlers/
    │   │   ├── ai_tools.go      # AI 工具管理 API
    │   │   ├── ai_sessions.go   # 会话管理 API
    │   │   └── ai_websocket.go  # WebSocket 处理
    │   ├── services/
    │   │   └── ai_service.go    # AI 服务层（调用 core）
    │   └── router/
    │       └── router.go         # 添加 AI 相关路由
    └── configs/
        └── ai.yaml               # AI 工具配置
```

## 🚀 实施步骤

### Step 1: 创建 Core 模块

```bash
cd anywhere
mkdir -p core/adapters/{claude,gemini,cursor,copilot}

# 初始化 core 模块
cd core
go mod init github.com/majiayu000/ai-cli-core

# 添加必要的依赖
go get github.com/creack/pty
go get github.com/google/uuid
go get golang.org/x/term
```

### Step 2: 在 go-web-starter 中使用 Core

```bash
cd ../go-web-starter

# 添加本地 core 模块依赖
go mod edit -replace github.com/majiayu000/ai-cli-core=../core
go get github.com/majiayu000/ai-cli-core
```

## 💻 Core 模块实现

### 1. 核心接口 (core/interface.go)

```go
package core

import (
    "context"
    "io"
    "time"
)

// ToolAdapter - AI 工具适配器接口
type ToolAdapter interface {
    // 基本信息
    GetName() string
    GetVersion() string
    GetDescription() string
    GetIcon() string
    
    // 检查与配置
    IsInstalled() bool
    GetExecutablePath() string
    GetDefaultArgs() []string
    ValidateConfig() error
    
    // 命令构建
    BuildCommand(args []string) *Command
    
    // 输出处理
    ParseOutput(data []byte) (*ParsedOutput, error)
    IsPromptReady(output string) bool
    IsWaitingForInput(output string) bool
    DetectError(output string) *ToolError
    
    // 输入处理
    TransformInput(input string) string
    HandleSpecialCommand(cmd string) (handled bool, response string)
}

// Session - 会话接口
type Session interface {
    GetID() string
    GetTool() ToolAdapter
    GetStartTime() time.Time
    GetStatus() SessionStatus
    
    // 生命周期
    Start(ctx context.Context) error
    Stop() error
    Restart() error
    
    // IO 操作
    SendInput(input string) error
    GetOutputStream() <-chan *OutputMessage
    
    // 状态
    IsRunning() bool
    GetStats() *SessionStats
}

// ProcessManager - 进程管理接口
type ProcessManager interface {
    Start(ctx context.Context, cmd *Command) error
    Stop() error
    
    GetPID() int
    IsRunning() bool
    
    Write(data []byte) (int, error)
    Read() <-chan []byte
}
```

### 2. PTY 管理器 (core/pty.go)

```go
package core

import (
    "context"
    "os"
    "os/exec"
    "syscall"
    
    "github.com/creack/pty"
    "golang.org/x/term"
)

type PTYManager struct {
    cmd      *exec.Cmd
    pty      *os.File
    ctx      context.Context
    cancel   context.CancelFunc
    
    readChan chan []byte
    doneChan chan struct{}
}

func NewPTYManager() *PTYManager {
    return &PTYManager{
        readChan: make(chan []byte, 100),
        doneChan: make(chan struct{}),
    }
}

func (p *PTYManager) Start(ctx context.Context, command *Command) error {
    p.ctx, p.cancel = context.WithCancel(ctx)
    
    // 创建命令
    p.cmd = exec.CommandContext(p.ctx, command.Path, command.Args...)
    p.cmd.Env = append(os.Environ(), command.Env...)
    
    // 启动 PTY
    ptmx, err := pty.Start(p.cmd)
    if err != nil {
        return err
    }
    p.pty = ptmx
    
    // 设置终端大小
    if err := p.setSize(); err != nil {
        return err
    }
    
    // 启动读取协程
    go p.readLoop()
    
    // 监听进程退出
    go p.waitForExit()
    
    return nil
}

func (p *PTYManager) setSize() error {
    // 获取当前终端大小
    ws, err := pty.GetsizeFull(os.Stdin)
    if err != nil {
        // 使用默认大小
        ws = &pty.Winsize{
            Rows: 40,
            Cols: 120,
            X:    0,
            Y:    0,
        }
    }
    
    return pty.Setsize(p.pty, ws)
}

func (p *PTYManager) readLoop() {
    defer close(p.readChan)
    
    buf := make([]byte, 4096)
    for {
        n, err := p.pty.Read(buf)
        if err != nil {
            return
        }
        
        if n > 0 {
            data := make([]byte, n)
            copy(data, buf[:n])
            
            select {
            case p.readChan <- data:
            case <-p.ctx.Done():
                return
            }
        }
    }
}

func (p *PTYManager) Write(data []byte) (int, error) {
    return p.pty.Write(data)
}

func (p *PTYManager) Read() <-chan []byte {
    return p.readChan
}
```

### 3. Claude 适配器实现 (core/adapters/claude/claude.go)

```go
package claude

import (
    "fmt"
    "os/exec"
    "regexp"
    "strings"
    
    "github.com/majiayu000/ai-cli-core"
)

type ClaudeAdapter struct {
    execPath string
    
    // 输出模式匹配
    promptPattern       *regexp.Regexp
    waitingPattern      *regexp.Regexp
    errorPattern        *regexp.Regexp
    costPattern         *regexp.Regexp
    toolUsePattern      *regexp.Regexp
}

func NewClaudeAdapter() *ClaudeAdapter {
    return &ClaudeAdapter{
        execPath: "claude",
        
        promptPattern:  regexp.MustCompile(`^[▶►>]\s*$`),
        waitingPattern: regexp.MustCompile(`(?i)(waiting for|awaiting|need your|would you like)`),
        errorPattern:   regexp.MustCompile(`(?i)(error|failed|exception|invalid)`),
        costPattern:    regexp.MustCompile(`Cost:\s*\$?([0-9.]+)`),
        toolUsePattern: regexp.MustCompile(`Using tool:\s*(\w+)`),
    }
}

func (c *ClaudeAdapter) GetName() string {
    return "Claude Code"
}

func (c *ClaudeAdapter) GetDescription() string {
    return "Anthropic's Claude AI assistant for coding"
}

func (c *ClaudeAdapter) GetIcon() string {
    return "🤖"
}

func (c *ClaudeAdapter) IsInstalled() bool {
    _, err := exec.LookPath(c.execPath)
    return err == nil
}

func (c *ClaudeAdapter) GetExecutablePath() string {
    path, _ := exec.LookPath(c.execPath)
    return path
}

func (c *ClaudeAdapter) GetDefaultArgs() []string {
    return []string{
        "--no-color",     // 禁用颜色输出
        "--no-markdown",  // 禁用 markdown 渲染
    }
}

func (c *ClaudeAdapter) BuildCommand(args []string) *core.Command {
    finalArgs := c.GetDefaultArgs()
    finalArgs = append(finalArgs, args...)
    
    return &core.Command{
        Path: c.execPath,
        Args: finalArgs,
        Env: []string{
            "TERM=xterm-256color",
            "CLAUDE_NO_INTERACTIVE=1",
        },
    }
}

func (c *ClaudeAdapter) ParseOutput(data []byte) (*core.ParsedOutput, error) {
    content := string(data)
    
    output := &core.ParsedOutput{
        Raw:       content,
        Type:      core.OutputTypeStandard,
        Timestamp: time.Now(),
        Metadata:  make(map[string]interface{}),
    }
    
    // 去除 ANSI 转义序列
    cleaned := stripANSI(content)
    output.Content = cleaned
    
    // 检测输出类型
    switch {
    case c.promptPattern.MatchString(cleaned):
        output.Type = core.OutputTypePrompt
        
    case c.waitingPattern.MatchString(cleaned):
        output.Type = core.OutputTypeWaitingInput
        output.Metadata["requires_input"] = true
        
    case c.errorPattern.MatchString(cleaned):
        output.Type = core.OutputTypeError
    }
    
    // 提取成本信息
    if matches := c.costPattern.FindStringSubmatch(cleaned); len(matches) > 1 {
        output.Metadata["cost"] = matches[1]
    }
    
    // 提取工具使用
    if matches := c.toolUsePattern.FindAllStringSubmatch(cleaned, -1); len(matches) > 0 {
        tools := []string{}
        for _, match := range matches {
            if len(match) > 1 {
                tools = append(tools, match[1])
            }
        }
        output.Metadata["tools_used"] = tools
    }
    
    return output, nil
}

func (c *ClaudeAdapter) IsPromptReady(output string) bool {
    return c.promptPattern.MatchString(output)
}

func (c *ClaudeAdapter) IsWaitingForInput(output string) bool {
    return c.waitingPattern.MatchString(output)
}

func (c *ClaudeAdapter) TransformInput(input string) string {
    // Claude 特殊命令处理
    if strings.HasPrefix(input, "/") {
        return input // Claude 命令直接传递
    }
    
    // 普通输入
    return input
}

func (c *ClaudeAdapter) HandleSpecialCommand(cmd string) (bool, string) {
    switch cmd {
    case "exit", "quit", "q":
        return true, "/exit"
    case "clear", "cls":
        return true, "/clear"
    case "help", "?":
        return true, "/help"
    case "cost":
        return true, "/cost"
    default:
        return false, ""
    }
}

func stripANSI(str string) string {
    ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
    return ansiRegex.ReplaceAllString(str, "")
}
```

### 4. 在 go-web-starter 中集成

```go
// go-web-starter/internal/services/ai_service.go
package services

import (
    "context"
    "fmt"
    
    core "github.com/majiayu000/ai-cli-core"
    "github.com/majiayu000/ai-cli-core/adapters/claude"
    "github.com/majiayu000/ai-cli-core/adapters/gemini"
)

type AIService struct {
    adapters map[string]core.ToolAdapter
    sessions map[string]core.Session
}

func NewAIService() *AIService {
    svc := &AIService{
        adapters: make(map[string]core.ToolAdapter),
        sessions: make(map[string]core.Session),
    }
    
    // 注册适配器
    svc.RegisterAdapter(claude.NewClaudeAdapter())
    svc.RegisterAdapter(gemini.NewGeminiAdapter())
    // ... 更多适配器
    
    return svc
}

func (s *AIService) RegisterAdapter(adapter core.ToolAdapter) {
    s.adapters[adapter.GetName()] = adapter
}

func (s *AIService) ListTools() []ToolInfo {
    tools := []ToolInfo{}
    
    for _, adapter := range s.adapters {
        tools = append(tools, ToolInfo{
            Name:        adapter.GetName(),
            Description: adapter.GetDescription(),
            Icon:        adapter.GetIcon(),
            Installed:   adapter.IsInstalled(),
        })
    }
    
    return tools
}

func (s *AIService) CreateSession(toolName string, userID string) (string, error) {
    adapter, ok := s.adapters[toolName]
    if !ok {
        return "", fmt.Errorf("tool %s not found", toolName)
    }
    
    if !adapter.IsInstalled() {
        return "", fmt.Errorf("tool %s is not installed", toolName)
    }
    
    // 创建会话
    session := core.NewSession(adapter)
    
    // 启动会话
    ctx := context.Background()
    if err := session.Start(ctx); err != nil {
        return "", err
    }
    
    // 保存会话
    sessionID := session.GetID()
    s.sessions[sessionID] = session
    
    return sessionID, nil
}

// go-web-starter/internal/handlers/ai_websocket.go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func HandleAIWebSocket(aiService *services.AIService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 升级到 WebSocket
        conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
        if err != nil {
            return
        }
        defer conn.Close()
        
        // 获取参数
        toolName := c.Query("tool")
        userID := c.GetString("user_id")
        
        // 创建会话
        sessionID, err := aiService.CreateSession(toolName, userID)
        if err != nil {
            conn.WriteJSON(gin.H{"error": err.Error()})
            return
        }
        
        session := aiService.GetSession(sessionID)
        
        // 处理输入
        go func() {
            for {
                var msg WebSocketMessage
                if err := conn.ReadJSON(&msg); err != nil {
                    break
                }
                
                if msg.Type == "input" {
                    session.SendInput(msg.Content)
                }
            }
        }()
        
        // 处理输出
        outputChan := session.GetOutputStream()
        for output := range outputChan {
            conn.WriteJSON(gin.H{
                "type":    output.Type,
                "content": output.Content,
                "time":    output.Timestamp,
            })
        }
    }
}

// go-web-starter/internal/router/router.go
// 在现有的 SetupRouter 函数中添加：

// AI CLI 管理路由
aiService := services.NewAIService()

ai := api.Group("/ai")
{
    ai.GET("/tools", handlers.ListAITools(aiService))
    ai.POST("/sessions", handlers.CreateAISession(aiService))
    ai.GET("/sessions/:id", handlers.GetAISession(aiService))
    ai.DELETE("/sessions/:id", handlers.StopAISession(aiService))
}

// WebSocket
r.GET("/ws/ai", handlers.HandleAIWebSocket(aiService))
```

## 🎯 开发顺序

### 今天：Day 1
1. 创建 `anywhere/core` 目录结构
2. 实现 `interface.go` 和 `types.go`
3. 实现 `pty.go` 基础功能

### 明天：Day 2
1. 实现 Claude 适配器
2. 测试进程启动和基本 IO
3. 实现 session 管理

### Day 3
1. 在 go-web-starter 中集成
2. 添加 WebSocket 支持
3. 测试端到端流程

## 📝 测试命令

```bash
# 测试 core 模块
cd anywhere/core
go test ./...

# 测试集成
cd ../go-web-starter
go run cmd/main.go

# 测试 WebSocket
wscat -c ws://localhost:8080/ws/ai?tool=Claude%20Code
```

这样所有代码都在 `anywhere` 文件夹下，`core` 作为独立模块，`go-web-starter` 调用它。