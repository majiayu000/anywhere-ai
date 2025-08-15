.PHONY: build run clean test fmt vet tidy

# 应用名称
APP_NAME=go-web-starter

# 构建应用
build:
	@echo "Building $(APP_NAME)..."
	go build -o bin/$(APP_NAME) cmd/main.go

# 运行应用
run:
	@echo "Running $(APP_NAME)..."
	go run cmd/main.go

# 运行开发模式
dev:
	@echo "Running $(APP_NAME) in development mode..."
	GIN_MODE=debug go run cmd/main.go

# 清理构建文件
clean:
	@echo "Cleaning..."
	rm -rf bin/

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 格式化代码
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
vet:
	@echo "Running go vet..."
	go vet ./...

# 整理依赖
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# 安装依赖
install:
	@echo "Installing dependencies..."
	go mod download

# 完整检查
check: fmt vet test
	@echo "All checks passed!"

# 帮助
help:
	@echo "Available commands:"
	@echo "  build   - Build the application"
	@echo "  run     - Run the application"
	@echo "  dev     - Run in development mode"
	@echo "  clean   - Clean build files"
	@echo "  test    - Run tests"
	@echo "  fmt     - Format code"
	@echo "  vet     - Run go vet"
	@echo "  tidy    - Tidy dependencies"
	@echo "  install - Install dependencies"
	@echo "  check   - Run all checks"