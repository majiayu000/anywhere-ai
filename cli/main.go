package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/majiayu000/anywhere-ai/core/database"
	"github.com/majiayu000/anywhere-ai/core/output"
	"github.com/majiayu000/anywhere-ai/core/tmux"
	"github.com/majiayu000/anywhere-ai/core/tools"
)

func main() {
	// 命令行参数
	var (
		tool       = flag.String("tool", "claude", "AI tool to use (claude/gemini/cursor)")
		sessionID  = flag.String("session", "", "Session ID to attach/restore")
		listOnly   = flag.Bool("list", false, "List all sessions")
		dbPath     = flag.String("db", "anywhere.db", "Database path")
	)
	flag.Parse()

	// 初始化数据库
	db, err := database.NewSQLiteDB(*dbPath)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// 列出会话
	if *listOnly {
		listSessions(db)
		return
	}

	// 初始化管理器
	tmuxManager := tmux.NewManager()
	sessionManager := tools.NewSessionManager(tmuxManager)
	outputProcessor := output.NewOutputProcessor()
	ctx := context.Background()

	var session *tools.ToolSession

	// 恢复或创建会话
	if *sessionID != "" {
		// 尝试恢复会话
		dbSession, err := db.GetSession(*sessionID)
		if err != nil {
			log.Fatal("Session not found:", err)
		}

		fmt.Printf("📱 Restoring session: %s\n", dbSession.ID)
		
		// 恢复tmux会话
		tmuxSession := &tmux.Session{
			ID:   dbSession.ID,
			Name: dbSession.ID,
			Tool: dbSession.Tool,
		}
		
		if err := tmuxManager.RestoreSession(ctx, tmuxSession); err != nil {
			log.Fatal("Failed to restore session:", err)
		}

		// 创建工具会话
		session = &tools.ToolSession{
			ID:          dbSession.ID,
			Tool:        tools.ToolType(dbSession.Tool),
			TmuxSession: tmuxSession,
			State:       tools.SessionState(dbSession.Status),
		}
	} else {
		// 创建新会话
		sessionName := fmt.Sprintf("%s-%d", *tool, time.Now().Unix())
		
		fmt.Printf("🚀 Creating new %s session: %s\n", *tool, sessionName)
		
		session, err = sessionManager.CreateSession(ctx, tools.ToolType(*tool), sessionName)
		if err != nil {
			log.Fatal("Failed to create session:", err)
		}

		// 保存到数据库
		dbSession := &database.Session{
			ID:           session.ID,
			Tool:         *tool,
			DeviceID:     getDeviceID(),
			DeviceName:   getDeviceName(),
			Status:       string(session.State),
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
		}
		
		if err := db.SaveSession(dbSession); err != nil {
			log.Printf("Failed to save session: %v", err)
		}
	}

	fmt.Printf("\n✅ Session ready: %s\n", session.ID)
	fmt.Println("Commands:")
	fmt.Println("  Type your message and press Enter to send")
	fmt.Println("  'exit' - Exit the program (session keeps running)")
	fmt.Println("  'kill' - Kill the session and exit")
	fmt.Println("  'status' - Show session status")
	fmt.Println("  'clear' - Clear the screen")
	fmt.Println("")

	// 监控输出
	outputChan := make(chan string, 100)
	go func() {
		err := sessionManager.MonitorSession(ctx, session.ID, func(s *tools.ToolSession, output string) {
			outputProcessor.ProcessOutput(output)
			outputChan <- output
			
			// 检查权限请求
			if permission := outputProcessor.GetLastPermission(); permission != nil {
				fmt.Printf("\n⚠️  Permission Request: %s\n", permission.Description)
				fmt.Printf("Options: %v\n", permission.Options)
				fmt.Print("Response: ")
			}
		})
		if err != nil {
			log.Printf("Monitor error: %v", err)
		}
	}()

	// 显示输出
	go func() {
		for output := range outputChan {
			// 清理输出，只显示最新的内容变化
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					fmt.Println(line)
				}
			}
		}
	}()

	// 等待工具启动
	time.Sleep(2 * time.Second)

	// 交互式输入
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	
	for scanner.Scan() {
		input := scanner.Text()
		
		switch strings.ToLower(input) {
		case "exit":
			fmt.Println("👋 Exiting... (session keeps running)")
			fmt.Printf("To reattach: go run main.go -session %s\n", session.ID)
			return
			
		case "kill":
			fmt.Println("🔥 Killing session...")
			sessionManager.StopSession(ctx, session.ID)
			db.DeleteSession(session.ID)
			return
			
		case "status":
			fmt.Printf("📊 Session: %s\n", session.ID)
			fmt.Printf("   Tool: %s\n", session.Tool)
			fmt.Printf("   State: %s\n", session.State)
			
		case "clear":
			fmt.Print("\033[H\033[2J")
			
		default:
			// 发送到AI工具
			if err := sessionManager.SendInput(ctx, session.ID, input); err != nil {
				log.Printf("Failed to send input: %v", err)
			}
		}
		
		// 短暂等待输出
		time.Sleep(100 * time.Millisecond)
		fmt.Print("> ")
	}
}

func listSessions(db *database.SQLiteDB) {
	sessions, err := db.ListSessions("", "")
	if err != nil {
		log.Fatal("Failed to list sessions:", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active sessions")
		return
	}

	fmt.Println("\n📋 Active Sessions:")
	fmt.Println("─────────────────────────────────────────────")
	for _, s := range sessions {
		fmt.Printf("ID: %s\n", s.ID)
		fmt.Printf("  Tool: %s | Device: %s\n", s.Tool, s.DeviceName)
		fmt.Printf("  Status: %s | Last Active: %s\n", s.Status, s.LastActivity.Format("2006-01-02 15:04:05"))
		fmt.Println("─────────────────────────────────────────────")
	}
	fmt.Println("\nTo attach: go run main.go -session <ID>")
}

func getDeviceID() string {
	hostname, _ := os.Hostname()
	return hostname
}

func getDeviceName() string {
	hostname, _ := os.Hostname()
	if hostname != "" {
		return hostname
	}
	return "unknown"
}