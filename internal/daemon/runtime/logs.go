// internal/daemon/runtime/logs.go
package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nxadm/tail"
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

// GetReader returns a reader for a node's logs.
// If opts.Follow is true, it uses nxadm/tail to follow the file for new content.
func (lm *LogManager) GetReader(ctx context.Context, logPath string, opts LogOptions) (io.ReadCloser, error) {
	// If follow mode, use tail package
	if opts.Follow {
		return lm.followFile(ctx, logPath, opts.Lines)
	}

	// Non-follow mode: return static content
	if opts.Lines > 0 {
		return lm.tailFile(logPath, opts.Lines)
	}

	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// followFile uses nxadm/tail to follow a log file for new content.
func (lm *LogManager) followFile(ctx context.Context, logPath string, lines int) (io.ReadCloser, error) {
	// If lines > 0, we need to first read the last N lines, then follow from end.
	// The tail package doesn't support "last N lines" directly - Location is for byte offset.
	var initialContent io.ReadCloser
	if lines > 0 {
		// Check if file has been recently modified (within last 2 seconds).
		// If not, the content is likely stale (e.g., from before a node restart),
		// so we skip showing initial content and just follow for new logs.
		const freshnessThreshold = 2 * time.Second
		showInitialContent := true
		if info, err := os.Stat(logPath); err == nil {
			if time.Since(info.ModTime()) > freshnessThreshold {
				showInitialContent = false
			}
		}

		if showInitialContent {
			// Read the last N lines first
			var err error
			initialContent, err = lm.tailFile(logPath, lines)
			if err != nil {
				return nil, fmt.Errorf("failed to read initial lines: %w", err)
			}
		}
	}

	// Always start following from the end of the file
	cfg := tail.Config{
		Follow:    true,
		ReOpen:    true, // Handle log rotation
		Poll:      true, // Use polling instead of inotify - more reliable for detecting appends after process restart
		MustExist: true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd},
		Logger:    log.New(io.Discard, "", 0), // Suppress tail's internal logging
	}

	t, err := tail.TailFile(logPath, cfg)
	if err != nil {
		if initialContent != nil {
			initialContent.Close()
		}
		return nil, fmt.Errorf("failed to tail file: %w", err)
	}

	return newTailReader(ctx, t, initialContent), nil
}

// tailReader wraps tail.Tail to implement io.ReadCloser.
// It optionally reads from initialContent first, then follows with tail.
type tailReader struct {
	ctx            context.Context
	t              *tail.Tail
	buf            []byte
	initialContent io.ReadCloser // Optional: content to read before following
	lineCh         <-chan *tail.Line
	done           chan struct{}
	closed         bool
	mu             sync.Mutex
}

func newTailReader(ctx context.Context, t *tail.Tail, initialContent io.ReadCloser) *tailReader {
	return &tailReader{
		ctx:            ctx,
		t:              t,
		initialContent: initialContent,
		lineCh:         t.Lines,
		done:           make(chan struct{}),
	}
}

func (tr *tailReader) Read(p []byte) (int, error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.closed {
		return 0, io.EOF
	}

	// If we have buffered data, return it first
	if len(tr.buf) > 0 {
		n := copy(p, tr.buf)
		tr.buf = tr.buf[n:]
		return n, nil
	}

	// If we have initial content (last N lines), read from it first
	if tr.initialContent != nil {
		n, err := tr.initialContent.Read(p)
		if err == io.EOF {
			// Done with initial content, close it and switch to following
			tr.initialContent.Close()
			tr.initialContent = nil
			// If we got some data before EOF, return it
			if n > 0 {
				return n, nil
			}
			// Otherwise fall through to tail following
		} else if err != nil {
			return n, err
		} else {
			return n, nil
		}
	}

	// Wait for new line from tail
	select {
	case <-tr.ctx.Done():
		return 0, tr.ctx.Err()
	case <-tr.done:
		return 0, io.EOF
	case line, ok := <-tr.lineCh:
		if !ok {
			return 0, io.EOF
		}
		if line.Err != nil {
			return 0, line.Err
		}
		// Add newline to match original file format
		data := line.Text + "\n"
		n := copy(p, data)
		if n < len(data) {
			tr.buf = []byte(data[n:])
		}
		return n, nil
	}
}

func (tr *tailReader) Close() error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.closed {
		return nil
	}
	tr.closed = true
	close(tr.done)

	// Close initial content if still open
	if tr.initialContent != nil {
		tr.initialContent.Close()
		tr.initialContent = nil
	}

	err := tr.t.Stop()
	tr.t.Cleanup()
	return err
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

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Get last N lines
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}
	lastLines := allLines[start:]

	// Build content string efficiently using strings.Join
	content := strings.Join(lastLines, "\n")
	if len(lastLines) > 0 {
		content += "\n"
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

	// Rotate existing files (errors are non-critical - old files might not exist)
	for i := w.maxFiles - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", w.path, i)
		newPath := fmt.Sprintf("%s.%d", w.path, i+1)
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			// Log but continue - rotation of old files is best-effort
			_ = err // acknowledge error but continue
		}
	}

	// Move current file to .1 - this is critical for rotation
	if err := os.Rename(w.path, fmt.Sprintf("%s.1", w.path)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to rotate current log file: %w", err)
	}

	// Delete old files beyond maxFiles
	w.cleanOldFiles()

	// Open new file
	return w.openFile()
}

func (w *rotatingWriter) cleanOldFiles() {
	pattern := w.path + ".*"
	matches, err := filepath.Glob(pattern)
	if err != nil || matches == nil {
		// Glob only returns error for malformed patterns, which shouldn't happen
		// If no matches, nothing to clean
		return
	}

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

	// Remove old files (best-effort cleanup, ignore errors)
	for i := w.maxFiles; i < len(matches); i++ {
		_ = os.Remove(matches[i])
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
