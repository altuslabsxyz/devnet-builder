package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// Logger provides colored output functions for CLI feedback.
type Logger struct {
	out      io.Writer
	errOut   io.Writer
	noColor  bool
	verbose  bool
	jsonMode bool

	// Spinner state
	spinnerMu      sync.Mutex
	spinnerActive  bool
	spinnerStop    chan struct{}
	spinnerDone    chan struct{}
	spinnerMessage string
	autoSpinner    bool // If true, automatically start spinner after Success/Info logs
}

// NewLogger creates a new Logger instance.
func NewLogger() *Logger {
	return &Logger{
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

// SetNoColor disables colored output.
func (l *Logger) SetNoColor(noColor bool) {
	l.noColor = noColor
	color.NoColor = noColor
}

// SetVerbose enables verbose logging.
func (l *Logger) SetVerbose(verbose bool) {
	l.verbose = verbose
}

// SetJSONMode enables JSON output mode (suppresses text output).
func (l *Logger) SetJSONMode(jsonMode bool) {
	l.jsonMode = jsonMode
}

// SetAutoSpinner enables or disables automatic spinner after Success/Info logs.
// When enabled, a spinner will be shown after each Success or Info log to indicate
// ongoing work. The spinner is automatically cleared when the next log is printed.
func (l *Logger) SetAutoSpinner(enabled bool) {
	l.spinnerMu.Lock()
	defer l.spinnerMu.Unlock()

	l.autoSpinner = enabled
	if !enabled && l.spinnerActive {
		l.stopSpinnerLocked()
	}
}

// Info prints an informational message in default color.
// If autoSpinner is enabled, a spinner will be shown after the message.
func (l *Logger) Info(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	l.StopSpinner() // Stop any existing spinner
	fmt.Fprintf(l.out, format+"\n", args...)
	if l.autoSpinner {
		l.StartSpinner("Processing...")
	}
}

// Warn prints a warning message in yellow.
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	l.StopSpinner()
	yellow := color.New(color.FgYellow)
	yellow.Fprintf(l.errOut, "Warning: "+format+"\n", args...)
}

// Error prints an error message in red.
func (l *Logger) Error(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	l.StopSpinner()
	red := color.New(color.FgRed)
	red.Fprintf(l.errOut, "Error: "+format+"\n", args...)
}

// Success prints a success message in green with checkmark.
// If autoSpinner is enabled, a spinner will be shown after the message.
func (l *Logger) Success(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	l.StopSpinner() // Stop any existing spinner
	green := color.New(color.FgGreen)
	green.Fprintf(l.out, "✓ "+format+"\n", args...)
	if l.autoSpinner {
		l.StartSpinner("Processing...")
	}
}

// Debug prints a debug message if verbose mode is enabled.
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.jsonMode || !l.verbose {
		return
	}
	l.StopSpinner()
	gray := color.New(color.FgHiBlack)
	gray.Fprintf(l.out, "[DEBUG] "+format+"\n", args...)
	if l.autoSpinner {
		l.StartSpinner("Processing...")
	}
}

// Bold prints a message in bold.
func (l *Logger) Bold(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	l.StopSpinner()
	bold := color.New(color.Bold)
	bold.Fprintf(l.out, format+"\n", args...)
}

// Cyan prints a message in cyan (for highlights).
func (l *Logger) Cyan(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	l.StopSpinner()
	cyan := color.New(color.FgCyan)
	cyan.Fprintf(l.out, format+"\n", args...)
}

// Print prints a plain message without newline.
func (l *Logger) Print(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	fmt.Fprintf(l.out, format, args...)
}

// Println prints a plain message with newline.
func (l *Logger) Println(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	l.StopSpinner()
	fmt.Fprintf(l.out, format+"\n", args...)
}

// Progress prints a progress bar on the same line (uses carriage return).
// downloaded and total are in bytes. speed is in bytes per second.
func (l *Logger) Progress(downloaded, total int64, speed float64) {
	if l.jsonMode {
		return
	}

	// Calculate percentage
	var percent float64
	if total > 0 {
		percent = float64(downloaded) / float64(total) * 100
	}

	// Format sizes
	downloadedMB := float64(downloaded) / (1024 * 1024)
	totalMB := float64(total) / (1024 * 1024)

	// Format speed
	speedMB := speed / (1024 * 1024)

	// Build progress bar (width: 30 chars)
	barWidth := 30
	filled := int(percent / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	// Calculate ETA
	eta := ""
	if speed > 0 && total > 0 {
		remaining := total - downloaded
		etaSecs := float64(remaining) / speed
		if etaSecs < 60 {
			eta = fmt.Sprintf("%.0fs", etaSecs)
		} else if etaSecs < 3600 {
			eta = fmt.Sprintf("%.1fm", etaSecs/60)
		} else {
			eta = fmt.Sprintf("%.1fh", etaSecs/3600)
		}
	}

	// Print progress line (overwrite previous)
	cyan := color.New(color.FgCyan)
	if total > 0 {
		cyan.Fprintf(l.out, "\r  %s %5.1f%% | %.1f/%.1f MB | %.1f MB/s | ETA: %s    ",
			bar, percent, downloadedMB, totalMB, speedMB, eta)
	} else {
		cyan.Fprintf(l.out, "\r  Downloaded: %.1f MB | %.1f MB/s    ", downloadedMB, speedMB)
	}
}

// ProgressComplete finishes the progress bar and moves to a new line.
func (l *Logger) ProgressComplete() {
	if l.jsonMode {
		return
	}
	fmt.Fprintf(l.out, "\n")
}

// spinnerFrames defines the animation frames for the spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StartSpinner starts an animated spinner with a message.
// The spinner runs in a background goroutine until StopSpinner is called.
func (l *Logger) StartSpinner(message string) {
	if l.jsonMode {
		return
	}

	l.spinnerMu.Lock()
	defer l.spinnerMu.Unlock()

	// If spinner is already running, stop it first
	if l.spinnerActive {
		l.stopSpinnerLocked()
	}

	l.spinnerActive = true
	l.spinnerMessage = message
	l.spinnerStop = make(chan struct{})
	l.spinnerDone = make(chan struct{})

	go l.runSpinner()
}

// runSpinner runs the spinner animation in a goroutine.
func (l *Logger) runSpinner() {
	defer close(l.spinnerDone)

	cyan := color.New(color.FgCyan)
	frameIdx := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-l.spinnerStop:
			return
		case <-ticker.C:
			l.spinnerMu.Lock()
			if l.spinnerActive {
				frame := spinnerFrames[frameIdx%len(spinnerFrames)]
				cyan.Fprintf(l.out, "\r  %s %s", frame, l.spinnerMessage)
				frameIdx++
			}
			l.spinnerMu.Unlock()
		}
	}
}

// StopSpinner stops the spinner and clears the spinner line.
func (l *Logger) StopSpinner() {
	if l.jsonMode {
		return
	}

	l.spinnerMu.Lock()
	defer l.spinnerMu.Unlock()

	l.stopSpinnerLocked()
}

// stopSpinnerLocked stops the spinner (must be called with spinnerMu held).
func (l *Logger) stopSpinnerLocked() {
	if !l.spinnerActive {
		return
	}

	l.spinnerActive = false
	close(l.spinnerStop)
	<-l.spinnerDone

	// Clear the spinner line
	l.clearLineLocked()
}

// clearLineLocked clears the current line (must be called with spinnerMu held).
func (l *Logger) clearLineLocked() {
	// Get terminal width if possible, otherwise use default
	width := 80
	if f, ok := l.out.(*os.File); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			width = w
		}
	}
	fmt.Fprintf(l.out, "\r%s\r", strings.Repeat(" ", width))
}

// clearSpinnerIfActive clears the spinner line if active (for use before logging).
func (l *Logger) clearSpinnerIfActive() {
	l.spinnerMu.Lock()
	defer l.spinnerMu.Unlock()

	if l.spinnerActive {
		l.clearLineLocked()
	}
}

// restoreSpinnerIfActive restores spinner output after logging.
func (l *Logger) restoreSpinnerIfActive() {
	// Spinner goroutine will automatically redraw on next tick
}

// DefaultLogger is the package-level default logger instance.
//
// Deprecated: DefaultLogger is provided for backward compatibility.
// New code should use dependency injection by passing *Logger through
// constructors or configuration structs. Use NewLogger() to create
// a new logger instance instead of relying on this global.
var DefaultLogger = NewLogger()

// Info prints an informational message using the default logger.
//
// Deprecated: Use (*Logger).Info() with an injected logger instead.
func Info(format string, args ...interface{}) {
	DefaultLogger.Info(format, args...)
}

// Warn prints a warning message using the default logger.
//
// Deprecated: Use (*Logger).Warn() with an injected logger instead.
func Warn(format string, args ...interface{}) {
	DefaultLogger.Warn(format, args...)
}

// Error prints an error message using the default logger.
//
// Deprecated: Use (*Logger).Error() with an injected logger instead.
func Error(format string, args ...interface{}) {
	DefaultLogger.Error(format, args...)
}

// Success prints a success message using the default logger.
//
// Deprecated: Use (*Logger).Success() with an injected logger instead.
func Success(format string, args ...interface{}) {
	DefaultLogger.Success(format, args...)
}

// Debug prints a debug message using the default logger.
//
// Deprecated: Use (*Logger).Debug() with an injected logger instead.
func Debug(format string, args ...interface{}) {
	DefaultLogger.Debug(format, args...)
}

// Bold prints a bold message using the default logger.
//
// Deprecated: Use (*Logger).Bold() with an injected logger instead.
func Bold(format string, args ...interface{}) {
	DefaultLogger.Bold(format, args...)
}

// Cyan prints a cyan message using the default logger.
//
// Deprecated: Use (*Logger).Cyan() with an injected logger instead.
func Cyan(format string, args ...interface{}) {
	DefaultLogger.Cyan(format, args...)
}

// IsVerbose returns whether verbose mode is enabled.
func (l *Logger) IsVerbose() bool {
	return l.verbose
}

// Writer returns the underlying writer for stdout.
// This can be used to pass to external commands.
func (l *Logger) Writer() io.Writer {
	if l.jsonMode {
		return io.Discard
	}
	return l.out
}

// ErrWriter returns the underlying writer for stderr.
func (l *Logger) ErrWriter() io.Writer {
	return l.errOut
}

// PrintNodeError prints formatted error information for a failed node.
// Includes log file contents and contextual information.
// In default mode: prints node name, log path, and log contents.
// In verbose mode: also prints command, work directory, and PID.
func (l *Logger) PrintNodeError(info *NodeErrorInfo) {
	if l.jsonMode {
		return
	}

	red := color.New(color.FgRed)
	cyan := color.New(color.FgCyan)
	gray := color.New(color.FgHiBlack)

	// Print separator and header
	fmt.Fprintln(l.errOut)
	red.Fprintln(l.errOut, Separator())
	red.Fprintf(l.errOut, "Node: %s\n", info.NodeName)
	cyan.Fprintf(l.errOut, "Log file: %s\n", info.LogPath)

	// Verbose mode: print additional context
	if l.verbose {
		if info.Command != "" {
			gray.Fprintf(l.errOut, "Command: %s\n", info.Command)
		}
		if info.WorkDir != "" {
			gray.Fprintf(l.errOut, "Work dir: %s\n", info.WorkDir)
		}
		if info.PID > 0 {
			gray.Fprintf(l.errOut, "PID: %d\n", info.PID)
		}
	}

	red.Fprintln(l.errOut, Separator())

	// Print log lines
	if len(info.LogLines) == 0 {
		gray.Fprintln(l.errOut, "(No log content available)")
	} else {
		for _, line := range info.LogLines {
			fmt.Fprintln(l.errOut, line)
		}
	}

	red.Fprintln(l.errOut, Separator())
	fmt.Fprintln(l.errOut)
}

// PrintCommandError prints formatted error information for a failed command.
// In default mode: prints command and stderr output.
// In verbose mode: also prints work directory, stdout, and exit code.
func (l *Logger) PrintCommandError(info *CommandErrorInfo) {
	if l.jsonMode {
		return
	}

	red := color.New(color.FgRed)
	cyan := color.New(color.FgCyan)
	gray := color.New(color.FgHiBlack)

	// Print separator and header
	fmt.Fprintln(l.errOut)
	red.Fprintln(l.errOut, Separator())

	// Build command display
	cmdDisplay := info.Command
	if len(info.Args) > 0 {
		cmdDisplay = info.Command + " " + joinArgs(info.Args)
	}
	cyan.Fprintf(l.errOut, "Command: %s\n", cmdDisplay)

	// Verbose mode: print additional context
	if l.verbose {
		if info.WorkDir != "" {
			gray.Fprintf(l.errOut, "Work dir: %s\n", info.WorkDir)
		}
		gray.Fprintf(l.errOut, "Exit code: %d\n", info.ExitCode)
	}

	red.Fprintln(l.errOut, Separator())

	// Print output (stderr first, then stdout if verbose)
	hasOutput := false
	if info.Stderr != "" {
		fmt.Fprint(l.errOut, info.Stderr)
		hasOutput = true
	}

	if l.verbose && info.Stdout != "" {
		if hasOutput {
			fmt.Fprintln(l.errOut)
		}
		gray.Fprintln(l.errOut, "[stdout]")
		fmt.Fprint(l.errOut, info.Stdout)
		hasOutput = true
	}

	if !hasOutput {
		gray.Fprintln(l.errOut, "(No output available)")
	}

	red.Fprintln(l.errOut, Separator())
	fmt.Fprintln(l.errOut)
}

// joinArgs joins command arguments with spaces.
func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

// PrintNodeErrorDefault prints node error using the default logger.
func PrintNodeErrorDefault(info *NodeErrorInfo) {
	DefaultLogger.PrintNodeError(info)
}

// PrintCommandErrorDefault prints command error using the default logger.
func PrintCommandErrorDefault(info *CommandErrorInfo) {
	DefaultLogger.PrintCommandError(info)
}
