# 🚀 Anywhere AI CLI Manager - 使用指南

## 安装准备

### 1. 安装必要依赖

```bash
# macOS
brew install tmux
brew install go

# Ubuntu/Debian  
sudo apt-get install tmux golang

# 检查安装
tmux -V
go version
```

### 2. 编译项目

```bash
cd anywhere/cli
go build -o anywhere main.go
```

## 基本使用

### 创建新的Claude会话

```bash
# 使用默认Claude
./anywhere

# 指定工具类型
./anywhere -tool gemini
./anywhere -tool cursor
```

### 列出所有会话

```bash
./anywhere -list

# 输出示例：
📋 Active Sessions:
─────────────────────────────────────────────
ID: claude-1234567890
  Tool: claude | Device: MacBook-Pro
  Status: ready | Last Active: 2024-01-15 14:30:00
─────────────────────────────────────────────
```

### 恢复/附加到现有会话

```bash
# 从列表中获取session ID
./anywhere -session claude-1234567890
```

## 交互命令

启动后，可以使用以下命令：

- **直接输入文本** - 发送到AI工具
- **`exit`** - 退出程序（会话保持运行）
- **`kill`** - 终止会话并退出
- **`status`** - 显示会话状态
- **`clear`** - 清屏

## 跨设备使用场景

### 场景1：在Mac上创建，在另一台Mac恢复

```bash
# Mac A - 创建会话
./anywhere -tool claude
> Hello Claude, help me write a Python script
> exit  # 退出但保持会话

# Mac B - 恢复会话
./anywhere -list  # 查看会话ID
./anywhere -session claude-1234567890
> # 继续之前的对话
```

### 场景2：后台运行会话

```bash
# 创建会话后退出
./anywhere
> Start working on the project
> exit

# 稍后恢复
./anywhere -session claude-1234567890
```

## 直接使用tmux

如果你熟悉tmux，也可以直接操作：

```bash
# 查看所有tmux会话
tmux ls

# 直接附加到tmux会话
tmux attach -t claude-1234567890

# 分离会话 (在tmux内)
Ctrl+b, d
```

## 权限处理

当AI工具请求权限时，会看到提示：

```
⚠️  Permission Request: Tool wants to write to file
Options: [y n]
Response: y
```

输入对应选项即可。

## 配置文件

可以创建配置文件 `~/.anywhere/config.json`：

```json
{
  "default_tool": "claude",
  "db_path": "~/.anywhere/sessions.db",
  "auto_save": true
}
```

## 故障排除

### tmux未安装
```bash
Error: tmux not found
解决：brew install tmux
```

### 会话无法恢复
```bash
# 检查tmux会话
tmux ls

# 清理死会话
./anywhere -list
./anywhere -clean  # 清理无效会话
```

### 权限问题
```bash
# 确保有执行权限
chmod +x anywhere
```

## 高级用法

### 批量操作

```bash
# 创建多个会话
for tool in claude gemini cursor; do
  ./anywhere -tool $tool &
done

# 列出所有会话
./anywhere -list
```

### 脚本集成

```go
// 在Go代码中使用
import "github.com/anywhere-ai/anywhere/core/tools"

manager := tools.NewSessionManager(tmuxManager)
session, _ := manager.CreateSession(ctx, tools.ToolClaude, "my-session")
manager.SendInput(ctx, session.ID, "Hello Claude!")
```

## 示例工作流

### 1. 开始新项目
```bash
./anywhere -tool claude
> Help me create a REST API in Go
> What database should I use?
> exit
```

### 2. 切换设备继续
```bash
# 在另一台设备
./anywhere -list
./anywhere -session claude-xxxxx
> Let's continue with PostgreSQL
```

### 3. 同时使用多个AI
```bash
# 终端1
./anywhere -tool claude

# 终端2  
./anywhere -tool gemini

# 终端3
./anywhere -tool cursor
```

## 常见问题

**Q: 会话会自动保存吗？**
A: 是的，所有会话都保存在SQLite数据库中。

**Q: 可以同时运行多个会话吗？**
A: 可以，每个会话独立运行在自己的tmux session中。

**Q: 如何完全清理所有会话？**
A: 运行 `tmux kill-server` 清理所有tmux会话。

**Q: 支持哪些AI工具？**
A: 目前支持Claude、Gemini、Cursor、GitHub Copilot。

## 获取帮助

```bash
./anywhere -help
```

或查看项目README获取更多信息。