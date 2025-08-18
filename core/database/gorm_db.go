package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitGormDB initializes GORM with SQLite
func InitGormDB() (*gorm.DB, error) {
	// Create data directory if it doesn't exist
	dataDir := "./data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "anywhere.db")
	
	// Configure GORM
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	
	// Enable logging in development
	if os.Getenv("DEBUG") == "true" {
		config.Logger = logger.Default.LogMode(logger.Info)
	}

	// Open database connection
	db, err := gorm.Open(sqlite.Open(dbPath), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate schemas
	if err := migrateSchemas(db); err != nil {
		return nil, fmt.Errorf("failed to migrate schemas: %w", err)
	}

	log.Printf("✅ Database initialized at %s", dbPath)
	return db, nil
}

// migrateSchemas runs auto-migration for all models
func migrateSchemas(db *gorm.DB) error {
	models := []interface{}{
		&User{},
		&AgentInstance{},
		&TerminalSession{},
		&Message{},              // Original Message model from Omnara
		&TerminalMessage{},      // Terminal-specific message model
		&MessageSession{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	return nil
}