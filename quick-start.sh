#!/bin/bash

# 快速启动脚本 - 用于测试

echo "🚀 Anywhere AI Manager - Quick Start"
echo "===================================="

# 检查tmux
if ! command -v tmux &> /dev/null; then
    echo "❌ 请先安装tmux: brew install tmux"
    exit 1
fi

# 简单的测试：直接使用tmux创建Claude会话
SESSION_NAME="claude-test-$(date +%s)"

echo "📱 创建测试会话: $SESSION_NAME"

# 创建tmux会话并运行claude（假设claude已安装）
tmux new-session -d -s "$SESSION_NAME" -n "claude-test"

# 检查claude是否安装
if command -v claude &> /dev/null; then
    echo "✅ 启动Claude..."
    tmux send-keys -t "$SESSION_NAME" "claude" Enter
else
    echo "⚠️  Claude未安装，创建模拟会话..."
    tmux send-keys -t "$SESSION_NAME" "echo 'Claude simulation mode'" Enter
    tmux send-keys -t "$SESSION_NAME" "echo 'Type your message:'" Enter
fi

echo ""
echo "✅ 会话已创建！"
echo ""
echo "使用方法:"
echo "────────────────────────────"
echo "1. 附加到会话:"
echo "   tmux attach -t $SESSION_NAME"
echo ""
echo "2. 查看所有会话:"
echo "   tmux ls"
echo ""
echo "3. 分离会话 (在tmux内):"
echo "   按 Ctrl+b, 然后按 d"
echo ""
echo "4. 终止会话:"
echo "   tmux kill-session -t $SESSION_NAME"
echo "────────────────────────────"
echo ""
read -p "现在附加到会话吗? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    tmux attach -t "$SESSION_NAME"
fi