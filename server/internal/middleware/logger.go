// internal/middleware/logger.go
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/majiayu000/gin-starter/pkg/logger"
)

// Logger 返回日志中间件
// 已弃用：请使用 pkg/logger 包中的 GinLogger() 函数
func Logger() gin.HandlerFunc {
	return logger.GinLogger()
}

// Recovery 返回恢复中间件
// 已弃用：请使用 pkg/logger 包中的 GinRecovery() 函数
func Recovery() gin.HandlerFunc {
	return logger.GinRecovery()
}
