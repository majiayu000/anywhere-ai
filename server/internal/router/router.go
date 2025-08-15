// internal/router/router.go
package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	config "github.com/majiayu000/gin-starter/configs"
	"github.com/majiayu000/gin-starter/internal/middleware"
	"github.com/majiayu000/gin-starter/pkg/cache"
	"github.com/majiayu000/gin-starter/pkg/database"
	"github.com/majiayu000/gin-starter/pkg/logger"
	"github.com/majiayu000/gin-starter/pkg/monitor"
	"github.com/majiayu000/gin-starter/pkg/response"
	// "github.com/majiayu000/gin-starter/internal/auth"
	// "github.com/majiayu000/gin-starter/internal/handlers"
)

// SetupRouter 初始化基础路由（不包含认证功能）
func SetupRouter(cfg *config.Config, db *database.Database, cache cache.Cache, healthChecker *monitor.HealthChecker) *gin.Engine {
	r := gin.New()

	// CORS 中间件
	corsConfig := middleware.CORSConfig{
		AllowOrigins:     cfg.CORS.AllowOrigins,
		AllowMethods:     cfg.CORS.AllowMethods,
		AllowHeaders:     cfg.CORS.AllowHeaders,
		ExposeHeaders:    cfg.CORS.ExposeHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
	}
	r.Use(middleware.CORS(corsConfig))

	// 请求ID中间件
	r.Use(middleware.RequestID())

	// 日志中间件
	r.Use(logger.GinLogger())
	r.Use(logger.GinRecovery())

	// 限流中间件
	// 解析配置中的字符串为时间间隔
	rate, _ := time.ParseDuration(cfg.RateLimit.Rate)
	ttl, _ := time.ParseDuration(cfg.RateLimit.TTL)

	rateLimitConfig := middleware.RateLimitConfig{
		Capacity: int64(cfg.RateLimit.Capacity),
		Rate:     rate,
		TTL:      ttl,
		KeyFunc:  func(c *gin.Context) string { return c.ClientIP() },
	}
	r.Use(middleware.RateLimit(rateLimitConfig))

	// 指标收集中间件
	if cfg.Monitor.EnableMetrics {
		r.Use(monitor.MetricsMiddleware())
		// 指标端点
		r.GET("/metrics", monitor.MetricsHandler())
		r.GET("/metrics/summary", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Metrics summary endpoint",
				"status":  "active",
			})
		})
	}

	// Swagger 文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查端点
	if cfg.Monitor.EnableHealthCheck {
		r.GET("/health", monitor.HealthHandler(healthChecker))
		r.GET("/health/ready", monitor.ReadinessHandler(healthChecker))
		r.GET("/health/live", monitor.LivenessHandler())
	}

	// 根路径
	r.GET("/", func(c *gin.Context) {
		response.Success(c, gin.H{
			"message": "Welcome to Go Web Starter",
			"version": "1.0.0",
			"docs":    "/swagger/index.html",
		})
	})

	// API 路由组
	api := r.Group("/api/v1")
	{
		// 示例路由
		api.GET("/ping", func(c *gin.Context) {
			response.Success(c, gin.H{
				"message":    "pong",
				"timestamp":  time.Now().Unix(),
				"request_id": middleware.GetRequestID(c),
			})
		})

		// 缓存测试路由
		api.GET("/cache/test", func(c *gin.Context) {
			key := "test_key"
			value := "test_value"

			// 设置缓存
			err := cache.Set(c.Request.Context(), key, value, 5*time.Minute)
			if err != nil {
				response.Error(c, response.InternalErrorCode, "缓存设置失败")
				return
			}

			// 获取缓存
			cachedValue, err := cache.Get(c.Request.Context(), key)
			if err != nil {
				response.Error(c, response.InternalErrorCode, "缓存获取失败")
				return
			}

			response.Success(c, gin.H{
				"cached_value": cachedValue,
				"message":      "Cache test successful",
			})
		})

		// 在这里添加更多API路由
	}

	return r
}

// SetupRouterWithAuth 初始化带认证功能的路由
// 当需要启用认证功能时使用此函数
/*
func SetupRouterWithAuth(oauthManager *auth.OAuthManager, sessionManager auth.SessionManager) *gin.Engine {
	r := gin.Default()

	// 使用中间件
	r.Use(middleware.Logger())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"message": "Service is running",
		})
	})

	// 根路径
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to Go Web Starter with Auth",
			"version": "1.0.0",
		})
	})

	// 初始化认证处理器
	authHandler := handlers.NewAuthHandler(oauthManager, sessionManager)

	// 认证路由
	auth := r.Group("/auth")
	{
		auth.GET("/login/:provider", authHandler.Login)
		auth.GET("/callback/:provider", authHandler.Callback)
		auth.POST("/logout", authHandler.Logout)
	}

	// API 路由组
	api := r.Group("/api/v1")
	{
		// 公开路由
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})

		// 需要认证的路由
		protected := api.Group("/")
		protected.Use(middleware.AuthRequired(sessionManager))
		{
			protected.GET("/user/:id", handlers.GetUser)
			// 在这里添加更多需要认证的API路由
		}
	}

	return r
}
*/
