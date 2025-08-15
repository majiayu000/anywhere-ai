package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(key string) bool
}

// TokenBucket 令牌桶限流器
type TokenBucket struct {
	mu       sync.RWMutex
	buckets  map[string]*bucket
	capacity int64         // 桶容量
	rate     time.Duration // 令牌生成速率
	ttl      time.Duration // 桶的生存时间
}

type bucket struct {
	tokens   int64
	lastTime time.Time
}

// NewTokenBucket 创建令牌桶限流器
func NewTokenBucket(capacity int64, rate time.Duration, ttl time.Duration) *TokenBucket {
	tb := &TokenBucket{
		buckets:  make(map[string]*bucket),
		capacity: capacity,
		rate:     rate,
		ttl:      ttl,
	}

	// 启动清理协程
	go tb.cleanup()

	return tb
}

// Allow 检查是否允许请求
func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	b, exists := tb.buckets[key]

	if !exists {
		b = &bucket{
			tokens:   tb.capacity - 1, // 消耗一个令牌
			lastTime: now,
		}
		tb.buckets[key] = b
		return true
	}

	// 计算应该添加的令牌数
	elapsed := now.Sub(b.lastTime)
	tokensToAdd := int64(elapsed / tb.rate)

	if tokensToAdd > 0 {
		b.tokens += tokensToAdd
		if b.tokens > tb.capacity {
			b.tokens = tb.capacity
		}
		b.lastTime = now
	}

	// 检查是否有可用令牌
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// cleanup 清理过期的桶
func (tb *TokenBucket) cleanup() {
	ticker := time.NewTicker(tb.ttl)
	defer ticker.Stop()

	for range ticker.C {
		tb.mu.Lock()
		now := time.Now()
		for key, b := range tb.buckets {
			if now.Sub(b.lastTime) > tb.ttl {
				delete(tb.buckets, key)
			}
		}
		tb.mu.Unlock()
	}
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Capacity int64                     // 桶容量（每个时间窗口允许的请求数）
	Rate     time.Duration             // 令牌生成速率
	TTL      time.Duration             // 桶的生存时间
	KeyFunc  func(*gin.Context) string // 生成限流key的函数
}

// DefaultRateLimitConfig 默认限流配置
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Capacity: 100,                                                 // 100个请求
		Rate:     time.Minute,                                         // 每分钟
		TTL:      time.Hour,                                           // 1小时TTL
		KeyFunc:  func(c *gin.Context) string { return c.ClientIP() }, // 基于IP限流
	}
}

// RateLimit 限流中间件
func RateLimit(config ...RateLimitConfig) gin.HandlerFunc {
	var cfg RateLimitConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultRateLimitConfig()
	}

	limiter := NewTokenBucket(cfg.Capacity, cfg.Rate, cfg.TTL)

	return func(c *gin.Context) {
		key := cfg.KeyFunc(c)

		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByUser 基于用户ID的限流
func RateLimitByUser(capacity int64, rate time.Duration) gin.HandlerFunc {
	config := RateLimitConfig{
		Capacity: capacity,
		Rate:     rate,
		TTL:      time.Hour,
		KeyFunc: func(c *gin.Context) string {
			// 尝试从上下文中获取用户ID
			if userID, exists := c.Get("user_id"); exists {
				return "user:" + userID.(string)
			}
			// 如果没有用户ID，则使用IP
			return "ip:" + c.ClientIP()
		},
	}
	return RateLimit(config)
}

// RateLimitByAPI 基于API路径的限流
func RateLimitByAPI(capacity int64, rate time.Duration) gin.HandlerFunc {
	config := RateLimitConfig{
		Capacity: capacity,
		Rate:     rate,
		TTL:      time.Hour,
		KeyFunc: func(c *gin.Context) string {
			return c.Request.Method + ":" + c.FullPath()
		},
	}
	return RateLimit(config)
}
