#!/bin/bash

# 检查项目状态

echo "🔍 Anywhere AI 项目状态检查"
echo "==========================="
echo ""

# 检查目录结构
echo "📁 目录结构:"
echo "────────────"
if [ -d "core" ]; then
    echo "✅ core/ - 核心模块"
else
    echo "❌ core/ - 缺失"
fi

if [ -d "server" ]; then
    echo "✅ server/ - 后端服务器 (原go-web-starter)"
else
    echo "❌ server/ - 缺失"
fi

if [ -d "cli" ]; then
    echo "✅ cli/ - 命令行客户端"
else
    echo "❌ cli/ - 缺失"
fi

if [ -d "pkg/sdk" ]; then
    echo "✅ pkg/sdk/ - Go SDK"
else
    echo "❌ pkg/sdk/ - 缺失"
fi

if [ -d "examples" ]; then
    echo "✅ examples/ - 示例代码"
else
    echo "❌ examples/ - 缺失"
fi

echo ""

# 检查Git状态
echo "📊 Git状态:"
echo "──────────"
if [ -d ".git" ]; then
    echo "✅ Git仓库已初始化"
    
    # 检查remote
    if git remote -v | grep -q "majiayu000/anywhere-ai"; then
        echo "✅ 远程仓库已配置: github.com/majiayu000/anywhere-ai"
    else
        echo "⚠️  远程仓库未配置或不正确"
    fi
    
    # 检查是否有未提交的更改
    if [ -n "$(git status --porcelain)" ]; then
        echo "⚠️  有未提交的更改"
        echo ""
        echo "建议运行:"
        echo "  git add ."
        echo "  git commit -m 'Integrate web module and complete project structure'"
    else
        echo "✅ 所有更改已提交"
    fi
else
    echo "❌ Git仓库未初始化"
    echo "  运行: ./init-repo.sh"
fi

echo ""

# 检查go-web-starter残留
echo "🧹 清理状态:"
echo "──────────"
if [ -d "go-web-starter" ]; then
    echo "⚠️  go-web-starter目录仍存在，建议删除"
else
    echo "✅ go-web-starter已清理"
fi

if [ -d "go-web-starter-backup" ]; then
    echo "⚠️  备份目录存在，建议删除: rm -rf go-web-starter-backup"
else
    echo "✅ 无备份残留"
fi

if [ -d "server/.git" ]; then
    echo "❌ server/.git存在，需要删除: rm -rf server/.git"
else
    echo "✅ server模块已脱离git子模块"
fi

echo ""

# 显示项目统计
echo "📊 项目统计:"
echo "──────────"
GO_FILES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | wc -l)
echo "Go文件数: $GO_FILES"

if [ -f "go.mod" ]; then
    MODULES=$(grep -c "^module " go.mod server/go.mod core/go.mod cli/go.mod 2>/dev/null | paste -sd+ | bc)
    echo "Go模块数: ${MODULES:-0}"
fi

echo ""

# 最终建议
echo "💡 下一步建议:"
echo "──────────────"
echo "1. 提交所有更改:"
echo "   git add ."
echo "   git commit -m 'Complete project integration'"
echo ""
echo "2. 推送到GitHub:"
echo "   git push -u origin main"
echo ""
echo "3. 创建首个Release:"
echo "   git tag v0.1.0"
echo "   git push origin v0.1.0"
echo ""
echo "✨ 项目已准备就绪！"