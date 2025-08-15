# Anywhere AI - 统一的AI CLI管理平台

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

让你的AI CLI工具随处可用 - 跨设备、跨平台的统一管理方案

## 🌟 核心特性

- 🤖 **多AI工具支持** - Claude、Gemini、Cursor、GitHub Copilot等
- 📱 **跨设备会话** - 在iPhone创建，在Mac恢复，无缝切换
- 🔄 **会话持久化** - 基于tmux的强大会话管理
- 🔐 **智能权限检测** - 自动识别文件、命令、网络权限请求
- 💾 **轻量存储** - SQLite本地数据库，无需复杂配置
- 🚀 **即插即用** - 简单命令即可开始使用

## 📦 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/majiayu000/anywhere-ai.git
cd anywhere-ai

# 运行快速启动
./quick-start.sh
```

### 基本使用

```bash
# 创建Claude会话
./cli/anywhere

# 列出所有会话
./cli/anywhere -list

# 恢复会话
./cli/anywhere -session claude-1234567890

# 使用其他AI工具
./cli/anywhere -tool gemini
```

## 🏗️ 项目架构

```
anywhere-ai/
├── core/                     # 核心功能模块
│   ├── tmux/                # tmux会话管理
│   ├── tools/               # AI工具适配器
│   ├── output/              # 输出处理器
│   └── database/            # 数据持久化
├── server/                  # 后端服务器
│   ├── cmd/                 # 服务入口
│   ├── internal/            # 内部实现
│   └── pkg/                 # 公共包
├── cli/                     # 命令行客户端
├── pkg/sdk/                 # Go SDK
└── examples/                # 使用示例
```

## 💡 使用场景

### 场景1：移动办公

早上在家用Mac开始和Claude讨论项目架构，路上用iPhone继续查看，到公司后在工作电脑上无缝继续。

### 场景2：多AI协作

同时运行Claude处理代码、Gemini分析数据、Cursor编辑文件，统一管理所有会话。

### 场景3：长时任务

启动一个AI辅助的代码重构任务，随时断开重连，任务持续进行。

## 🛠️ 高级功能

### 工具适配器系统

轻松添加新的AI工具支持：

```go
type ToolAdapter interface {
    GetCommand() []string
    ParseOutput(output string) SessionState
    IsPermissionPrompt(output string) bool
    FormatInput(input string) string
}
```

### 跨设备发现

基于mDNS的设备发现机制，自动找到局域网内的其他设备会话。

### 权限智能处理

自动检测并提示：
- 文件写入权限
- 系统命令执行权限  
- 网络访问权限

## 🔧 配置

创建 `~/.anywhere/config.json`:

```json
{
  "default_tool": "claude",
  "db_path": "~/.anywhere/sessions.db",
  "auto_save": true,
  "permission_mode": "ask"
}
```

## 📚 文档

- [使用指南](USAGE.md)
- [架构设计](ai-cli-manager-structure.md)
- [API文档](docs/api.md)

## 🤝 贡献

欢迎贡献代码！请查看 [贡献指南](CONTRIBUTING.md)。

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE)

## 🙏 致谢

- [tmux](https://github.com/tmux/tmux) - 强大的终端复用器
- [Omnara](https://github.com/omnara) - 架构灵感来源
- 所有AI工具的开发者们

## 📮 联系

- GitHub: [@majiayu000](https://github.com/majiayu000)
- Issues: [提交问题](https://github.com/majiayu000/anywhere-ai/issues)

---

**让AI随处可及，让效率无处不在！** 🚀