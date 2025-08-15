// configs/config.go
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Port           int    `mapstructure:"port"`
		Mode           string `mapstructure:"mode"` // debug, release, test
		ReadTimeout    int    `mapstructure:"read_timeout"`
		WriteTimeout   int    `mapstructure:"write_timeout"`
		MaxHeaderBytes int    `mapstructure:"max_header_bytes"`
	} `mapstructure:"server"`
	Log struct {
		Level      string `mapstructure:"level"`
		Format     string `mapstructure:"format"` // json, text
		Output     string `mapstructure:"output"` // console, file, both
		Filename   string `mapstructure:"filename"`
		MaxSize    int    `mapstructure:"max_size"`
		MaxAge     int    `mapstructure:"max_age"`
		MaxBackups int    `mapstructure:"max_backups"`
		Compress   bool   `mapstructure:"compress"`
	} `mapstructure:"log"`
	Database struct {
		Driver          string `mapstructure:"driver"`
		Host            string `mapstructure:"host"`
		Port            int    `mapstructure:"port"`
		Username        string `mapstructure:"username"`
		Password        string `mapstructure:"password"`
		Database        string `mapstructure:"database"`
		Charset         string `mapstructure:"charset"`
		Timezone        string `mapstructure:"timezone"`
		SSLMode         string `mapstructure:"ssl_mode"`
		MaxOpenConns    int    `mapstructure:"max_open_conns"`
		MaxIdleConns    int    `mapstructure:"max_idle_conns"`
		ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
		LogLevel        string `mapstructure:"log_level"`
	} `mapstructure:"database"`
	Redis struct {
		Addr         string `mapstructure:"addr"`
		Password     string `mapstructure:"password"`
		DB           int    `mapstructure:"db"`
		PoolSize     int    `mapstructure:"pool_size"`
		MinIdleConns int    `mapstructure:"min_idle_conns"`
		DialTimeout  int    `mapstructure:"dial_timeout"`
		ReadTimeout  int    `mapstructure:"read_timeout"`
		WriteTimeout int    `mapstructure:"write_timeout"`
		IdleTimeout  int    `mapstructure:"idle_timeout"`
	} `mapstructure:"redis"`
	JWT struct {
		SecretKey         string `mapstructure:"secret_key"`
		Issuer            string `mapstructure:"issuer"`
		Expiration        string `mapstructure:"expiration"`
		RefreshExpiration string `mapstructure:"refresh_expiration"`
	} `mapstructure:"jwt"`
	CORS struct {
		AllowOrigins     []string `mapstructure:"allow_origins"`
		AllowMethods     []string `mapstructure:"allow_methods"`
		AllowHeaders     []string `mapstructure:"allow_headers"`
		ExposeHeaders    []string `mapstructure:"expose_headers"`
		AllowCredentials bool     `mapstructure:"allow_credentials"`
		MaxAge           int      `mapstructure:"max_age"`
	} `mapstructure:"cors"`
	RateLimit struct {
		Capacity int    `mapstructure:"capacity"`
		Rate     string `mapstructure:"rate"`
		TTL      string `mapstructure:"ttl"`
	} `mapstructure:"rate_limit"`
	Monitor struct {
		EnableMetrics     bool `mapstructure:"enable_metrics"`
		EnableHealthCheck bool `mapstructure:"enable_health_check"`
		MaxMemoryMB       int  `mapstructure:"max_memory_mb"`
		MaxGoroutines     int  `mapstructure:"max_goroutines"`
	} `mapstructure:"monitor"`
	OAuth struct {
		Google struct {
			ClientID     string `mapstructure:"client_id"`
			ClientSecret string `mapstructure:"client_secret"`
			RedirectURL  string `mapstructure:"redirect_url"`
		} `mapstructure:"google"`
		Apple struct {
			ClientID     string `mapstructure:"client_id"`
			ClientSecret string `mapstructure:"client_secret"`
			RedirectURL  string `mapstructure:"redirect_url"`
		} `mapstructure:"apple"`
	} `mapstructure:"oauth"`
}

func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("..")

	// 设置默认值
	// 服务器配置
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_timeout", 60)
	viper.SetDefault("server.write_timeout", 60)
	viper.SetDefault("server.max_header_bytes", 1048576)

	// 日志配置
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")
	viper.SetDefault("log.output", "console")
	viper.SetDefault("log.filename", "logs/app.log")
	viper.SetDefault("log.max_size", 100)
	viper.SetDefault("log.max_age", 30)
	viper.SetDefault("log.max_backups", 10)
	viper.SetDefault("log.compress", true)

	// 数据库配置
	viper.SetDefault("database.driver", "sqlite")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.database", "data/app.db")
	viper.SetDefault("database.charset", "utf8mb4")
	viper.SetDefault("database.timezone", "Asia/Shanghai")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 100)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", 3600)
	viper.SetDefault("database.log_level", "info")

	// Redis配置
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.dial_timeout", 5)
	viper.SetDefault("redis.read_timeout", 3)
	viper.SetDefault("redis.write_timeout", 3)
	viper.SetDefault("redis.idle_timeout", 300)

	// JWT配置
	viper.SetDefault("jwt.secret_key", "your-jwt-secret-key-change-this-in-production")
	viper.SetDefault("jwt.issuer", "gin-starter")
	viper.SetDefault("jwt.expiration", "24h")
	viper.SetDefault("jwt.refresh_expiration", "168h")

	// CORS配置
	viper.SetDefault("cors.allow_origins", []string{"*"})
	viper.SetDefault("cors.allow_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"})
	viper.SetDefault("cors.allow_headers", []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Requested-With"})
	viper.SetDefault("cors.expose_headers", []string{"Content-Length"})
	viper.SetDefault("cors.allow_credentials", false)
	viper.SetDefault("cors.max_age", 43200)

	// 限流配置
	viper.SetDefault("rate_limit.capacity", 100)
	viper.SetDefault("rate_limit.rate", "60s")
	viper.SetDefault("rate_limit.ttl", "3600s")

	// 监控配置
	viper.SetDefault("monitor.enable_metrics", true)
	viper.SetDefault("monitor.enable_health_check", true)
	viper.SetDefault("monitor.max_memory_mb", 512)
	viper.SetDefault("monitor.max_goroutines", 1000)

	viper.AutomaticEnv()
	viper.SetEnvPrefix("APP")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 从环境变量覆盖特定值
	if envClientID := viper.GetString("GOOGLE_CLIENT_ID"); envClientID != "" {
		config.OAuth.Google.ClientID = envClientID
	}
	if envClientSecret := viper.GetString("GOOGLE_CLIENT_SECRET"); envClientSecret != "" {
		config.OAuth.Google.ClientSecret = envClientSecret
	}
	if envJWTSecret := viper.GetString("JWT_SECRET_KEY"); envJWTSecret != "" {
		config.JWT.SecretKey = envJWTSecret
	}
	if envDBPassword := viper.GetString("DATABASE_PASSWORD"); envDBPassword != "" {
		config.Database.Password = envDBPassword
	}
	if envRedisPassword := viper.GetString("REDIS_PASSWORD"); envRedisPassword != "" {
		config.Redis.Password = envRedisPassword
	}

	return &config, nil
}
