package output

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// OutputProcessor processes and analyzes tool output
type OutputProcessor struct {
	detectors      []PermissionDetector
	patterns       map[string]*regexp.Regexp
	buffer         *OutputBuffer
	lastPermission *PermissionRequest
	mu             sync.RWMutex
}

// OutputBuffer manages output buffering
type OutputBuffer struct {
	lines      []string
	maxLines   int
	lastUpdate time.Time
}

// PermissionRequest represents a detected permission request
type PermissionRequest struct {
	Type        string    // "file_write", "command_execute", "network", etc.
	Description string
	Options     []string  // e.g., ["y", "n", "always", "never"]
	DetectedAt  time.Time
	RawPrompt   string
}

// PermissionDetector interface for tool-specific permission detection
type PermissionDetector interface {
	Detect(output string) *PermissionRequest
	GetPriority() int
}

// NewOutputProcessor creates a new output processor
func NewOutputProcessor() *OutputProcessor {
	p := &OutputProcessor{
		detectors: []PermissionDetector{},
		patterns:  make(map[string]*regexp.Regexp),
		buffer: &OutputBuffer{
			lines:    []string{},
			maxLines: 1000,
		},
	}
	
	// Initialize common patterns
	p.initializePatterns()
	
	// Register default detectors
	p.RegisterDetector(&FilePermissionDetector{})
	p.RegisterDetector(&CommandPermissionDetector{})
	p.RegisterDetector(&NetworkPermissionDetector{})
	
	return p
}

// RegisterDetector registers a permission detector
func (p *OutputProcessor) RegisterDetector(detector PermissionDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Insert by priority (higher priority first)
	inserted := false
	for i, d := range p.detectors {
		if detector.GetPriority() > d.GetPriority() {
			p.detectors = append(p.detectors[:i], append([]PermissionDetector{detector}, p.detectors[i:]...)...)
			inserted = true
			break
		}
	}
	
	if !inserted {
		p.detectors = append(p.detectors, detector)
	}
}

// ProcessOutput processes new output from a tool
func (p *OutputProcessor) ProcessOutput(output string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Add to buffer
	lines := strings.Split(output, "\n")
	p.buffer.lines = append(p.buffer.lines, lines...)
	
	// Trim buffer if too large
	if len(p.buffer.lines) > p.buffer.maxLines {
		p.buffer.lines = p.buffer.lines[len(p.buffer.lines)-p.buffer.maxLines:]
	}
	
	p.buffer.lastUpdate = time.Now()
	
	// Check for permission requests
	for _, detector := range p.detectors {
		if permission := detector.Detect(output); permission != nil {
			p.lastPermission = permission
			break
		}
	}
}

// GetLastPermission returns the last detected permission request
func (p *OutputProcessor) GetLastPermission() *PermissionRequest {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastPermission
}

// ClearPermission clears the last permission request
func (p *OutputProcessor) ClearPermission() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastPermission = nil
}

// GetBuffer returns the current output buffer
func (p *OutputProcessor) GetBuffer() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	result := make([]string, len(p.buffer.lines))
	copy(result, p.buffer.lines)
	return result
}

// ExtractContext extracts context around a pattern match
func (p *OutputProcessor) ExtractContext(pattern string, linesBefore, linesAfter int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	re, exists := p.patterns[pattern]
	if !exists {
		// Try to compile the pattern
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return []string{}
		}
	}
	
	var result []string
	for i, line := range p.buffer.lines {
		if re.MatchString(line) {
			// Extract context
			start := i - linesBefore
			if start < 0 {
				start = 0
			}
			
			end := i + linesAfter + 1
			if end > len(p.buffer.lines) {
				end = len(p.buffer.lines)
			}
			
			result = append(result, p.buffer.lines[start:end]...)
		}
	}
	
	return result
}

// Helper functions

func (p *OutputProcessor) initializePatterns() {
	// Common patterns for different scenarios
	p.patterns["permission"] = regexp.MustCompile(`(?i)(allow|permit|authorize|permission|confirm).*\?`)
	p.patterns["error"] = regexp.MustCompile(`(?i)(error|failed|exception|fatal)`)
	p.patterns["warning"] = regexp.MustCompile(`(?i)(warning|caution|alert)`)
	p.patterns["success"] = regexp.MustCompile(`(?i)(success|completed|done|finished)`)
	p.patterns["prompt"] = regexp.MustCompile(`[>$#]\s*$`)
}

// FilePermissionDetector detects file operation permissions
type FilePermissionDetector struct{}

func (d *FilePermissionDetector) Detect(output string) *PermissionRequest {
	lower := strings.ToLower(output)
	
	filePatterns := []string{
		"write to",
		"create file",
		"modify file",
		"delete file",
		"overwrite",
	}
	
	for _, pattern := range filePatterns {
		if strings.Contains(lower, pattern) && strings.Contains(lower, "?") {
			return &PermissionRequest{
				Type:        "file_write",
				Description: "Tool wants to perform a file operation",
				Options:     []string{"y", "n"},
				DetectedAt:  time.Now(),
				RawPrompt:   output,
			}
		}
	}
	
	return nil
}

func (d *FilePermissionDetector) GetPriority() int {
	return 10
}

// CommandPermissionDetector detects command execution permissions
type CommandPermissionDetector struct{}

func (d *CommandPermissionDetector) Detect(output string) *PermissionRequest {
	lower := strings.ToLower(output)
	
	cmdPatterns := []string{
		"run command",
		"execute",
		"sudo",
		"install",
		"system command",
	}
	
	for _, pattern := range cmdPatterns {
		if strings.Contains(lower, pattern) && strings.Contains(lower, "?") {
			return &PermissionRequest{
				Type:        "command_execute",
				Description: "Tool wants to execute a system command",
				Options:     []string{"y", "n", "always"},
				DetectedAt:  time.Now(),
				RawPrompt:   output,
			}
		}
	}
	
	return nil
}

func (d *CommandPermissionDetector) GetPriority() int {
	return 20
}

// NetworkPermissionDetector detects network operation permissions
type NetworkPermissionDetector struct{}

func (d *NetworkPermissionDetector) Detect(output string) *PermissionRequest {
	lower := strings.ToLower(output)
	
	netPatterns := []string{
		"connect to",
		"download",
		"upload",
		"api call",
		"network request",
		"fetch",
	}
	
	for _, pattern := range netPatterns {
		if strings.Contains(lower, pattern) && strings.Contains(lower, "?") {
			return &PermissionRequest{
				Type:        "network",
				Description: "Tool wants to perform a network operation",
				Options:     []string{"y", "n"},
				DetectedAt:  time.Now(),
				RawPrompt:   output,
			}
		}
	}
	
	return nil
}

func (d *NetworkPermissionDetector) GetPriority() int {
	return 5
}