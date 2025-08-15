package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	
	"github.com/majiayu000/anywhere-ai/core/services"
	"github.com/majiayu000/anywhere-ai/core/tmux"
)

func main() {
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

	// Serve static files (web interface)
	router.Static("/static", "../web")
	router.StaticFile("/", "../web/index.html")

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
			"service":   "anywhere-core",
		})
	})

	// Initialize core managers
	tmuxManager := tmux.NewManager()

	// Initialize services
	wsService := services.NewTerminalWebSocketService(tmuxManager)
	apiService := services.NewTerminalAPIService(tmuxManager, wsService)

	// Register routes
	apiService.RegisterRoutes(router)
	
	// WebSocket endpoint
	router.GET("/api/v1/ws", wsService.HandleWebSocket)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Anywhere Core server starting on port %s", port)
	log.Printf("💻 Terminal API: http://localhost:%s/api/v1/terminal", port)
	log.Printf("🔗 WebSocket: ws://localhost:%s/api/v1/ws", port)
	log.Printf("🌐 Web Interface: Open web/index.html in your browser")
	
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}