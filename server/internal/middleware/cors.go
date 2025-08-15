package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORSConfig CORS配置
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig 默认CORS配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * 60 * 60, // 12小时
	}
}

// CORS 跨域中间件
func CORS(config ...CORSConfig) gin.HandlerFunc {
	var cfg CORSConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultCORSConfig()
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 检查是否允许该源
		allowed := false
		for _, allowedOrigin := range cfg.AllowOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		// 设置允许的方法
		if len(cfg.AllowMethods) > 0 {
			methods := ""
			for i, method := range cfg.AllowMethods {
				if i > 0 {
					methods += ", "
				}
				methods += method
			}
			c.Header("Access-Control-Allow-Methods", methods)
		}

		// 设置允许的头部
		if len(cfg.AllowHeaders) > 0 {
			headers := ""
			for i, header := range cfg.AllowHeaders {
				if i > 0 {
					headers += ", "
				}
				headers += header
			}
			c.Header("Access-Control-Allow-Headers", headers)
		}

		// 设置暴露的头部
		if len(cfg.ExposeHeaders) > 0 {
			headers := ""
			for i, header := range cfg.ExposeHeaders {
				if i > 0 {
					headers += ", "
				}
				headers += header
			}
			c.Header("Access-Control-Expose-Headers", headers)
		}

		// 设置是否允许凭证
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 设置预检请求缓存时间
		if cfg.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", string(rune(cfg.MaxAge)))
		}

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
