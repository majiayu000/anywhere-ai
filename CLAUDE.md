# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Anywhere AI CLI Manager is a unified platform for managing AI CLI tool sessions (Claude Code, Gemini CLI, Cursor, GitHub Copilot) across devices using tmux for terminal session management.

## Architecture

### Two-Server Architecture
- **Core Server** (`core/main.go`): Terminal and agent management via tmux (port 8080)
- **Backend Server** (`server/cmd/main.go`): OAuth, user management, API gateway

### Key Components
- **tmux Manager** (`core/tmux/manager.go`): Creates, attaches, detaches, and monitors tmux sessions
- **Tool Adapters** (`core/tools/adapters.go`): Adapters for Claude, Gemini, Cursor, Copilot with state parsing
- **Output Processor** (`core/output/processor.go`): Real-time output buffering and permission detection
- **Database** (`core/database/sqlite.go`): SQLite for session persistence

## Development Commands

### Running the Application
```bash
# Core server (terminal management)
cd core
go run main.go

# Backend server (API/auth)
cd server
go run cmd/main.go
```

### Building
```bash
# Build core server
cd core
go build -o anywhere-core main.go

# Build backend server
cd server
go build -o anywhere-server cmd/main.go
```

### Testing
```bash
# Run tests with coverage
go test ./... -cover

# Run specific package tests
go test ./core/tmux -v
go test ./core/tools -v
```

### Dependencies
```bash
# Update dependencies
go mod tidy

# Download dependencies
go mod download
```

## Key Technical Patterns

### Tool Adapter Interface
All AI tools implement the adapter pattern in `core/tools/adapters.go`:
- `GetCommand()`: Returns CLI command to launch tool
- `ParseOutput()`: Parses tool output to determine session state
- `IsPermissionPrompt()`: Detects permission request patterns
- Session states: `StateStarting`, `StateReady`, `StateProcessing`, `StateWaitingInput`, `StateError`

### tmux Session Management
Sessions are managed through `core/tmux/manager.go`:
- Sessions stored in memory with SQLite persistence
- Each session tracks: ID, Tool, WindowID, PaneID, Status, DeviceID
- Monitor output with 500ms polling interval
- Cross-device session restoration via `RestoreSession()`

### Database Models
Located in `core/database/models.go`, using GORM:
- User model with OAuth support
- AgentInstance for AI tool instances
- TerminalSession for tmux session persistence
- Auto-migration on startup

### Server Configuration
Backend server uses layered configuration (`server/configs/config.go`):
- YAML config file loading
- Environment variable overrides
- Database, Redis, OAuth, monitoring settings

## Important Notes

### Authentication
- Backend server handles OAuth (Google, Apple)
- Session management via Redis/memory cache
- JWT tokens for API authentication

### Error Handling
- Custom error types in `server/pkg/errors/errors.go`
- Structured logging via zap (`server/pkg/logger/logger.go`)
- Health checks for database, cache, memory, goroutines

### API Structure
- RESTful endpoints under `/api/v1/`
- WebSocket support at `/api/v1/ws`
- Swagger documentation auto-generated
- CORS middleware configured

### Cross-Device Features
- Sessions persist to SQLite
- Device discovery via mDNS (commented out, planned feature)
- Session state synchronization across devices