package database

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// InitDatabase initializes the database connection
func InitDatabase() error {
	config := getDatabaseConfig()
	
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		config.Host, config.User, config.Password, config.DBName, config.Port, config.SSLMode)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings (similar to Omnara's settings)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(35)
	sqlDB.SetConnMaxLifetime(3600) // 1 hour

	log.Println("Database connected successfully")
	return nil
}

// AutoMigrate runs auto migration for all models
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	err := DB.AutoMigrate(
		&User{},
		&UserAgent{},
		&AgentInstance{},
		&Message{},
		&APIKey{},
		&PushToken{},
		&TerminalSession{},
		&SessionCheckpoint{},
		&SessionLog{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database migrated successfully")
	return nil
}

// getDatabaseConfig gets database configuration from environment
func getDatabaseConfig() DatabaseConfig {
	// First try to get full DATABASE_URL (like Omnara)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// Parse DATABASE_URL format: postgresql://user:password@host:port/dbname
		// For simplicity, we'll use it directly with GORM
		// In production, you'd want to parse this properly
	}

	// Fallback to individual environment variables
	return DatabaseConfig{
		Host:     getEnvWithDefault("DB_HOST", "localhost"),
		Port:     getEnvWithDefault("DB_PORT", "5432"),
		User:     getEnvWithDefault("DB_USER", "user"),
		Password: getEnvWithDefault("DB_PASSWORD", "password"),
		DBName:   getEnvWithDefault("DB_NAME", "anywhere_core"),
		SSLMode:  getEnvWithDefault("DB_SSLMODE", "disable"),
	}
}

// getEnvWithDefault gets environment variable with default value
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// CloseDatabase closes the database connection
func CloseDatabase() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}