#!/bin/bash

# 清理go-web-starter子模块并整合到anywhere-ai项目

echo "🧹 清理go-web-starter Git子模块"
echo "================================"

# 1. 保存go-web-starter内容（排除.git）
echo "📦 备份go-web-starter内容..."
if [ -d "go-web-starter-backup" ]; then
    rm -rf go-web-starter-backup
fi
cp -r go-web-starter go-web-starter-backup

# 2. 删除.git目录
echo "🗑️  移除go-web-starter/.git..."
rm -rf go-web-starter/.git
rm -f go-web-starter/.gitignore

# 3. 创建新的README说明这是项目的一部分
echo "📝 更新go-web-starter README..."
cat > go-web-starter/README.md << 'EOF'
# Web API Module

This module provides the web API interface for Anywhere AI.

## Overview

Originally based on go-web-starter template, now integrated as part of the Anywhere AI platform.

## Features

- RESTful API endpoints
- Authentication & Authorization  
- WebSocket support
- Health checks
- Metrics collection

## Usage

This module is used internally by the Anywhere AI platform for:
- Web dashboard API
- Remote control interface
- Real-time communication

See main project README for more information.
EOF

# 4. 更新go.mod为项目子模块
echo "📦 更新go.mod..."
cat > go-web-starter/go.mod << 'EOF'
module github.com/majiayu000/anywhere-ai/web

go 1.21

require (
	github.com/gin-contrib/cors v1.4.0
	github.com/gin-gonic/gin v1.9.1
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/joho/godotenv v1.5.1
	github.com/prometheus/client_golang v1.17.0
	github.com/spf13/viper v1.17.0
	github.com/stretchr/testify v1.8.4
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.0
	github.com/swaggo/swag v1.16.2
	go.uber.org/zap v1.26.0
	golang.org/x/crypto v0.14.0
	golang.org/x/oauth2 v0.13.0
	golang.org/x/time v0.3.0
	gorm.io/driver/postgres v1.5.3
	gorm.io/gorm v1.25.5
)
EOF

# 5. 移除不需要的文件
echo "🧹 清理不必要的文件..."
rm -f go-web-starter/.air.toml
rm -f go-web-starter/Dockerfile
rm -f go-web-starter/README_OPTIMIZATION.md

# 6. 创建集成配置
echo "⚙️  创建集成配置..."
cat > go-web-starter/INTEGRATION.md << 'EOF'
# Integration with Anywhere AI

This module is now part of the Anywhere AI platform.

## Directory Structure

```
go-web-starter/           → web/  (建议重命名)
├── cmd/                 # 入口点
├── internal/            # 内部实现
├── pkg/                 # 公共包
└── configs/             # 配置文件
```

## API Endpoints

- `/api/v1/sessions` - Session management
- `/api/v1/tools` - Tool management  
- `/api/v1/devices` - Device discovery
- `/ws` - WebSocket connections

## Environment Variables

```bash
WEB_PORT=8080
WEB_HOST=0.0.0.0
JWT_SECRET=your-secret
DATABASE_URL=sqlite://anywhere.db
```
EOF

echo ""
echo "✅ 清理完成！"
echo ""
echo "建议的下一步操作："
echo "────────────────────────────"
echo "1. 重命名目录："
echo "   mv go-web-starter server"
echo ""
echo "2. 更新引用："
echo "   将所有 go-web-starter 引用改为 server"
echo ""
echo "3. 提交更改："
echo "   git add ."
echo "   git commit -m 'Integrate web module (formerly go-web-starter)'"
echo "────────────────────────────"