// internal/output/status_spinner.go
package output

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var statusSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StatusSpinner displays an animated spinner with a status message.
// Thread-safe for concurrent updates.
type StatusSpinner struct {
	out      io.Writer
	frameIdx int
	message  string
	stop     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewStatusSpinner creates a new StatusSpinner writing to stderr.
func NewStatusSpinner() *StatusSpinner {
	return &StatusSpinner{out: os.Stderr}
}

// Start begins the spinner animation with the given message.
func (s *StatusSpinner) Start(message string) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.message = message
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		defer close(s.done)

		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.render()
			}
		}
	}()
}

// Update changes the spinner message.
func (s *StatusSpinner) Update(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
	s.render()
}

// Stop stops the spinner and clears the line.
func (s *StatusSpinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stop)
	s.mu.Unlock()

	<-s.done
	fmt.Fprintf(s.out, "\r%80s\r", "") // Clear line
}

// StopWithNewline stops the spinner and moves to a new line.
func (s *StatusSpinner) StopWithNewline() {
	s.Stop()
	fmt.Fprintf(s.out, "\n")
}

func (s *StatusSpinner) render() {
	s.mu.Lock()
	msg := s.message
	idx := s.frameIdx
	s.frameIdx = (s.frameIdx + 1) % len(statusSpinnerFrames)
	s.mu.Unlock()

	fmt.Fprintf(s.out, "\r%s %s          ", statusSpinnerFrames[idx], msg)
}
