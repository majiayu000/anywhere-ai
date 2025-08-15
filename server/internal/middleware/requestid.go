package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// RequestIDHeader 请求ID头部名称
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey 在gin.Context中存储请求ID的key
	RequestIDKey = "request_id"
)

var (
	// 用于生成递增序列号
	counter uint64
)

// RequestIDConfig 请求ID配置
type RequestIDConfig struct {
	Header    string                    // 请求ID头部名称
	Generator func(*gin.Context) string // 自定义生成器
	Skipper   func(*gin.Context) bool   // 跳过条件
}

// DefaultRequestIDConfig 默认请求ID配置
func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		Header:    RequestIDHeader,
		Generator: DefaultRequestIDGenerator,
		Skipper:   nil,
	}
}

// DefaultRequestIDGenerator 默认请求ID生成器
func DefaultRequestIDGenerator(c *gin.Context) string {
	// 首先检查请求头中是否已有请求ID
	if requestID := c.GetHeader(RequestIDHeader); requestID != "" {
		return requestID
	}

	// 生成新的请求ID：时间戳 + 随机数 + 递增序列
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	seq := atomic.AddUint64(&counter, 1)

	return fmt.Sprintf("%d-%s-%d", timestamp, randomHex, seq)
}

// UUIDGenerator UUID风格的请求ID生成器
func UUIDGenerator(c *gin.Context) string {
	// 检查请求头中是否已有请求ID
	if requestID := c.GetHeader(RequestIDHeader); requestID != "" {
		return requestID
	}

	// 生成UUID风格的ID
	b := make([]byte, 16)
	rand.Read(b)

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ShortIDGenerator 短ID生成器
func ShortIDGenerator(c *gin.Context) string {
	// 检查请求头中是否已有请求ID
	if requestID := c.GetHeader(RequestIDHeader); requestID != "" {
		return requestID
	}

	// 生成8位随机字符串
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestID 请求ID中间件
func RequestID(config ...RequestIDConfig) gin.HandlerFunc {
	var cfg RequestIDConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultRequestIDConfig()
	}

	// 设置默认值
	if cfg.Header == "" {
		cfg.Header = RequestIDHeader
	}
	if cfg.Generator == nil {
		cfg.Generator = DefaultRequestIDGenerator
	}

	return func(c *gin.Context) {
		// 检查是否跳过
		if cfg.Skipper != nil && cfg.Skipper(c) {
			c.Next()
			return
		}

		// 生成请求ID
		requestID := cfg.Generator(c)

		// 设置到响应头
		c.Header(cfg.Header, requestID)

		// 存储到上下文中
		c.Set(RequestIDKey, requestID)

		c.Next()
	}
}

// GetRequestID 从上下文中获取请求ID
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		return requestID.(string)
	}
	return ""
}

// RequestIDWithUUID 使用UUID生成器的请求ID中间件
func RequestIDWithUUID() gin.HandlerFunc {
	return RequestID(RequestIDConfig{
		Header:    RequestIDHeader,
		Generator: UUIDGenerator,
	})
}

// RequestIDWithShortID 使用短ID生成器的请求ID中间件
func RequestIDWithShortID() gin.HandlerFunc {
	return RequestID(RequestIDConfig{
		Header:    RequestIDHeader,
		Generator: ShortIDGenerator,
	})
}
