# Integration with Anywhere AI

This module is now part of the Anywhere AI platform.

## Directory Structure

```
server/                  # 后端服务器
├── cmd/                 # 服务入口点
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
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
JWT_SECRET=your-secret
DATABASE_URL=sqlite://anywhere.db
```
