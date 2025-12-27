package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// PIDFile manages a PID file for the daemon.
type PIDFile struct {
	path string
}

// NewPIDFile creates a new PID file manager.
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Write creates the PID file with the current process ID.
func (p *PIDFile) Write() error {
	if p.path == "" {
		return nil
	}

	// Create directory if needed
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create PID file directory: %w", err)
	}

	// Write PID
	pid := os.Getpid()
	content := []byte(strconv.Itoa(pid))

	if err := os.WriteFile(p.path, content, 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// Remove deletes the PID file.
func (p *PIDFile) Remove() error {
	if p.path == "" {
		return nil
	}

	if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	return nil
}

// Read returns the PID from the file, or 0 if not found.
func (p *PIDFile) Read() (int, error) {
	if p.path == "" {
		return 0, nil
	}

	content, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(content))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

// IsRunning checks if the process in the PID file is still running.
func (p *PIDFile) IsRunning() (bool, int) {
	pid, err := p.Read()
	if err != nil || pid == 0 {
		return false, 0
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, pid
	}

	// On Unix, FindProcess always succeeds, so we send signal 0 to check
	err = process.Signal(os.Signal(nil))
	if err != nil {
		return false, pid
	}

	return true, pid
}
