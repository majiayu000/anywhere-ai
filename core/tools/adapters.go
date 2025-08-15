package tools

import (
	"strings"
)

// ClaudeAdapter implements ToolAdapter for Claude Code
type ClaudeAdapter struct{}

func (a *ClaudeAdapter) GetCommand() []string {
	return []string{"claude"}
}

func (a *ClaudeAdapter) ParseOutput(output string) SessionState {
	lines := strings.Split(output, "\n")
	lastLine := ""
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			lastLine = strings.ToLower(lines[i])
			break
		}
	}
	
	// Check for various states
	switch {
	case strings.Contains(lastLine, "claude>"):
		return StateReady
	case strings.Contains(lastLine, "?") && strings.Contains(lastLine, "(y/n)"):
		return StateWaitingInput
	case strings.Contains(lastLine, "permission"):
		return StateWaitingInput
	case strings.Contains(lastLine, "error"):
		return StateError
	case strings.Contains(lastLine, "processing"):
		return StateProcessing
	default:
		return StateProcessing
	}
}

func (a *ClaudeAdapter) IsPermissionPrompt(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "allow") && 
		   (strings.Contains(lower, "?") || strings.Contains(lower, "(y/n)"))
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