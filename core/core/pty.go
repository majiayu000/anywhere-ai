package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// PTYManager manages pseudo-terminal for CLI tool interaction
type PTYManager struct {
	cmd      *exec.Cmd
	ptmx     *os.File
	ctx      context.Context
	cancel   context.CancelFunc
	
	readChan chan []byte
	doneChan chan struct{}
	
	mu       sync.RWMutex
	running  bool
	pid      int
	
	// Statistics
	bytesRead    int64
	bytesWritten int64
}

// NewPTYManager creates a new PTY manager
func NewPTYManager() *PTYManager {
	return &PTYManager{
		readChan: make(chan []byte, 100),
		doneChan: make(chan struct{}),
	}
}

// Start starts the process with PTY
func (p *PTYManager) Start(ctx context.Context, command *Command) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.running {
		return fmt.Errorf("process already running")
	}
	
	// Create context with cancel
	p.ctx, p.cancel = context.WithCancel(ctx)
	
	// Build command
	p.cmd = exec.CommandContext(p.ctx, command.Path, command.Args...)
	
	// Set environment
	if len(command.Env) > 0 {
		p.cmd.Env = append(os.Environ(), command.Env...)
	} else {
		p.cmd.Env = os.Environ()
	}
	
	// Set working directory
	if command.Dir != "" {
		p.cmd.Dir = command.Dir
	}
	
	// Start with PTY
	ptmx, err := pty.Start(p.cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}
	p.ptmx = ptmx
	
	// Get process ID
	if p.cmd.Process != nil {
		p.pid = p.cmd.Process.Pid
	}
	
	// Set initial terminal size
	if err := p.setTerminalSize(); err != nil {
		// Log error but don't fail - default size will be used
		fmt.Fprintf(os.Stderr, "Warning: failed to set terminal size: %v\n", err)
	}
	
	// Handle terminal resize
	go p.handleTerminalResize()
	
	// Start reading output
	go p.readLoop()
	
	// Monitor process exit
	go p.waitForExit()
	
	p.running = true
	
	return nil
}

// Stop stops the process
func (p *PTYManager) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.running {
		return nil
	}
	
	// Cancel context
	if p.cancel != nil {
		p.cancel()
	}
	
	// Close PTY
	if p.ptmx != nil {
		p.ptmx.Close()
	}
	
	// Send termination signal
	if p.cmd != nil && p.cmd.Process != nil {
		// Try graceful termination first
		p.cmd.Process.Signal(syscall.SIGTERM)
		
		// TODO: Add timeout and force kill if needed
	}
	
	// Wait for done signal
	<-p.doneChan
	
	p.running = false
	
	return nil
}

// Write writes data to the process stdin
func (p *PTYManager) Write(data []byte) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if !p.running || p.ptmx == nil {
		return 0, fmt.Errorf("process not running")
	}
	
	n, err := p.ptmx.Write(data)
	if err == nil {
		p.bytesWritten += int64(n)
	}
	
	return n, err
}

// Read returns the channel for reading output
func (p *PTYManager) Read() <-chan []byte {
	return p.readChan
}

// GetPID returns the process ID
func (p *PTYManager) GetPID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pid
}

// IsRunning checks if the process is running
func (p *PTYManager) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// GetStats returns statistics about the PTY session
func (p *PTYManager) GetStats() (bytesRead, bytesWritten int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.bytesRead, p.bytesWritten
}

// readLoop continuously reads from the PTY
func (p *PTYManager) readLoop() {
	defer close(p.readChan)
	
	buf := make([]byte, 4096)
	for {
		n, err := p.ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Log error if not EOF
				fmt.Fprintf(os.Stderr, "PTY read error: %v\n", err)
			}
			return
		}
		
		if n > 0 {
			// Make a copy of the data
			data := make([]byte, n)
			copy(data, buf[:n])
			
			// Update statistics
			p.mu.Lock()
			p.bytesRead += int64(n)
			p.mu.Unlock()
			
			// Send to channel
			select {
			case p.readChan <- data:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// waitForExit waits for the process to exit
func (p *PTYManager) waitForExit() {
	defer close(p.doneChan)
	
	if p.cmd != nil {
		p.cmd.Wait()
	}
	
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
}

// setTerminalSize sets the PTY size to match the current terminal
func (p *PTYManager) setTerminalSize() error {
	// Try to get current terminal size
	ws, err := pty.GetsizeFull(os.Stdin)
	if err != nil {
		// Use default size if we can't get the current terminal size
		ws = &pty.Winsize{
			Rows: 40,
			Cols: 120,
			X:    0,
			Y:    0,
		}
	}
	
	return pty.Setsize(p.ptmx, ws)
}

// handleTerminalResize handles terminal resize events
func (p *PTYManager) handleTerminalResize() {
	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	
	// TODO: Implement signal-based resize handling
	// This would typically listen for SIGWINCH signals
	// and update the PTY size accordingly
}