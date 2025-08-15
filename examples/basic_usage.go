package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/majiayu000/anywhere-ai/core/database"
	"github.com/majiayu000/anywhere-ai/core/output"
	"github.com/majiayu000/anywhere-ai/core/tmux"
	"github.com/majiayu000/anywhere-ai/core/tools"
)

func main() {
	// Initialize components
	tmuxManager := tmux.NewManager()
	sessionManager := tools.NewSessionManager(tmuxManager)
	outputProcessor := output.NewOutputProcessor()
	
	// Initialize SQLite database
	db, err := database.NewSQLiteDB("anywhere.db")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()
	
	// Create a Claude session
	ctx := context.Background()
	session, err := sessionManager.CreateSession(ctx, tools.ToolClaude, "claude-demo")
	if err != nil {
		log.Fatal("Failed to create session:", err)
	}
	
	fmt.Printf("Created session: %s\n", session.ID)
	
	// Save session to database
	dbSession := &database.Session{
		ID:           session.ID,
		Tool:         string(session.Tool),
		DeviceID:     getDeviceID(),
		DeviceName:   getDeviceName(),
		Status:       string(session.State),
		CreatedAt:    session.StartedAt,
		LastActivity: session.LastActivity,
	}
	
	if err := db.SaveSession(dbSession); err != nil {
		log.Printf("Failed to save session: %v", err)
	}
	
	// Monitor session output
	go func() {
		err := sessionManager.MonitorSession(ctx, session.ID, func(s *tools.ToolSession, output string) {
			// Process output
			outputProcessor.ProcessOutput(output)
			
			// Check for permission requests
			if permission := outputProcessor.GetLastPermission(); permission != nil {
				fmt.Printf("\n⚠️  Permission Request: %s\n", permission.Description)
				fmt.Printf("Options: %v\n", permission.Options)
				// In a real application, you would handle user input here
			}
			
			// Update database
			dbSession.LastActivity = time.Now()
			dbSession.Status = string(s.State)
			db.SaveSession(dbSession)
		})
		if err != nil {
			log.Printf("Monitor error: %v", err)
		}
	}()
	
	// Example: Send a command
	time.Sleep(3 * time.Second)
	fmt.Println("\nSending command to Claude...")
	if err := sessionManager.SendInput(ctx, session.ID, "Hello Claude, can you help me write a Python function?"); err != nil {
		log.Printf("Failed to send input: %v", err)
	}
	
	// List all sessions
	sessions, err := db.ListSessions("", "")
	if err != nil {
		log.Printf("Failed to list sessions: %v", err)
	}
	
	fmt.Printf("\nActive sessions:\n")
	for _, s := range sessions {
		fmt.Printf("- %s (%s) on %s\n", s.ID, s.Tool, s.DeviceName)
	}
	
	// Keep running for demo
	select {}
}

func getDeviceID() string {
	hostname, _ := os.Hostname()
	return hostname
}

func getDeviceName() string {
	if hostname, err := os.Hostname(); err == nil {
		return hostname
	}
	return "unknown"
}