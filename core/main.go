package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	
	"github.com/majiayu000/ai-cli-core/database"
	"github.com/majiayu000/ai-cli-core/services"
)

func main() {
	// Initialize database
	if err := database.InitDatabase(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Run migrations
	if err := database.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
			"service":   "anywhere-core",
		})
	})

	// Initialize services
	agentService := services.NewAgentCommunicationService()
	terminalService, err := services.NewTerminalManagerService()
	if err != nil {
		log.Fatalf("Failed to initialize terminal service: %v", err)
	}

	// Register routes
	agentService.RegisterRoutes(router)
	terminalService.RegisterRoutes(router)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Anywhere Core server starting on port %s", port)
	log.Printf("📡 Agent Communication: http://localhost:%s/api/v1/agent", port)
	log.Printf("💻 Terminal Management: http://localhost:%s/api/v1/terminal", port)
	log.Printf("🔗 WebSocket: ws://localhost:%s/api/v1/ws", port)
	
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}