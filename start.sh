#!/bin/bash

# Anywhere AI CLI Manager - 启动脚本

set -e

echo "🚀 Anywhere AI CLI Manager"
echo "=========================="

# 检查tmux是否安装
if ! command -v tmux &> /dev/null; then
    echo "❌ tmux not found. Installing..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install tmux
    else
        sudo apt-get install -y tmux
    fi
fi

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Please install Go first."
    echo "Visit: https://golang.org/dl/"
    exit 1
fi

# 设置Go模块代理（加速下载）
export GOPROXY=https://proxy.golang.org,direct

# 进入cli目录
cd cli

# 初始化Go模块（如果需要）
if [ ! -f "go.mod" ]; then
    echo "📦 Initializing Go module..."
    go mod init github.com/anywhere-ai/anywhere/cli
fi

# 添加本地模块替换（使用相对路径）
if ! grep -q "replace github.com/anywhere-ai/anywhere/core" go.mod; then
    echo "📝 Adding local module replacement..."
    echo "" >> go.mod
    echo "replace github.com/anywhere-ai/anywhere/core => ../core" >> go.mod
fi

# 下载依赖
echo "📥 Downloading dependencies..."
go mod tidy

# 编译
echo "🔨 Building anywhere CLI..."
go build -o anywhere main.go

# 创建符号链接到用户bin目录（可选）
if [ -d "$HOME/.local/bin" ]; then
    ln -sf "$(pwd)/anywhere" "$HOME/.local/bin/anywhere"
    echo "✅ Installed to ~/.local/bin/anywhere"
elif [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    ln -sf "$(pwd)/anywhere" /usr/local/bin/anywhere
    echo "✅ Installed to /usr/local/bin/anywhere"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "Quick Start:"
echo "───────────────────────────────"
echo "1. Create new Claude session:"
echo "   ./anywhere"
echo ""
echo "2. List all sessions:"
echo "   ./anywhere -list"
echo ""
echo "3. Restore session:"
echo "   ./anywhere -session <ID>"
echo ""
echo "4. Use different AI tool:"
echo "   ./anywhere -tool gemini"
echo "───────────────────────────────"

# 询问是否立即启动
echo ""
read -p "Start a new Claude session now? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    ./anywhere
fi