#!/bin/bash

# 初始化并推送到GitHub仓库

echo "🚀 初始化 Anywhere AI 仓库"
echo "=========================="

# 检查是否已经是git仓库
if [ -d .git ]; then
    echo "⚠️  已经是git仓库"
else
    echo "📦 初始化git仓库..."
    git init
fi

# 创建.gitignore
cat > .gitignore << 'EOF'
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
anywhere
cli/anywhere

# Test binary
*.test

# Output
*.out

# Dependency directories
vendor/

# Go workspace
go.work

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Database
*.db
*.sqlite
*.sqlite3

# Logs
*.log

# Temp
tmp/
temp/
EOF

echo "✅ Created .gitignore"

# 添加所有文件
git add .

# 创建初始提交
git commit -m "Initial commit: Anywhere AI CLI Manager

- Multi-tool AI CLI management platform
- Support for Claude, Gemini, Cursor, Copilot
- tmux-based session management
- Cross-device session recovery
- SQLite persistence
- Permission detection system"

# 添加远程仓库
echo ""
echo "📡 添加GitHub远程仓库..."
git remote add origin https://github.com/majiayu000/anywhere-ai.git

echo ""
echo "✅ 仓库初始化完成!"
echo ""
echo "推送到GitHub:"
echo "─────────────────────────"
echo "git push -u origin main"
echo ""
echo "或强制推送（如果远程已有内容）:"
echo "git push -u origin main --force"
echo "─────────────────────────"