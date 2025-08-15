# 终端交互方案设计 - Go PTY vs tmux

## 🎯 方案概述

本文档分析了两种终端交互方案，并推荐使用 **Go PTY** 来实现终端与前端的交互，特别是将 Claude Code 的终端交互转换为前端可见的形式。

## 📊 方案对比

### Go PTY vs tmux 详细对比

| 特性 | Go PTY | tmux | 说明 |
|------|--------|------|------|
| **控制精度** | ✅ 完全控制 | ❌ 间接控制 | PTY 可直接管理终端状态 |
| **实时性** | ✅ 直接 I/O | ❌ 需要解析输出 | PTY 无中间层延迟 |
| **跨平台** | ✅ 跨平台支持 | ❌ Windows 支持差 | PTY 在所有平台都可用 |
| **依赖性** | ✅ 无外部依赖 | ❌ 需要安装 tmux | PTY 是 Go 标准库 |
| **前端集成** | ✅ 直接 WebSocket | ❌ 需要额外转换 | PTY 输出可直接发送 |
| **ANSI 支持** | ✅ 完整支持 | ✅ 完整支持 | 两者都支持 ANSI 转义序列 |
| **会话持久化** | ❌ 需要自己实现 | ✅ 天然支持 | tmux 的优势 |
| **多窗口** | ❌ 需要自己实现 | ✅ 天然支持 | tmux 的优势 |

### 性能对比

| 指标 | Go PTY | tmux | 差异 |
|------|--------|------|------|
| **延迟** | ~1ms | ~10-50ms | PTY 延迟更低 |
| **CPU 使用** | 低 | 中等 | PTY 更节省资源 |
| **内存占用** | 小 | 大 | PTY 内存占用更少 |
| **集成复杂度** | 简单 | 复杂 | PTY 集成更简单 |

## 🚀 推荐方案：Go PTY + 智能解析

### 选择理由

1. **完全控制** - 直接管理终端会话，无中间层
2. **实时响应** - 数据流直接传输，延迟最小
3. **格式解析** - 可以精确解析 ANSI 转义序列
4. **状态管理** - 精确跟踪终端状态和光标位置
5. **前端友好** - 输出格式可以直接适配前端需求

## 🏗️ 技术架构设计

### 整体架构
```
┌─────────────────┐    PTY     ┌─────────────────┐    WebSocket    ┌─────────────────┐
│  Claude Process │◄──────────►│  Go PTY Manager │◄──────────────►│  Frontend UI    │
│                 │            │                 │                │                 │
│ - stdin/stdout  │            │ - ANSI Parser   │                │ - Virtual Term  │
│ - Command Parse │            │ - Buffer Mgmt   │                │ - Real-time UI  │
└─────────────────┘            │ - State Track   │                │ - Command UI    │
                               └─────────────────┘                └─────────────────┘
```

### 核心组件

#### 1. PTY 管理器
```go
type TerminalManager struct {
    ptmx        *os.File           // PTY 主设备
    cmd         *exec.Cmd          // Claude 进程
    clients     map[string]*websocket.Conn  // WebSocket 客户端
    buffer      *TerminalBuffer    // 终端缓冲区
    parser      *ANSIParser        // ANSI 解析器
    detector    *CommandDetector   // 命令检测器
    state       *TerminalState     // 终端状态
}
```

#### 2. ANSI 解析器
```go
type ANSIParser struct {
    state       ParserState
    buffer      []byte
    currentLine int
    currentCol  int
}

type ParsedOutput struct {
    Type        string      `json:"type"`        // "text", "command", "cursor", "clear"
    Content     string      `json:"content"`     // 文本内容
    Position    *Position   `json:"position,omitempty"`    // 光标位置
    Style       *TextStyle  `json:"style,omitempty"`       // 文本样式
    Timestamp   time.Time   `json:"timestamp"`   // 时间戳
}
```

#### 3. 命令检测器
```go
type CommandDetector struct {
    patterns map[string]*regexp.Regexp
    state    CommandState
}

type CommandInfo struct {
    Type        string    `json:"type"`         // 命令类型
    Content     string    `json:"content"`      // 命令内容
    NeedsInput  bool      `json:"needs_input"`  // 是否需要用户输入
    Options     []string  `json:"options,omitempty"`  // 可选项
    Timestamp   time.Time `json:"timestamp"`
}
```

## 🔧 核心实现

### 1. PTY 启动和管理
```go
func (tm *TerminalManager) StartClaude(args []string) error {
    // 创建 PTY
    cmd := exec.Command("claude", args...)
    ptmx, err := pty.Start(cmd)
    if err != nil {
        return fmt.Errorf("failed to start PTY: %w", err)
    }
    
    tm.ptmx = ptmx
    tm.cmd = cmd
    
    // 设置终端大小
    tm.setTerminalSize(80, 24)
    
    // 开始处理输出
    go tm.handleOutput()
    go tm.monitorProcess()
    
    return nil
}

func (tm *TerminalManager) setTerminalSize(cols, rows int) error {
    ws := &winsize{
        Row: uint16(rows),
        Col: uint16(cols),
    }
    
    _, _, errno := syscall.Syscall(
        syscall.SYS_IOCTL,
        tm.ptmx.Fd(),
        uintptr(syscall.TIOCSWINSZ),
        uintptr(unsafe.Pointer(ws)),
    )
    
    if errno != 0 {
        return errno
    }
    return nil
}
```

### 2. 输出处理和解析
```go
func (tm *TerminalManager) handleOutput() {
    reader := bufio.NewReader(tm.ptmx)
    
    for {
        // 读取原始数据
        data := make([]byte, 4096)
        n, err := reader.Read(data)
        if err != nil {
            if err == io.EOF {
                tm.handleProcessExit()
                break
            }
            log.Printf("Error reading from PTY: %v", err)
            continue
        }
        
        // 解析 ANSI 序列
        parsed := tm.parser.Parse(data[:n])
        
        // 检测命令
        for _, output := range parsed {
            if cmd := tm.detector.DetectCommand(output.Content); cmd != nil {
                tm.handleCommand(cmd)
            }
        }
        
        // 更新终端缓冲区
        tm.buffer.Update(parsed)
        
        // 发送到前端
        tm.broadcastToClients(parsed)
    }
}
```

### 3. ANSI 序列解析
```go
func (p *ANSIParser) Parse(data []byte) []ParsedOutput {
    var results []ParsedOutput
    
    for i := 0; i < len(data); i++ {
        char := data[i]
        
        switch char {
        case '\x1b': // ESC 序列开始
            if i+1 < len(data) && data[i+1] == '[' {
                // 解析 CSI 序列
                seq, length := p.parseCSI(data[i:])
                if seq != nil {
                    results = append(results, *seq)
                }
                i += length - 1
            }
        case '\r':
            p.currentCol = 0
            results = append(results, ParsedOutput{
                Type:      "cursor",
                Position:  &Position{p.currentLine, p.currentCol},
                Timestamp: time.Now(),
            })
        case '\n':
            p.currentLine++
            p.currentCol = 0
            results = append(results, ParsedOutput{
                Type:      "cursor",
                Position:  &Position{p.currentLine, p.currentCol},
                Timestamp: time.Now(),
            })
        case '\b': // 退格
            if p.currentCol > 0 {
                p.currentCol--
            }
            results = append(results, ParsedOutput{
                Type:      "cursor",
                Position:  &Position{p.currentLine, p.currentCol},
                Timestamp: time.Now(),
            })
        default:
            // 普通字符
            if char >= 32 || char == '\t' { // 可打印字符或制表符
                results = append(results, ParsedOutput{
                    Type:      "text",
                    Content:   string(char),
                    Position:  &Position{p.currentLine, p.currentCol},
                    Timestamp: time.Now(),
                })
                p.currentCol++
            }
        }
    }
    
    return results
}

func (p *ANSIParser) parseCSI(data []byte) (*ParsedOutput, int) {
    // 查找 CSI 序列结束
    end := 2 // 跳过 ESC[
    for end < len(data) {
        char := data[end]
        if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') {
            break
        }
        end++
    }
    
    if end >= len(data) {
        return nil, 0
    }
    
    sequence := string(data[2:end])
    command := data[end]
    
    switch command {
    case 'H', 'f': // 光标位置
        return p.parseCursorPosition(sequence), end + 1
    case 'J': // 清屏
        return &ParsedOutput{
            Type:      "clear",
            Content:   sequence,
            Timestamp: time.Now(),
        }, end + 1
    case 'm': // 颜色和样式
        return p.parseStyle(sequence), end + 1
    default:
        return &ParsedOutput{
            Type:      "control",
            Content:   string(data[:end+1]),
            Timestamp: time.Now(),
        }, end + 1
    }
}
```

## 📱 前端集成

### 虚拟终端实现
```go
// WebSocket 消息格式
type TerminalMessage struct {
    Type      string        `json:"type"`      // "output", "command", "status"
    Data      []ParsedOutput `json:"data,omitempty"`
    Command   *CommandInfo  `json:"command,omitempty"`
    Status    string        `json:"status,omitempty"`
    Timestamp time.Time     `json:"timestamp"`
}

func (tm *TerminalManager) broadcastToClients(outputs []ParsedOutput) {
    message := TerminalMessage{
        Type:      "output",
        Data:      outputs,
        Timestamp: time.Now(),
    }
    
    data, err := json.Marshal(message)
    if err != nil {
        log.Printf("Error marshaling message: %v", err)
        return
    }
    
    tm.clientsMutex.RLock()
    defer tm.clientsMutex.RUnlock()
    
    for clientID, conn := range tm.clients {
        if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
            log.Printf("Error sending to client %s: %v", clientID, err)
            delete(tm.clients, clientID)
        }
    }
}
```

### 输入处理
```go
func (tm *TerminalManager) handleInput(input string) error {
    // 清理输入
    sanitized := tm.sanitizeInput(input)
    
    // 发送到 PTY
    _, err := tm.ptmx.Write([]byte(sanitized))
    if err != nil {
        return fmt.Errorf("failed to write to PTY: %w", err)
    }
    
    // 记录输入历史
    tm.inputHistory.Add(sanitized)
    
    return nil
}

func (tm *TerminalManager) sanitizeInput(input string) string {
    // 移除危险字符
    input = strings.ReplaceAll(input, "\x00", "")
    
    // 限制长度
    if len(input) > maxInputLength {
        input = input[:maxInputLength]
    }
    
    // 确保以换行符结束（如果需要）
    if !strings.HasSuffix(input, "\n") && !strings.HasSuffix(input, "\r") {
        input += "\n"
    }
    
    return input
}
```

## 🎯 Claude 命令特殊处理

### 命令检测模式
```go
func NewCommandDetector() *CommandDetector {
    return &CommandDetector{
        patterns: map[string]*regexp.Regexp{
            "cost":           regexp.MustCompile(`Session cost: \$[\d.]+`),
            "clear":          regexp.MustCompile(`Conversation cleared|History cleared`),
            "config":         regexp.MustCompile(`Configuration|Settings|Config`),
            "permission":     regexp.MustCompile(`Do you want.*\(esc to cancel\)`),
            "waiting_input":  regexp.MustCompile(`Waiting for.*input|Please provide`),
            "plan_mode":      regexp.MustCompile(`Would you like to proceed.*No, keep planning`),
            "error":          regexp.MustCompile(`Error:|Failed:|Exception:`),
            "success":        regexp.MustCompile(`Success|Completed|Done`),
        },
    }
}

func (cd *CommandDetector) DetectCommand(text string) *CommandInfo {
    for cmdType, pattern := range cd.patterns {
        if pattern.MatchString(text) {
            return &CommandInfo{
                Type:        cmdType,
                Content:     text,
                NeedsInput:  cd.needsUserInput(cmdType),
                Options:     cd.extractOptions(text, cmdType),
                Timestamp:   time.Now(),
            }
        }
    }
    return nil
}

func (cd *CommandDetector) extractOptions(text, cmdType string) []string {
    switch cmdType {
    case "permission":
        // 提取权限选项
        return cd.extractPermissionOptions(text)
    case "plan_mode":
        // 提取计划模式选项
        return []string{"Yes, proceed", "No, keep planning", "Cancel"}
    default:
        return nil
    }
}
```

## 🔍 优势总结

### Go PTY 方案的核心优势

1. **精确控制** - 完全控制终端状态，包括光标位置、屏幕内容、颜色样式
2. **实时性能** - 直接 I/O，无中间层，延迟最小
3. **前端友好** - 输出格式可以直接适配前端渲染需求
4. **跨平台** - 在 Windows、macOS、Linux 上都有良好支持
5. **简单集成** - 无外部依赖，集成复杂度低
6. **状态同步** - 可以精确同步终端状态到前端

### 相比 tmux 的优势

- **更低延迟** - 直接 PTY 通信 vs tmux 命令解析
- **更好控制** - 直接终端控制 vs tmux API 限制
- **更简部署** - 无需安装 tmux
- **更好集成** - 原生 Go 集成 vs 外部进程通信

## 🎨 前端渲染优化

### iOS/Web 虚拟终端实现

#### Swift 实现示例
```swift
class VirtualTerminal: ObservableObject {
    @Published var lines: [TerminalLine] = []
    @Published var cursorPosition = Position(line: 0, column: 0)
    @Published var isWaitingForInput = false

    private var buffer: [[TerminalCell]] = []
    private let maxLines = 1000
    private let maxColumns = 120

    func processOutput(_ outputs: [ParsedOutput]) {
        for output in outputs {
            switch output.type {
            case "text":
                insertText(output.content, at: output.position, style: output.style)
            case "cursor":
                updateCursorPosition(output.position)
            case "clear":
                clearScreen(output.content)
            case "control":
                handleControlSequence(output.content)
            default:
                break
            }
        }

        // 更新 UI
        DispatchQueue.main.async {
            self.updateDisplay()
        }
    }

    private func insertText(_ text: String, at position: Position?, style: TextStyle?) {
        let line = position?.line ?? cursorPosition.line
        let col = position?.column ?? cursorPosition.column

        // 确保缓冲区足够大
        ensureBufferSize(line: line, column: col + text.count)

        // 插入字符
        for (i, char) in text.enumerated() {
            if col + i < maxColumns {
                buffer[line][col + i] = TerminalCell(
                    character: char,
                    style: style ?? TextStyle(),
                    timestamp: Date()
                )
            }
        }

        // 更新光标位置
        cursorPosition = Position(line: line, column: col + text.count)
    }
}

struct TerminalCell {
    let character: Character
    let style: TextStyle
    let timestamp: Date

    init(character: Character = " ", style: TextStyle = TextStyle(), timestamp: Date = Date()) {
        self.character = character
        self.style = style
        self.timestamp = timestamp
    }
}

struct TextStyle {
    let foregroundColor: Color?
    let backgroundColor: Color?
    let isBold: Bool
    let isItalic: Bool
    let isUnderlined: Bool

    init(foregroundColor: Color? = nil, backgroundColor: Color? = nil,
         isBold: Bool = false, isItalic: Bool = false, isUnderlined: Bool = false) {
        self.foregroundColor = foregroundColor
        self.backgroundColor = backgroundColor
        self.isBold = isBold
        self.isItalic = isItalic
        self.isUnderlined = isUnderlined
    }
}
```

#### Web 实现示例 (React)
```javascript
class VirtualTerminal {
    constructor(container) {
        this.container = container;
        this.buffer = [];
        this.cursorPosition = { line: 0, column: 0 };
        this.maxLines = 1000;
        this.maxColumns = 120;

        this.initializeBuffer();
        this.setupEventListeners();
    }

    processOutput(outputs) {
        outputs.forEach(output => {
            switch (output.type) {
                case 'text':
                    this.insertText(output.content, output.position, output.style);
                    break;
                case 'cursor':
                    this.updateCursorPosition(output.position);
                    break;
                case 'clear':
                    this.clearScreen(output.content);
                    break;
                case 'control':
                    this.handleControlSequence(output.content);
                    break;
            }
        });

        this.render();
    }

    insertText(text, position, style) {
        const line = position?.line ?? this.cursorPosition.line;
        const col = position?.column ?? this.cursorPosition.column;

        // 确保缓冲区足够大
        this.ensureBufferSize(line, col + text.length);

        // 插入字符
        for (let i = 0; i < text.length; i++) {
            if (col + i < this.maxColumns) {
                this.buffer[line][col + i] = {
                    character: text[i],
                    style: style || {},
                    timestamp: Date.now()
                };
            }
        }

        // 更新光标
        this.cursorPosition = { line, column: col + text.length };
    }

    render() {
        // 使用虚拟 DOM 或直接 DOM 操作来渲染终端
        const terminalHTML = this.buffer.map((line, lineIndex) => {
            const lineHTML = line.map((cell, colIndex) => {
                const className = this.getCellClassName(cell.style);
                const isCursor = lineIndex === this.cursorPosition.line &&
                               colIndex === this.cursorPosition.column;

                return `<span class="${className} ${isCursor ? 'cursor' : ''}">${cell.character}</span>`;
            }).join('');

            return `<div class="terminal-line">${lineHTML}</div>`;
        }).join('');

        this.container.innerHTML = terminalHTML;
    }
}
```

## 🔧 性能优化策略

### 1. 缓冲区管理
```go
type TerminalBuffer struct {
    lines       [][]TerminalCell
    maxLines    int
    maxColumns  int
    scrollback  int
    mutex       sync.RWMutex
    dirty       map[int]bool  // 标记脏行
}

func (tb *TerminalBuffer) Update(outputs []ParsedOutput) {
    tb.mutex.Lock()
    defer tb.mutex.Unlock()

    for _, output := range outputs {
        switch output.Type {
        case "text":
            line := output.Position.Line
            col := output.Position.Column

            // 确保行存在
            tb.ensureLine(line)

            // 更新单元格
            if col < tb.maxColumns {
                tb.lines[line][col] = TerminalCell{
                    Character: rune(output.Content[0]),
                    Style:     output.Style,
                    Dirty:     true,
                }

                // 标记脏行
                tb.dirty[line] = true
            }
        }
    }
}

func (tb *TerminalBuffer) GetDirtyLines() map[int][]TerminalCell {
    tb.mutex.RLock()
    defer tb.mutex.RUnlock()

    result := make(map[int][]TerminalCell)
    for lineNum := range tb.dirty {
        if lineNum < len(tb.lines) {
            result[lineNum] = make([]TerminalCell, len(tb.lines[lineNum]))
            copy(result[lineNum], tb.lines[lineNum])
        }
    }

    // 清除脏标记
    tb.dirty = make(map[int]bool)

    return result
}
```

### 2. 增量更新
```go
func (tm *TerminalManager) sendIncrementalUpdate() {
    dirtyLines := tm.buffer.GetDirtyLines()
    if len(dirtyLines) == 0 {
        return
    }

    message := TerminalMessage{
        Type:      "incremental_update",
        DirtyLines: dirtyLines,
        Timestamp: time.Now(),
    }

    tm.broadcastToClients(message)
}
```

### 3. 批量处理
```go
type OutputBatcher struct {
    outputs   []ParsedOutput
    timer     *time.Timer
    batchSize int
    timeout   time.Duration
    callback  func([]ParsedOutput)
}

func (ob *OutputBatcher) Add(output ParsedOutput) {
    ob.outputs = append(ob.outputs, output)

    if len(ob.outputs) >= ob.batchSize {
        ob.flush()
    } else if ob.timer == nil {
        ob.timer = time.AfterFunc(ob.timeout, ob.flush)
    }
}

func (ob *OutputBatcher) flush() {
    if ob.timer != nil {
        ob.timer.Stop()
        ob.timer = nil
    }

    if len(ob.outputs) > 0 {
        ob.callback(ob.outputs)
        ob.outputs = ob.outputs[:0]
    }
}
```

## 🛡️ 错误处理和恢复

### 1. PTY 异常处理
```go
func (tm *TerminalManager) monitorProcess() {
    for {
        if tm.cmd.ProcessState != nil {
            if tm.cmd.ProcessState.Exited() {
                tm.handleProcessExit()
                break
            }
        }

        time.Sleep(time.Second)
    }
}

func (tm *TerminalManager) handleProcessExit() {
    exitCode := tm.cmd.ProcessState.ExitCode()

    message := TerminalMessage{
        Type:   "process_exit",
        Status: fmt.Sprintf("Process exited with code %d", exitCode),
        Timestamp: time.Now(),
    }

    tm.broadcastToClients(message)

    // 尝试重启（如果配置允许）
    if tm.config.AutoRestart {
        go tm.attemptRestart()
    }
}

func (tm *TerminalManager) attemptRestart() {
    time.Sleep(5 * time.Second) // 等待一段时间

    if err := tm.StartClaude(tm.lastArgs); err != nil {
        log.Printf("Failed to restart Claude: %v", err)
    } else {
        log.Println("Claude process restarted successfully")
    }
}
```

### 2. 连接恢复
```go
func (tm *TerminalManager) handleClientDisconnect(clientID string) {
    tm.clientsMutex.Lock()
    defer tm.clientsMutex.Unlock()

    if conn, exists := tm.clients[clientID]; exists {
        conn.Close()
        delete(tm.clients, clientID)

        log.Printf("Client %s disconnected", clientID)
    }
}

func (tm *TerminalManager) handleClientReconnect(clientID string, conn *websocket.Conn) {
    tm.clientsMutex.Lock()
    tm.clients[clientID] = conn
    tm.clientsMutex.Unlock()

    // 发送当前状态
    tm.sendCurrentState(clientID)

    log.Printf("Client %s reconnected", clientID)
}
```

## 📊 监控和调试

### 1. 性能指标
```go
type PerformanceMetrics struct {
    OutputLatency    time.Duration
    InputLatency     time.Duration
    BufferSize       int
    ClientCount      int
    ProcessCPU       float64
    ProcessMemory    int64
    LastUpdate       time.Time
}

func (tm *TerminalManager) collectMetrics() *PerformanceMetrics {
    return &PerformanceMetrics{
        OutputLatency: tm.getAverageOutputLatency(),
        InputLatency:  tm.getAverageInputLatency(),
        BufferSize:    tm.buffer.GetSize(),
        ClientCount:   len(tm.clients),
        ProcessCPU:    tm.getProcessCPU(),
        ProcessMemory: tm.getProcessMemory(),
        LastUpdate:    time.Now(),
    }
}
```

### 2. 调试日志
```go
type DebugLogger struct {
    file   *os.File
    level  LogLevel
    mutex  sync.Mutex
}

func (dl *DebugLogger) LogPTYData(direction string, data []byte) {
    if dl.level >= LogLevelDebug {
        dl.mutex.Lock()
        defer dl.mutex.Unlock()

        timestamp := time.Now().Format("2006-01-02 15:04:05.000")
        fmt.Fprintf(dl.file, "[%s] PTY %s: %q\n", timestamp, direction, data)
    }
}

func (dl *DebugLogger) LogCommand(cmd *CommandInfo) {
    if dl.level >= LogLevelInfo {
        dl.mutex.Lock()
        defer dl.mutex.Unlock()

        timestamp := time.Now().Format("2006-01-02 15:04:05.000")
        fmt.Fprintf(dl.file, "[%s] Command detected: %s (needs_input: %v)\n",
                   timestamp, cmd.Type, cmd.NeedsInput)
    }
}
```

## 🎯 最佳实践建议

### 1. 配置管理
```yaml
# config.yaml
terminal:
  max_lines: 1000
  max_columns: 120
  scroll_back: 10000
  auto_restart: true
  restart_delay: 5s

performance:
  batch_size: 100
  batch_timeout: 50ms
  max_clients: 10
  buffer_size: 4096

logging:
  level: info
  file: terminal.log
  max_size: 100MB
  max_backups: 5

security:
  max_input_length: 1024
  rate_limit: 100/min
  allowed_commands: ["claude", "/clear", "/cost", "/config"]
```

### 2. 部署建议
```bash
# 生产环境部署
go build -ldflags="-s -w" -o terminal-manager
sudo setcap 'cap_sys_ptrace=ep' terminal-manager  # 允许 PTY 操作

# 使用 systemd 管理
sudo systemctl enable terminal-manager
sudo systemctl start terminal-manager

# 监控和日志
journalctl -u terminal-manager -f
```

### 3. 安全考虑
- **输入验证** - 严格验证所有用户输入
- **权限控制** - 最小权限原则运行
- **资源限制** - 限制内存和 CPU 使用
- **审计日志** - 记录所有重要操作

---

**结论：Go PTY 是实现终端到前端交互的最佳方案**，特别适合需要精确控制和实时响应的场景。通过合理的架构设计和性能优化，可以实现高效、稳定、用户友好的终端交互体验。
