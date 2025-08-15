package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 全局日志实例
var Logger *logrus.Logger

// Config 日志配置
type Config struct {
	Level      string `mapstructure:"level" yaml:"level"`
	Format     string `mapstructure:"format" yaml:"format"`
	Output     string `mapstructure:"output" yaml:"output"`
	Filename   string `mapstructure:"filename" yaml:"filename"`
	MaxSize    int    `mapstructure:"max_size" yaml:"max_size"`
	MaxAge     int    `mapstructure:"max_age" yaml:"max_age"`
	MaxBackups int    `mapstructure:"max_backups" yaml:"max_backups"`
	Compress   bool   `mapstructure:"compress" yaml:"compress"`
}

// Init 初始化日志系统
func Init(config Config) {
	Logger = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Logger.SetLevel(level)

	// 设置日志格式
	switch config.Format {
	case "json":
		Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	default:
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	// 设置输出
	switch config.Output {
	case "file":
		if config.Filename == "" {
			config.Filename = "logs/app.log"
		}
		// 确保日志目录存在
		logDir := filepath.Dir(config.Filename)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			Logger.Errorf("创建日志目录失败: %v", err)
			return
		}

		// 配置日志轮转
		lumberjackLogger := &lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize,    // MB
			MaxAge:     config.MaxAge,     // days
			MaxBackups: config.MaxBackups, // files
			Compress:   config.Compress,
		}
		Logger.SetOutput(lumberjackLogger)
	case "both":
		if config.Filename == "" {
			config.Filename = "logs/app.log"
		}
		// 确保日志目录存在
		logDir := filepath.Dir(config.Filename)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			Logger.Errorf("创建日志目录失败: %v", err)
			return
		}

		// 同时输出到控制台和文件
		lumberjackLogger := &lumberjack.Logger{
			Filename:   config.Filename,
			MaxSize:    config.MaxSize,
			MaxAge:     config.MaxAge,
			MaxBackups: config.MaxBackups,
			Compress:   config.Compress,
		}
		Logger.SetOutput(io.MultiWriter(os.Stdout, lumberjackLogger))
	default:
		Logger.SetOutput(os.Stdout)
	}
}

// GinLogger Gin框架日志中间件
func GinLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		Logger.WithFields(logrus.Fields{
			"status":     param.StatusCode,
			"method":     param.Method,
			"path":       param.Path,
			"ip":         param.ClientIP,
			"user_agent": param.Request.UserAgent(),
			"latency":    param.Latency,
			"time":       param.TimeStamp.Format(time.RFC3339),
		}).Info("HTTP请求")
		return ""
	})
}

// GinRecovery Gin框架恢复中间件
func GinRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		Logger.WithFields(logrus.Fields{
			"error":  recovered,
			"path":   c.Request.URL.Path,
			"method": c.Request.Method,
			"ip":     c.ClientIP(),
		}).Error("Panic恢复")
		c.AbortWithStatus(500)
	})
}

// WithFields 创建带字段的日志条目
func WithFields(fields logrus.Fields) *logrus.Entry {
	return Logger.WithFields(fields)
}

// WithField 创建带单个字段的日志条目
func WithField(key string, value interface{}) *logrus.Entry {
	return Logger.WithField(key, value)
}

// Debug 调试日志
func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	Logger.Debugf(format, args...)
}

// Info 信息日志
func Info(args ...interface{}) {
	Logger.Info(args...)
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	Logger.Infof(format, args...)
}

// Warn 警告日志
func Warn(args ...interface{}) {
	Logger.Warn(args...)
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	Logger.Warnf(format, args...)
}

// Error 错误日志
func Error(args ...interface{}) {
	Logger.Error(args...)
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	Logger.Errorf(format, args...)
}

// Fatal 致命错误日志
func Fatal(args ...interface{}) {
	Logger.Fatal(args...)
}

// Fatalf 格式化致命错误日志
func Fatalf(format string, args ...interface{}) {
	Logger.Fatalf(format, args...)
}
