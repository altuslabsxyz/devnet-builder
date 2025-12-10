package output

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Progress provides progress indication for multi-step operations.
type Progress struct {
	out      io.Writer
	total    int
	current  int
	noColor  bool
	jsonMode bool
}

// NewProgress creates a new Progress instance with the given total steps.
func NewProgress(total int) *Progress {
	return &Progress{
		out:   os.Stdout,
		total: total,
	}
}

// SetNoColor disables colored output.
func (p *Progress) SetNoColor(noColor bool) {
	p.noColor = noColor
}

// SetJSONMode enables JSON output mode (suppresses text output).
func (p *Progress) SetJSONMode(jsonMode bool) {
	p.jsonMode = jsonMode
}

// Stage prints a progress stage message in format [N/M] Description...
func (p *Progress) Stage(description string) {
	if p.jsonMode {
		return
	}
	p.current++
	cyan := color.New(color.FgCyan)
	cyan.Fprintf(p.out, "[%d/%d] %s...\n", p.current, p.total, description)
}

// StageWithNumber prints a progress stage with explicit step number.
func (p *Progress) StageWithNumber(step int, description string) {
	if p.jsonMode {
		return
	}
	p.current = step
	cyan := color.New(color.FgCyan)
	cyan.Fprintf(p.out, "[%d/%d] %s...\n", step, p.total, description)
}

// Reset resets the progress counter.
func (p *Progress) Reset() {
	p.current = 0
}

// SetTotal updates the total number of steps.
func (p *Progress) SetTotal(total int) {
	p.total = total
}

// Current returns the current step number.
func (p *Progress) Current() int {
	return p.current
}

// Total returns the total number of steps.
func (p *Progress) Total() int {
	return p.total
}

// Done prints a completion message.
func (p *Progress) Done(message string) {
	if p.jsonMode {
		return
	}
	green := color.New(color.FgGreen)
	green.Fprintf(p.out, "\n✓ %s\n", message)
}

// Spinner represents a simple spinner for indeterminate progress.
type Spinner struct {
	out      io.Writer
	message  string
	running  bool
	noColor  bool
	jsonMode bool
}

// NewSpinner creates a new Spinner with the given message.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		out:     os.Stdout,
		message: message,
	}
}

// Start prints the spinner message (simplified - no animation in CLI).
func (s *Spinner) Start() {
	if s.jsonMode {
		return
	}
	s.running = true
	fmt.Fprintf(s.out, "%s... ", s.message)
}

// Stop stops the spinner and prints done.
func (s *Spinner) Stop(success bool) {
	if s.jsonMode {
		return
	}
	s.running = false
	if success {
		green := color.New(color.FgGreen)
		green.Fprintln(s.out, "done")
	} else {
		red := color.New(color.FgRed)
		red.Fprintln(s.out, "failed")
	}
}

// StopWithMessage stops the spinner with a custom message.
func (s *Spinner) StopWithMessage(message string, success bool) {
	if s.jsonMode {
		return
	}
	s.running = false
	if success {
		green := color.New(color.FgGreen)
		green.Fprintln(s.out, message)
	} else {
		red := color.New(color.FgRed)
		red.Fprintln(s.out, message)
	}
}

// SetNoColor disables colored output.
func (s *Spinner) SetNoColor(noColor bool) {
	s.noColor = noColor
}

// SetJSONMode enables JSON output mode.
func (s *Spinner) SetJSONMode(jsonMode bool) {
	s.jsonMode = jsonMode
}
