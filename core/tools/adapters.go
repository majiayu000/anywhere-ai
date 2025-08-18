package tools

import (
	"regexp"
	"strings"
	"time"
)

// ClaudeAdapter implements ToolAdapter for Claude Code
type ClaudeAdapter struct {
	lastEscInterrupt   time.Time
	terminalBuffer     string
	isProcessing       bool
	lastUserInput      string
	lastClaudeResponse string
}

// NewClaudeAdapter creates a new Claude adapter
func NewClaudeAdapter() *ClaudeAdapter {
	return &ClaudeAdapter{
		terminalBuffer: "",
	}
}

func (a *ClaudeAdapter) GetCommand() []string {
	return []string{"claude"}
}

func (a *ClaudeAdapter) ParseOutput(output string) SessionState {
	// Update terminal buffer with new output
	a.terminalBuffer = output
	
	// Remove ANSI escape codes for cleaner parsing
	cleanOutput := a.stripANSI(output)
	
	// Check for various Claude states
	if a.isClaudeProcessing(cleanOutput) {
		a.isProcessing = true
		return StateProcessing
	}
	
	if a.isClaudeWaitingForInput(cleanOutput) {
		a.isProcessing = false
		return StateWaitingInput
	}
	
	if a.isClaudeError(cleanOutput) {
		return StateError
	}
	
	if a.isClaudeReady(cleanOutput) {
		a.isProcessing = false
		return StateReady
	}
	
	return StateStarting
}

func (a *ClaudeAdapter) IsPermissionPrompt(output string) bool {
	cleanOutput := a.stripANSI(output)
	
	// Check for permission prompts
	permissionPatterns := []string{
		"Do you want",
		"Would you like to proceed",
		"Allow Claude to",
		"Permission to",
		"Authorize",
	}
	
	// Check for option patterns that indicate a prompt
	optionPatterns := []string{
		"1. Yes",
		"2. No",
		"1. Yes, and auto-accept",
		"2. Yes, and manually approve",
		"3. No",
	}
	
	hasQuestion := false
	for _, pattern := range permissionPatterns {
		if strings.Contains(cleanOutput, pattern) {
			hasQuestion = true
			break
		}
	}
	
	hasOptions := false
	for _, pattern := range optionPatterns {
		if strings.Contains(cleanOutput, pattern) {
			hasOptions = true
			break
		}
	}
	
	return hasQuestion && hasOptions
}

// Helper methods for enhanced Claude detection

func (a *ClaudeAdapter) isClaudeProcessing(output string) bool {
	// Check for processing indicators
	processingPatterns := []string{
		"esc to interrupt",
		"ctrl+b to run in background",
		"Processing",
		"Thinking",
		"Analyzing",
		"Working on",
	}
	
	for _, pattern := range processingPatterns {
		if strings.Contains(output, pattern) {
			a.lastEscInterrupt = time.Now()
			return true
		}
	}
	
	return false
}

func (a *ClaudeAdapter) isClaudeWaitingForInput(output string) bool {
	// Check if Claude is idle (no "esc to interrupt" for a while)
	if time.Since(a.lastEscInterrupt) < 750*time.Millisecond {
		return false
	}
	
	// Check for input prompts
	inputPatterns := []string{
		"What would you like to",
		"How can I help",
		"What can I do for you",
		"Please provide",
		"Enter your",
		">",
		"$",
	}
	
	// Check the last few lines for prompts
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		lastFewLines := strings.Join(lines[max(0, len(lines)-5):], " ")
		for _, pattern := range inputPatterns {
			if strings.Contains(lastFewLines, pattern) {
				return true
			}
		}
	}
	
	return false
}

func (a *ClaudeAdapter) isClaudeError(output string) bool {
	errorPatterns := []string{
		"Error:",
		"Failed:",
		"Exception:",
		"command not found",
		"permission denied",
		"fatal:",
	}
	
	for _, pattern := range errorPatterns {
		if strings.Contains(strings.ToLower(output), strings.ToLower(pattern)) {
			return true
		}
	}
	
	return false
}

func (a *ClaudeAdapter) isClaudeReady(output string) bool {
	// Claude is ready if it's not processing and shows a prompt
	if a.isProcessing {
		return false
	}
	
	// Check for ready indicators
	readyPatterns := []string{
		"Ready",
		"Started",
		"Initialized",
		"Claude Code session started",
		"How can I help you today",
	}
	
	for _, pattern := range readyPatterns {
		if strings.Contains(output, pattern) {
			return true
		}
	}
	
	// Also check if enough time has passed since last activity
	return time.Since(a.lastEscInterrupt) > 2*time.Second
}

// stripANSI removes ANSI escape codes from text
func (a *ClaudeAdapter) stripANSI(text string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiRegex.ReplaceAllString(text, "")
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *ClaudeAdapter) FormatInput(input string) string {
	return input
}

func (a *ClaudeAdapter) GetInitCommands() []string {
	return []string{}
}

// GeminiAdapter implements ToolAdapter for Gemini CLI
type GeminiAdapter struct{}

func (a *GeminiAdapter) GetCommand() []string {
	return []string{"gemini"}
}

func (a *GeminiAdapter) ParseOutput(output string) SessionState {
	lines := strings.Split(output, "\n")
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastLine = strings.ToLower(lines[i])
			break
		}
	}
	
	switch {
	case strings.Contains(lastLine, "gemini>"):
		return StateReady
	case strings.Contains(lastLine, ">"):
		return StateReady
	case strings.Contains(lastLine, "?"):
		return StateWaitingInput
	case strings.Contains(lastLine, "error"):
		return StateError
	default:
		return StateProcessing
	}
}

func (a *GeminiAdapter) IsPermissionPrompt(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "confirm") || 
		   (strings.Contains(lower, "?") && strings.Contains(lower, "continue"))
}

func (a *GeminiAdapter) FormatInput(input string) string {
	return input
}

func (a *GeminiAdapter) GetInitCommands() []string {
	return []string{}
}

// CursorAdapter implements ToolAdapter for Cursor
type CursorAdapter struct{}

func (a *CursorAdapter) GetCommand() []string {
	// Cursor typically runs as an IDE, so we might need to use its CLI
	return []string{"cursor", "--cli"}
}

func (a *CursorAdapter) ParseOutput(output string) SessionState {
	lines := strings.Split(output, "\n")
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastLine = strings.ToLower(lines[i])
			break
		}
	}
	
	switch {
	case strings.Contains(lastLine, "cursor>"):
		return StateReady
	case strings.Contains(lastLine, "ready"):
		return StateReady
	case strings.Contains(lastLine, "?"):
		return StateWaitingInput
	case strings.Contains(lastLine, "error"):
		return StateError
	case strings.Contains(lastLine, "loading"):
		return StateStarting
	default:
		return StateProcessing
	}
}

func (a *CursorAdapter) IsPermissionPrompt(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "authorize") || 
		   strings.Contains(lower, "permission")
}

func (a *CursorAdapter) FormatInput(input string) string {
	return input
}

func (a *CursorAdapter) GetInitCommands() []string {
	// Initialize Cursor CLI if needed
	return []string{}
}

// CopilotAdapter implements ToolAdapter for GitHub Copilot
type CopilotAdapter struct{}

func (a *CopilotAdapter) GetCommand() []string {
	return []string{"gh", "copilot", "cli"}
}

func (a *CopilotAdapter) ParseOutput(output string) SessionState {
	lines := strings.Split(output, "\n")
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastLine = strings.ToLower(lines[i])
			break
		}
	}
	
	switch {
	case strings.Contains(lastLine, "$"):
		return StateReady
	case strings.Contains(lastLine, "#"):
		return StateReady
	case strings.Contains(lastLine, "?"):
		return StateWaitingInput
	case strings.Contains(lastLine, "error"):
		return StateError
	case strings.Contains(lastLine, "authenticating"):
		return StateStarting
	default:
		return StateProcessing
	}
}

func (a *CopilotAdapter) IsPermissionPrompt(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "authenticate") || 
		   strings.Contains(lower, "login") ||
		   strings.Contains(lower, "authorize")
}

func (a *CopilotAdapter) FormatInput(input string) string {
	return input
}

func (a *CopilotAdapter) GetInitCommands() []string {
	return []string{}
}