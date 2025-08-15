package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 数据库配置
type Config struct {
	Driver          string `mapstructure:"driver" yaml:"driver"`
	Host            string `mapstructure:"host" yaml:"host"`
	Port            int    `mapstructure:"port" yaml:"port"`
	Username        string `mapstructure:"username" yaml:"username"`
	Password        string `mapstructure:"password" yaml:"password"`
	Database        string `mapstructure:"database" yaml:"database"`
	Charset         string `mapstructure:"charset" yaml:"charset"`
	Timezone        string `mapstructure:"timezone" yaml:"timezone"`
	SSLMode         string `mapstructure:"ssl_mode" yaml:"ssl_mode"`
	MaxOpenConns    int    `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	LogLevel        string `mapstructure:"log_level" yaml:"log_level"`
}

// DefaultConfig 默认数据库配置
func DefaultConfig() Config {
	return Config{
		Driver:          "sqlite",
		Host:            "localhost",
		Port:            3306,
		Charset:         "utf8mb4",
		Timezone:        "Asia/Shanghai",
		SSLMode:         "disable",
		MaxOpenConns:    100,
		MaxIdleConns:    10,
		ConnMaxLifetime: 3600, // 1小时
		LogLevel:        "info",
	}
}

// Database 数据库实例
type Database struct {
	DB     *gorm.DB
	Config Config
}

// New 创建数据库连接
func New(config Config) (*Database, error) {
	db := &Database{
		Config: config,
	}

	if err := db.connect(); err != nil {
		return nil, err
	}

	return db, nil
}

// connect 连接数据库
func (db *Database) connect() error {
	var dialector gorm.Dialector

	switch db.Config.Driver {
	case "mysql":
		dsn := db.buildMySQLDSN()
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dsn := db.buildPostgresDSN()
		dialector = postgres.Open(dsn)
	case "sqlite":
		dsn := db.buildSQLiteDSN()
		dialector = sqlite.Open(dsn)
	default:
		return fmt.Errorf("不支持的数据库驱动: %s", db.Config.Driver)
	}

	// 配置GORM
	gormConfig := &gorm.Config{
		Logger: db.getLogger(),
	}

	var err error
	db.DB, err = gorm.Open(dialector, gormConfig)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 配置连接池
	if err := db.configureConnectionPool(); err != nil {
		return fmt.Errorf("配置连接池失败: %w", err)
	}

	return nil
}

// buildMySQLDSN 构建MySQL DSN
func (db *Database) buildMySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=%s",
		db.Config.Username,
		db.Config.Password,
		db.Config.Host,
		db.Config.Port,
		db.Config.Database,
		db.Config.Charset,
		db.Config.Timezone,
	)
}

// buildPostgresDSN 构建PostgreSQL DSN
func (db *Database) buildPostgresDSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		db.Config.Host,
		db.Config.Username,
		db.Config.Password,
		db.Config.Database,
		db.Config.Port,
		db.Config.SSLMode,
		db.Config.Timezone,
	)
}

// buildSQLiteDSN 构建SQLite DSN
func (db *Database) buildSQLiteDSN() string {
	if db.Config.Database == "" {
		return "data/app.db"
	}
	return db.Config.Database
}

// getLogger 获取日志器
func (db *Database) getLogger() logger.Interface {
	var logLevel logger.LogLevel

	switch db.Config.LogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	default:
		logLevel = logger.Info
	}

	return logger.Default.LogMode(logLevel)
}

// configureConnectionPool 配置连接池
func (db *Database) configureConnectionPool() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}

	// 设置最大打开连接数
	sqlDB.SetMaxOpenConns(db.Config.MaxOpenConns)

	// 设置最大空闲连接数
	sqlDB.SetMaxIdleConns(db.Config.MaxIdleConns)

	// 设置连接最大生存时间
	sqlDB.SetConnMaxLifetime(time.Duration(db.Config.ConnMaxLifetime) * time.Second)

	return nil
}

// Ping 测试数据库连接
func (db *Database) Ping() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Close 关闭数据库连接
func (db *Database) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Stats 获取连接池统计信息
func (db *Database) Stats() sql.DBStats {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return sql.DBStats{}
	}
	return sqlDB.Stats()
}

// AutoMigrate 自动迁移
func (db *Database) AutoMigrate(models ...interface{}) error {
	return db.DB.AutoMigrate(models...)
}

// Transaction 执行事务
func (db *Database) Transaction(fn func(*gorm.DB) error) error {
	return db.DB.Transaction(fn)
}

// WithContext 使用上下文
func (db *Database) WithContext(ctx context.Context) *gorm.DB {
	return db.DB.WithContext(ctx)
}

// HealthCheck 数据库健康检查
type HealthCheck struct {
	DB *Database
}

func (hc *HealthCheck) Name() string {
	return "database"
}

func (hc *HealthCheck) Check(ctx context.Context) HealthCheckResult {
	if hc.DB == nil {
		return HealthCheckResult{
			Status:  "unhealthy",
			Message: "数据库实例未初始化",
		}
	}

	if err := hc.DB.Ping(); err != nil {
		return HealthCheckResult{
			Status:  "unhealthy",
			Message: fmt.Sprintf("数据库连接失败: %v", err),
		}
	}

	stats := hc.DB.Stats()
	return HealthCheckResult{
		Status:  "healthy",
		Message: "数据库连接正常",
		Details: map[string]interface{}{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"wait_count":       stats.WaitCount,
			"wait_duration":    stats.WaitDuration.String(),
		},
	}
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Details interface{} `json:"details,omitempty"`
}
