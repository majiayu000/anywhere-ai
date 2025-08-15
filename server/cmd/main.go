// cmd/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	config "github.com/majiayu000/gin-starter/configs"
	"github.com/majiayu000/gin-starter/docs"
	"github.com/majiayu000/gin-starter/internal/router"
	"github.com/majiayu000/gin-starter/pkg/cache"
	"github.com/majiayu000/gin-starter/pkg/database"
	"github.com/majiayu000/gin-starter/pkg/logger"
	"github.com/majiayu000/gin-starter/pkg/monitor"
	// "github.com/majiayu000/gin-starter/internal/auth"
	// "github.com/majiayu000/gin-starter/internal/auth/oauth"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志系统
	loggerConfig := logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		Output:     cfg.Log.Output,
		Filename:   cfg.Log.Filename,
		MaxSize:    cfg.Log.MaxSize,
		MaxAge:     cfg.Log.MaxAge,
		MaxBackups: cfg.Log.MaxBackups,
		Compress:   cfg.Log.Compress,
	}
	logger.Init(loggerConfig)

	// 初始化数据库
	dbConfig := database.Config{
		Driver:          cfg.Database.Driver,
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		Charset:         cfg.Database.Charset,
		Timezone:        cfg.Database.Timezone,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		LogLevel:        cfg.Database.LogLevel,
	}
	db, err := database.New(dbConfig)
	if err != nil {
		logger.Logger.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 初始化缓存
	var cacheClient cache.Cache
	if cfg.Redis.Addr != "" {
		redisConfig := cache.RedisConfig{
			Addr:         cfg.Redis.Addr,
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
			PoolSize:     cfg.Redis.PoolSize,
			MinIdleConns: cfg.Redis.MinIdleConns,
			DialTimeout:  cfg.Redis.DialTimeout,
			ReadTimeout:  cfg.Redis.ReadTimeout,
			WriteTimeout: cfg.Redis.WriteTimeout,
			IdleTimeout:  cfg.Redis.IdleTimeout,
		}
		cacheClient = cache.NewRedisCache(redisConfig)
	} else {
		cacheClient = cache.NewMemoryCache()
	}

	// 初始化健康检查器
	healthChecker := monitor.NewHealthChecker()
	healthChecker.Register(&database.DatabaseHealthAdapter{HealthCheck: &database.HealthCheck{DB: db}})
	healthChecker.Register(&cache.CacheHealthAdapter{Cache: cacheClient})
	if cfg.Monitor.MaxMemoryMB > 0 {
		healthChecker.Register(&monitor.MemoryHealthCheck{MaxMemoryMB: uint64(cfg.Monitor.MaxMemoryMB)})
	}
	if cfg.Monitor.MaxGoroutines > 0 {
		healthChecker.Register(&monitor.GoroutineHealthCheck{MaxGoroutines: cfg.Monitor.MaxGoroutines})
	}

	// 设置 Gin 模式
	if cfg.Server.Mode != "" {
		gin.SetMode(cfg.Server.Mode)
	}

	// 初始化 Swagger 文档
	docs.UpdateSwaggerInfo("1.0", "localhost:8080", "/api/v1", "Gin Starter API", "A starter template for Gin web applications")

	// TODO: 如需启用认证功能，取消注释以下代码
	/*
		// 初始化 OAuth 管理器
		oauthManager := auth.NewOAuthManager()

		// 初始化会话管理器
		sessionManager := auth.NewSessionManager("localhost:6379", "", 0)

		// 添加 Google OAuth 提供商
		googleProvider := oauth.NewGoogleProvider(
			cfg.OAuth.Google.ClientID,
			cfg.OAuth.Google.ClientSecret,
			cfg.OAuth.Google.RedirectURL,
		)
		oauthManager.AddProvider(oauth.Google, googleProvider)

		// 添加 Apple OAuth 提供商
		appleProvider := oauth.NewAppleProvider(
			cfg.OAuth.Apple.ClientID,
			cfg.OAuth.Apple.TeamID,
			cfg.OAuth.Apple.KeyID,
			cfg.OAuth.Apple.KeyPath,
			cfg.OAuth.Apple.RedirectURL,
		)
		oauthManager.AddProvider(oauth.Apple, appleProvider)
	*/

	// 初始化路由
	r := router.SetupRouter(cfg, db, cacheClient, healthChecker)
	// 如需启用认证功能，使用以下代码替换上面的路由初始化
	// r := router.SetupRouterWithAuth(oauthManager, sessionManager)

	// 创建 HTTP 服务器
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:           addr,
		Handler:        r,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeout) * time.Second,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// 在 goroutine 中启动服务器
	go func() {
		logger.Logger.Infof("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Logger.Info("Shutting down server...")

	// 给服务器 5 秒钟来完成现有请求
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Logger.Info("Server exited")
}
