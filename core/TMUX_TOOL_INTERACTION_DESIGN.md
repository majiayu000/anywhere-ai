# tmux与AI工具交互设计文档
## 基于消息驱动的tmux会话管理

## 🎯 设计理念

**核心模式：**
- ✅ **SDK模式** - 提供Go SDK，不提供HTTP API
- ✅ **Wrapper模式** - 包装AI工具，监控输入输出
- ✅ **消息驱动** - 通过消息系统与后端通信
- ✅ **会话持久化** - AgentInstance + Messages模式

**技术创新：**
- 🔄 **tmux集成** - 用tmux替代PTY，支持跨设备恢复
- 🔄 **统一接口** - 统一的工具适配器接口

## 📋 核心组件

```
┌─────────────────────────────────────────────────────────────┐
│                    AI Tool Process                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │Claude Code  │  │ Gemini CLI  │  │   Cursor    │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ stdin/stdout
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    tmux Session                             │
│  ┌─────────────────────────────────────────────────────────┤
│  │ ai-claude-abc123 │ ai-gemini-def456 │ ai-cursor-ghi789  │
│  └─────────────────────────────────────────────────────────┤
└─────────────────────────────────────────────────────────────┘
                              │
                              │ tmux capture/send-keys
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Tool Wrapper                              │
│  ┌─────────────────────────────────────────────────────────┤
│  │ - tmux监控和控制                                          │
│  │ - 输出解析和过滤                                          │
│  │ - 权限提示检测                                            │
│  │ - 消息处理                                              │
│  └─────────────────────────────────────────────────────────┤
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Go SDK
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                 Omnara Backend                              │
│  ┌─────────────────────────────────────────────────────────┤
│  │ - AgentInstance管理                                     │
│  │ - Messages存储                                          │
│  │ - 跨设备状态同步                                         │
│  └─────────────────────────────────────────────────────────┤
└─────────────────────────────────────────────────────────────┘
```

## 🔧 Go SDK 设计

### 1. 核心SDK接口 (参考Omnara Python SDK)

```go
// Package sdk provides Go SDK for Omnara backend integration
package sdk

import (
    "context"
    "time"
)

// AnywhereClient anywhere后端客户端
type AnywhereClient struct {
    apiKey   string
    baseURL  string
    timeout  time.Duration
    httpClient *http.Client
}

// NewAnywhereClient 创建客户端
func NewAnywhereClient(apiKey, baseURL string) *AnywhereClient {
    return &AnywhereClient{
        apiKey:  apiKey,
        baseURL: baseURL,
        timeout: 30 * time.Second,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// SendMessage 发送消息
func (c *AnywhereClient) SendMessage(ctx context.Context, req *SendMessageRequest) (*SendMessageResponse, error) {
    // 实现与anywhere后端的消息通信
}

// GetPendingMessages 获取待处理消息
func (c *AnywhereClient) GetPendingMessages(ctx context.Context, agentInstanceID string, lastReadMessageID string) (*PendingMessagesResponse, error) {
    // 实现消息轮询
}

// RequestUserInput 请求用户输入
func (c *AnywhereClient) RequestUserInput(ctx context.Context, messageID string, timeoutMinutes int) ([]string, error) {
    // 实现用户输入请求
}

// EndSession 结束会话
func (c *AnywhereClient) EndSession(ctx context.Context, agentInstanceID string) (*EndSessionResponse, error) {
    // 实现会话结束
}
```

### 2. SDK数据模型

```go
// SendMessageRequest 发送消息请求 (对应Omnara的请求格式)
type SendMessageRequest struct {
    Content          string                 `json:"content"`
    AgentType        string                 `json:"agent_type,omitempty"`
    AgentInstanceID  string                 `json:"agent_instance_id,omitempty"`
    RequiresUserInput bool                  `json:"requires_user_input"`
    TimeoutMinutes   int                    `json:"timeout_minutes,omitempty"`
    PollInterval     float64                `json:"poll_interval,omitempty"`
    SendPush         *bool                  `json:"send_push,omitempty"`
    SendEmail        *bool                  `json:"send_email,omitempty"`
    SendSMS          *bool                  `json:"send_sms,omitempty"`
    GitDiff          string                 `json:"git_diff,omitempty"`
}

// SendMessageResponse 发送消息响应
type SendMessageResponse struct {
    Success             bool     `json:"success"`
    AgentInstanceID     string   `json:"agent_instance_id"`
    MessageID           string   `json:"message_id"`
    QueuedUserMessages  []string `json:"queued_user_messages"`
}

// PendingMessagesResponse 待处理消息响应
type PendingMessagesResponse struct {
    AgentInstanceID string    `json:"agent_instance_id"`
    Messages        []Message `json:"messages"`
    Status          string    `json:"status"`
}

// Message 消息模型
type Message struct {
    ID              string                 `json:"id"`
    Content         string                 `json:"content"`
    SenderType      string                 `json:"sender_type"`
    RequiresInput   bool                   `json:"requires_user_input"`
    GitDiff         string                 `json:"git_diff"`
    Metadata        map[string]interface{} `json:"metadata"`
    CreatedAt       time.Time              `json:"created_at"`
}

// EndSessionResponse 结束会话响应
type EndSessionResponse struct {
    Success         bool   `json:"success"`
    AgentInstanceID string `json:"agent_instance_id"`
    FinalStatus     string `json:"final_status"`
}
```

## 🛠️ Tool Wrapper 设计

### 1. 统一工具包装器接口

```go
// ToolSession 工具会话接口
type ToolSession interface {
    // 生命周期管理
    Start(ctx context.Context) error
    Stop() error
    IsRunning() bool
    
    // tmux会话管理
    GetTmuxSessionID() string
    AttachToSession() error
    DetachFromSession() error
    
    // 输入输出
    SendInput(input string) error
    ReadOutput() (string, error)
    GetTerminalBuffer() string
    
    // 状态监控
    IsIdle() bool
    DetectPermissionPrompt() *PermissionPrompt
    
    // 后端集成
    SendMessage(content string, requiresInput bool) error
    ProcessUserInput(input string) error
    HandlePermissionPrompt(prompt *PermissionPrompt) error
}

// TmuxToolSession tmux工具会话实现
type TmuxToolSession struct {
    // 基本信息
    toolName        string
    agentInstanceID string
    tmuxSessionID   string
    
    // tmux管理
    tmuxManager     *TmuxManager
    
    // 输出处理 (学习Omnara)
    terminalBuffer  string
    outputProcessor *OutputProcessor
    
    // 权限处理 (学习Omnara)
    permissionDetector *PermissionDetector
    lastToolUse        bool
    
    // 后端集成
    anywhereClient  *AnywhereClient
    messageProcessor *MessageProcessor
    
    // 状态
    running         bool
    lastActivity    time.Time
}
```

### 2. tmux管理器

```go
// TmuxManager tmux会话管理器
type TmuxManager struct {
    workingDir string
    serverHost string
}

// CreateSession 创建tmux会话
func (tm *TmuxManager) CreateSession(toolName, agentInstanceID string) (string, error) {
    sessionID := fmt.Sprintf("ai-%s-%s", toolName, agentInstanceID[:8])
    
    // 创建tmux会话
    cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionID)
    cmd.Dir = tm.workingDir
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("failed to create tmux session: %w", err)
    }
    
    return sessionID, nil
}

// StartTool 在tmux会话中启动AI工具
func (tm *TmuxManager) StartTool(sessionID, toolName, agentInstanceID string) error {
    // 构建工具命令
    toolCmd := tm.buildToolCommand(toolName, agentInstanceID)
    
    // 在tmux会话中启动工具
    cmd := exec.Command("tmux", "send-keys", "-t", sessionID, 
        strings.Join(toolCmd, " "), "Enter")
    return cmd.Run()
}

// SendKeys 向tmux会话发送按键
func (tm *TmuxManager) SendKeys(sessionID, keys string) error {
    cmd := exec.Command("tmux", "send-keys", "-t", sessionID, keys)
    return cmd.Run()
}

// CapturePane 捕获tmux窗格内容
func (tm *TmuxManager) CapturePane(sessionID string) (string, error) {
    cmd := exec.Command("tmux", "capture-pane", "-t", sessionID, "-p")
    output, err := cmd.Output()
    if err != nil {
        return "", err
    }
    return string(output), nil
}

// HasSession 检查tmux会话是否存在
func (tm *TmuxManager) HasSession(sessionID string) bool {
    cmd := exec.Command("tmux", "has-session", "-t", sessionID)
    return cmd.Run() == nil
}

// KillSession 终止tmux会话
func (tm *TmuxManager) KillSession(sessionID string) error {
    cmd := exec.Command("tmux", "kill-session", "-t", sessionID)
    return cmd.Run()
}

// buildToolCommand 构建工具启动命令
func (tm *TmuxManager) buildToolCommand(toolName, agentInstanceID string) []string {
    switch toolName {
    case "claude":
        return []string{
            "claude",
            "--session-id", agentInstanceID,
        }
    case "gemini":
        return []string{
            "gemini",
            "--session", agentInstanceID,
        }
    case "cursor":
        return []string{
            "cursor",
            "--session", agentInstanceID,
        }
    default:
        return []string{toolName}
    }
}
```

### 3. 输出处理器 (学习Omnara)

```go
// OutputProcessor 输出处理器 (对应Omnara的输出处理逻辑)
type OutputProcessor struct {
    buffer          string
    lastProcessTime time.Time
}

// ProcessOutput 处理工具输出 (学习Omnara的输出处理)
func (op *OutputProcessor) ProcessOutput(output string) (*ProcessedOutput, error) {
    op.buffer += output
    op.lastProcessTime = time.Now()
    
    // 清理ANSI转义码
    cleanOutput := op.cleanANSIEscapes(output)
    
    // 检测特殊状态
    result := &ProcessedOutput{
        CleanContent: cleanOutput,
        HasPermissionPrompt: op.detectPermissionPrompt(cleanOutput),
        IsIdle: op.detectIdleState(cleanOutput),
        IsToolUse: op.detectToolUse(cleanOutput),
    }
    
    return result, nil
}

// cleanANSIEscapes 清理ANSI转义码 (学习Omnara)
func (op *OutputProcessor) cleanANSIEscapes(text string) string {
    // 移除ANSI转义码
    ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
    return ansiRegex.ReplaceAllString(text, "")
}

// detectPermissionPrompt 检测权限提示 (学习Omnara的_extract_permission_prompt)
func (op *OutputProcessor) detectPermissionPrompt(text string) bool {
    // 检测Claude Code的权限提示模式
    patterns := []string{
        "Would you like to proceed",
        "Continue?",
        "[y/n]",
        "Allow this action?",
    }
    
    for _, pattern := range patterns {
        if strings.Contains(text, pattern) {
            return true
        }
    }
    return false
}

// detectIdleState 检测空闲状态 (学习Omnara的is_claude_idle)
func (op *OutputProcessor) detectIdleState(text string) bool {
    // 检测工具是否处于等待状态
    idlePatterns := []string{
        "waiting for input",
        "press enter to continue",
        "ready for next command",
    }
    
    for _, pattern := range idlePatterns {
        if strings.Contains(strings.ToLower(text), pattern) {
            return true
        }
    }
    
    // 如果最近没有输出，也认为是空闲
    return time.Since(op.lastProcessTime) > 5*time.Second
}

// detectToolUse 检测工具使用 (学习Omnara的工具使用检测)
func (op *OutputProcessor) detectToolUse(text string) bool {
    // 检测是否在使用工具
    toolPatterns := []string{
        "Using tool:",
        "Executing:",
        "Running command:",
    }
    
    for _, pattern := range patterns {
        if strings.Contains(text, pattern) {
            return true
        }
    }
    return false
}

// ProcessedOutput 处理后的输出
type ProcessedOutput struct {
    CleanContent        string
    HasPermissionPrompt bool
    IsIdle              bool
    IsToolUse           bool
}
```

### 4. 权限处理器 (学习Omnara)

```go
// PermissionDetector 权限检测器 (学习Omnara的权限处理)
type PermissionDetector struct {
    lastPromptTime time.Time
    promptHandled  bool
}

// DetectPermissionPrompt 检测权限提示 (学习Omnara的_extract_permission_prompt)
func (pd *PermissionDetector) DetectPermissionPrompt(text string) *PermissionPrompt {
    // 检查是否是计划模式
    isPlanMode := strings.Contains(text, "Would you like to proceed") &&
        (strings.Contains(text, "auto-accept edits") || 
         strings.Contains(text, "manually approve edits"))
    
    if isPlanMode {
        // 提取计划内容
        question := "Would you like to proceed with this plan?"
        planContent := pd.extractPlanContent(text)
        if planContent != "" {
            question = fmt.Sprintf("%s\n\n%s", question, planContent)
        }
        
        options := []string{"auto-accept edits", "manually approve edits", "cancel"}
        optionsMap := map[string]string{
            "a": "auto-accept edits",
            "m": "manually approve edits", 
            "c": "cancel",
        }
        
        return &PermissionPrompt{
            Question:   question,
            Options:    options,
            OptionsMap: optionsMap,
            Type:       "plan",
        }
    }
    
    // 检查普通权限提示
    if pd.containsPermissionKeywords(text) {
        return &PermissionPrompt{
            Question:   pd.extractQuestion(text),
            Options:    []string{"yes", "no"},
            OptionsMap: map[string]string{"y": "yes", "n": "no"},
            Type:       "permission",
        }
    }
    
    return nil
}

// extractPlanContent 提取计划内容 (学习Omnara)
func (pd *PermissionDetector) extractPlanContent(text string) string {
    planMarker := "Ready to code?"
    planStart := strings.LastIndex(text, planMarker)
    
    if planStart != -1 {
        planEnd := strings.Index(text[planStart:], "Would you like to proceed")
        if planEnd != -1 {
            planContent := text[planStart+len(planMarker) : planStart+planEnd]
            
            // 清理计划内容
            lines := []string{}
            for _, line := range strings.Split(planContent, "\n") {
                // 移除边框字符
                cleaned := regexp.MustCompile(`^[│\s]+`).ReplaceAllString(line, "")
                cleaned = regexp.MustCompile(`[│\s]+$`).ReplaceAllString(cleaned, "")
                cleaned = strings.TrimSpace(cleaned)
                
                // 跳过空行和边框
                if cleaned != "" && !regexp.MustCompile(`^[╭─╮╰╯]+$`).MatchString(cleaned) {
                    lines = append(lines, cleaned)
                }
            }
            
            return strings.Join(lines, "\n")
        }
    }
    
    return ""
}

// PermissionPrompt 权限提示
type PermissionPrompt struct {
    Question   string            `json:"question"`
    Options    []string          `json:"options"`
    OptionsMap map[string]string `json:"options_map"`
    Type       string            `json:"type"` // plan, permission
}
```

### 5. 消息处理器 (学习Omnara)

```go
// MessageProcessor 消息处理器
type MessageProcessor struct {
    anywhereClient    *AnywhereClient
    agentInstanceID   string
    lastMessageID     string
    lastMessageTime   time.Time
    webUIMessages     map[string]bool
    pendingInputID    string
    lastWasToolUse    bool
}

// ProcessUserMessage 处理用户消息 (学习Omnara的process_user_message_sync)
func (mp *MessageProcessor) ProcessUserMessage(content string, fromWeb bool) error {
    if fromWeb {
        // 来自Web UI的消息 - 标记以避免重复发送
        mp.webUIMessages[content] = true
    } else {
        // 来自CLI的消息 - 如果不是来自Web，则发送到Omnara
        if !mp.webUIMessages[content] {
            log.Printf("[INFO] Sending CLI message to Omnara: %s", content[:50])
            
            if mp.agentInstanceID != "" && mp.anywhereClient != nil {
                ctx := context.Background()
                req := &SendMessageRequest{
                    AgentInstanceID: mp.agentInstanceID,
                    Content:         content,
                    RequiresUserInput: false,
                }
                
                _, err := mp.aiClient.SendMessage(ctx, req)
                if err != nil {
                    return fmt.Errorf("failed to send user message: %w", err)
                }
            }
        } else {
            // 从跟踪集合中移除
            delete(mp.webUIMessages, content)
        }
    }
    
    // 重置空闲计时器并清除待处理输入
    mp.lastMessageTime = time.Now()
    mp.pendingInputID = ""
    
    return nil
}

// ProcessAssistantMessage 处理助手消息 (学习Omnara的process_assistant_message_sync)
func (mp *MessageProcessor) ProcessAssistantMessage(content string, toolsUsed []string) error {
    if mp.agentInstanceID == "" || mp.aiClient == nil {
        return nil
    }
    
    // 跟踪是否使用了工具
    mp.lastWasToolUse = len(toolsUsed) > 0
    
    // 清理内容 - 移除NUL字符和控制字符
    sanitizedContent := mp.sanitizeContent(content)
    
    // 获取git diff (如果需要)
    gitDiff := mp.getGitDiff()
    
    // 发送消息到Omnara
    ctx := context.Background()
    req := &SendMessageRequest{
        Content:         sanitizedContent,
        AgentInstanceID: mp.agentInstanceID,
        RequiresUserInput: false,
        GitDiff:         gitDiff,
    }
    
    response, err := mp.aiClient.SendMessage(ctx, req)
    if err != nil {
        return fmt.Errorf("failed to send assistant message: %w", err)
    }
    
    mp.lastMessageID = response.MessageID
    mp.lastMessageTime = time.Now()
    
    return nil
}

// RequestUserInput 请求用户输入 (学习Omnara的异步用户输入请求)
func (mp *MessageProcessor) RequestUserInput(messageID string) ([]string, error) {
    if mp.aiClient == nil {
        return nil, fmt.Errorf("ai client not available")
    }
    
    ctx := context.Background()
    
    // 使用长轮询请求用户输入
    responses, err := mp.aiClient.RequestUserInput(ctx, messageID, 1440) // 24小时超时
    if err != nil {
        return nil, fmt.Errorf("failed to request user input: %w", err)
    }
    
    // 处理响应
    for _, response := range responses {
        log.Printf("[INFO] Got user response from web UI: %s", response[:50])
        mp.ProcessUserMessage(response, true)
    }
    
    return responses, nil
}

// sanitizeContent 清理内容 (学习Omnara的内容清理)
func (mp *MessageProcessor) sanitizeContent(content string) string {
    // 移除NUL字符和不可打印字符
    result := ""
    for _, char := range content {
        if int(char) >= 32 || char == '\n' || char == '\r' || char == '\t' {
            result += string(char)
        }
    }
    return strings.ReplaceAll(result, "\x00", "")
}

// getGitDiff 获取git差异 (如果需要)
func (mp *MessageProcessor) getGitDiff() string {
    // 实现git diff获取逻辑
    cmd := exec.Command("git", "diff", "--cached")
    output, err := cmd.Output()
    if err != nil {
        return ""
    }
    return string(output)
}
```

## 🔧 具体工具适配器实现

### 1. Claude适配器

```go
// ClaudeSession Claude工具会话
type ClaudeSession struct {
    *TmuxToolSession // 嵌入基础会话
}

// NewClaudeSession 创建Claude会话
func NewClaudeSession(agentInstanceID string, anywhereClient *AnywhereClient) *ClaudeSession {
    base := &TmuxToolSession{
        toolName:        "claude",
        agentInstanceID: agentInstanceID,
        anywhereClient:  anywhereClient,
        tmuxManager:     NewTmuxManager(),
        outputProcessor: NewOutputProcessor(),
        permissionDetector: NewPermissionDetector(),
        messageProcessor: NewMessageProcessor(anywhereClient, agentInstanceID),
    }
    
    return &ClaudeSession{
        TmuxToolSession: base,
    }
}

// Start 启动Claude (学习Omnara的run_claude_with_pty逻辑)
func (cw *ClaudeWrapper) Start(ctx context.Context) error {
    // 1. 创建tmux会话
    sessionID, err := cw.tmuxManager.CreateSession("claude", cw.agentInstanceID)
    if err != nil {
        return fmt.Errorf("failed to create tmux session: %w", err)
    }
    cw.tmuxSessionID = sessionID
    
    // 2. 在tmux中启动Claude
    err = cw.tmuxManager.StartTool(sessionID, "claude", cw.agentInstanceID)
    if err != nil {
        return fmt.Errorf("failed to start Claude: %w", err)
    }
    
    // 3. 启动监控协程
    go cw.startOutputMonitoring(ctx)
    go cw.startInputProcessing(ctx)
    
    cw.running = true
    cw.lastActivity = time.Now()
    
    return nil
}

// startOutputMonitoring 启动输出监控 (学习Omnara的输出监控逻辑)
func (cw *ClaudeWrapper) startOutputMonitoring(ctx context.Context) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // 捕获tmux输出
            output, err := cw.tmuxManager.CapturePane(cw.tmuxSessionID)
            if err != nil {
                continue
            }
            
            // 检查是否有新输出
            if output != cw.terminalBuffer {
                newContent := output[len(cw.terminalBuffer):]
                cw.terminalBuffer = output
                
                // 处理新输出
                processed, err := cw.outputProcessor.ProcessOutput(newContent)
                if err != nil {
                    log.Printf("Error processing output: %v", err)
                    continue
                }
                
                // 检测权限提示
                if processed.HasPermissionPrompt {
                    prompt := cw.permissionDetector.DetectPermissionPrompt(processed.CleanContent)
                    if prompt != nil {
                        cw.handlePermissionPrompt(prompt)
                    }
                }
                
                // 检测空闲状态
                if processed.IsIdle {
                    cw.handleIdleState()
                }
                
                // 发送消息到Omnara
                if processed.CleanContent != "" {
                    cw.messageProcessor.ProcessAssistantMessage(processed.CleanContent, []string{})
                }
            }
        }
    }
}

// handlePermissionPrompt 处理权限提示 (学习Omnara的权限处理)
func (cw *ClaudeWrapper) handlePermissionPrompt(prompt *PermissionPrompt) {
    log.Printf("[INFO] Detected permission prompt: %s", prompt.Question)
    
    // 发送需要用户输入的消息到Omnara
    ctx := context.Background()
    req := &SendMessageRequest{
        Content:           prompt.Question,
        AgentInstanceID:   cw.agentInstanceID,
        RequiresUserInput: true,
        TimeoutMinutes:    1440, // 24小时
        PollInterval:      3.0,
    }
    
    response, err := cw.aiClient.SendMessage(ctx, req)
    if err != nil {
        log.Printf("Failed to send permission prompt: %v", err)
        return
    }
    
    // 处理用户响应
    for _, userResponse := range response.QueuedUserMessages {
        cw.processUserResponse(userResponse, prompt)
    }
}

// processUserResponse 处理用户响应
func (cw *ClaudeWrapper) processUserResponse(response string, prompt *PermissionPrompt) {
    // 将用户响应转换为适当的按键
    var keys string
    
    if mappedKey, exists := prompt.OptionsMap[strings.ToLower(response)]; exists {
        keys = strings.ToLower(response[:1]) // 使用映射的第一个字符
    } else {
        // 直接使用响应
        keys = response
    }
    
    // 发送按键到tmux会话
    err := cw.tmuxManager.SendKeys(cw.tmuxSessionID, keys, "Enter")
    if err != nil {
        log.Printf("Failed to send keys to tmux: %v", err)
    }
    
    log.Printf("[INFO] Sent user response to Claude: %s", keys)
}
```

### 2. Gemini适配器

```go
// GeminiSession Gemini工具会话
type GeminiSession struct {
    *TmuxToolSession
}

// NewGeminiSession 创建Gemini会话  
func NewGeminiSession(agentInstanceID string, anywhereClient *AnywhereClient) *GeminiSession {
    base := &TmuxToolWrapper{
        toolName:        "gemini",
        agentInstanceID: agentInstanceID,
        omnaraClient:    omnaraClient,
        // ... 其他初始化
    }
    
    return &GeminiWrapper{
        TmuxToolWrapper: base,
    }
}

// Start 启动Gemini
func (gw *GeminiWrapper) Start(ctx context.Context) error {
    // 类似Claude的启动逻辑，但适配Gemini CLI的特性
    // ...
}
```

### 3. Cursor适配器

```go
// CursorWrapper Cursor工具包装器
type CursorWrapper struct {
    *TmuxToolWrapper
}

// NewCursorWrapper 创建Cursor包装器
func NewCursorWrapper(agentInstanceID string, omnaraClient *OmnaraClient) *CursorWrapper {
    // 类似其他包装器的实现
    // ...
}
```

## 📦 SDK使用示例

### 1. 基本使用

```go
package main

import (
    "context"
    "log"
    
    "github.com/your-org/ai-terminal/pkg/sdk"
)

func main() {
    // 1. 创建anywhere客户端
    client := sdk.NewAnywhereClient("your-api-key", "https://anywhere-backend.com")
    
    // 2. 创建Claude会话
    claudeSession := sdk.NewClaudeSession("agent-instance-123", client)
    
    // 3. 启动Claude会话
    ctx := context.Background()
    if err := claudeSession.Start(ctx); err != nil {
        log.Fatalf("Failed to start Claude: %v", err)
    }
    defer claudeSession.Stop()
    
    log.Printf("Claude started in tmux session: %s", claudeSession.GetTmuxSessionID())
    
    // 4. 会话会自动与anywhere后端同步
    // 用户可以通过Web界面或其他设备访问
    
    // 5. 等待会话结束
    select {
    case <-ctx.Done():
        log.Println("Context cancelled")
    }
}
```

### 2. 跨设备恢复

```go
// 在另一个设备上恢复会话
func restoreSession() {
    // 1. 连接到现有的tmux会话
    sessionID := "ai-claude-abc12345" // 从anywhere后端获取
    
    // 2. 创建会话并连接到现有会话
    client := sdk.NewAnywhereClient("your-api-key", "https://anywhere-backend.com")
    claudeSession := sdk.NewClaudeSession("agent-instance-123", client)
    
    // 3. 连接到现有tmux会话
    err := claudeSession.AttachToSession(sessionID)
    if err != nil {
        log.Fatalf("Failed to attach to session: %v", err)
    }
    
    log.Printf("Attached to existing Claude session: %s", sessionID)
}
```

## 🎯 核心优势

### 1. **学习Omnara成功经验**
✅ **SDK模式** - 提供Go SDK而不是HTTP API  
✅ **消息驱动** - 完全兼容Omnara的消息系统  
✅ **权限处理** - 复用Omnara的权限检测逻辑  
✅ **状态管理** - 基于AgentInstance + Messages模式  

### 2. **tmux集成创新**
✅ **持久化会话** - 支持跨设备恢复  
✅ **原生终端访问** - `tmux attach` 直接访问  
✅ **统一接口** - 支持多种AI工具  

### 3. **简单可靠**
✅ **无复杂API** - 只提供SDK和基础功能  
✅ **基于成熟技术** - tmux + Omnara后端  
✅ **易于扩展** - 统一的工具适配器接口  

这个设计完全基于Omnara的成功模式，只是将PTY替换为tmux，并提供Go SDK用于集成。用户可以通过SDK创建和管理AI工具会话，支持跨设备的原生终端访问。