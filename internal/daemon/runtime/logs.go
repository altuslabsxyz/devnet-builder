// internal/daemon/runtime/logs.go
package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// LogConfig configures log management
type LogConfig struct {
	MaxSize  int64 // max file size before rotation (bytes)
	MaxFiles int   // max number of rotated files to keep
}

// DefaultLogConfig returns default log configuration
func DefaultLogConfig() LogConfig {
	return LogConfig{
		MaxSize:  100 * 1024 * 1024, // 100MB
		MaxFiles: 5,
	}
}

// LogManager manages log files with rotation
type LogManager struct {
	baseDir string
	config  LogConfig
	writers map[string]*rotatingWriter
	mu      sync.Mutex
}

// NewLogManager creates a new log manager
func NewLogManager(baseDir string, config LogConfig) *LogManager {
	return &LogManager{
		baseDir: baseDir,
		config:  config,
		writers: make(map[string]*rotatingWriter),
	}
}

// GetWriter returns a writer for a node's logs
func (lm *LogManager) GetWriter(nodeID string, logPath string) (io.WriteCloser, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Create directory if needed
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	// Check if we already have a writer
	if w, ok := lm.writers[nodeID]; ok {
		return w, nil
	}

	// Create new rotating writer
	w, err := newRotatingWriter(logPath, lm.config.MaxSize, lm.config.MaxFiles)
	if err != nil {
		return nil, err
	}

	lm.writers[nodeID] = w
	return w, nil
}

// GetReader returns a reader for a node's logs
func (lm *LogManager) GetReader(logPath string, opts LogOptions) (io.ReadCloser, error) {
	if opts.Lines > 0 {
		return lm.tailFile(logPath, opts.Lines)
	}

	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// tailFile returns the last N lines of a file
func (lm *LogManager) tailFile(logPath string, lines int) (io.ReadCloser, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}

	// Read all lines (simple implementation)
	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	f.Close()

	// Get last N lines
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}
	lastLines := allLines[start:]

	// Build content string
	var content string
	for _, line := range lastLines {
		content += line + "\n"
	}

	// Return as a string reader wrapped in NopCloser
	return io.NopCloser(strings.NewReader(content)), nil
}

// Close closes a writer for a node
func (lm *LogManager) Close(nodeID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if w, ok := lm.writers[nodeID]; ok {
		delete(lm.writers, nodeID)
		return w.Close()
	}
	return nil
}

// rotatingWriter writes to a file with rotation
type rotatingWriter struct {
	path     string
	maxSize  int64
	maxFiles int
	file     *os.File
	size     int64
	mu       sync.Mutex
}

func newRotatingWriter(path string, maxSize int64, maxFiles int) (*rotatingWriter, error) {
	w := &rotatingWriter{
		path:     path,
		maxSize:  maxSize,
		maxFiles: maxFiles,
	}

	if err := w.openFile(); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *rotatingWriter) openFile() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}

	w.file = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we need to rotate
	if w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotate() error {
	// Close current file
	w.file.Close()

	// Rotate existing files
	for i := w.maxFiles - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.path, i)
		newPath := fmt.Sprintf("%s.%d", w.path, i+1)
		os.Rename(oldPath, newPath)
	}

	// Move current file to .1
	os.Rename(w.path, fmt.Sprintf("%s.1", w.path))

	// Delete old files beyond maxFiles
	w.cleanOldFiles()

	// Open new file
	return w.openFile()
}

func (w *rotatingWriter) cleanOldFiles() {
	pattern := w.path + ".*"
	matches, _ := filepath.Glob(pattern)

	if len(matches) <= w.maxFiles {
		return
	}

	// Sort by modification time
	sort.Slice(matches, func(i, j int) bool {
		iInfo, _ := os.Stat(matches[i])
		jInfo, _ := os.Stat(matches[j])
		if iInfo == nil || jInfo == nil {
			return false
		}
		return iInfo.ModTime().After(jInfo.ModTime())
	})

	// Remove old files
	for i := w.maxFiles; i < len(matches); i++ {
		os.Remove(matches[i])
	}
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
