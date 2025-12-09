package output

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Logger provides colored output functions for CLI feedback.
type Logger struct {
	out       io.Writer
	errOut    io.Writer
	noColor   bool
	verbose   bool
	jsonMode  bool
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

// Info prints an informational message in default color.
func (l *Logger) Info(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	fmt.Fprintf(l.out, format+"\n", args...)
}

// Warn prints a warning message in yellow.
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	yellow := color.New(color.FgYellow)
	yellow.Fprintf(l.errOut, "Warning: "+format+"\n", args...)
}

// Error prints an error message in red.
func (l *Logger) Error(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	red := color.New(color.FgRed)
	red.Fprintf(l.errOut, "Error: "+format+"\n", args...)
}

// Success prints a success message in green with checkmark.
func (l *Logger) Success(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	green := color.New(color.FgGreen)
	green.Fprintf(l.out, "✓ "+format+"\n", args...)
}

// Debug prints a debug message if verbose mode is enabled.
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.jsonMode || !l.verbose {
		return
	}
	gray := color.New(color.FgHiBlack)
	gray.Fprintf(l.out, "[DEBUG] "+format+"\n", args...)
}

// Bold prints a message in bold.
func (l *Logger) Bold(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
	bold := color.New(color.Bold)
	bold.Fprintf(l.out, format+"\n", args...)
}

// Cyan prints a message in cyan (for highlights).
func (l *Logger) Cyan(format string, args ...interface{}) {
	if l.jsonMode {
		return
	}
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
	fmt.Fprintf(l.out, format+"\n", args...)
}

// DefaultLogger is the package-level default logger instance.
var DefaultLogger = NewLogger()

// Info prints an informational message using the default logger.
func Info(format string, args ...interface{}) {
	DefaultLogger.Info(format, args...)
}

// Warn prints a warning message using the default logger.
func Warn(format string, args ...interface{}) {
	DefaultLogger.Warn(format, args...)
}

// Error prints an error message using the default logger.
func Error(format string, args ...interface{}) {
	DefaultLogger.Error(format, args...)
}

// Success prints a success message using the default logger.
func Success(format string, args ...interface{}) {
	DefaultLogger.Success(format, args...)
}

// Debug prints a debug message using the default logger.
func Debug(format string, args ...interface{}) {
	DefaultLogger.Debug(format, args...)
}

// Bold prints a bold message using the default logger.
func Bold(format string, args ...interface{}) {
	DefaultLogger.Bold(format, args...)
}

// Cyan prints a cyan message using the default logger.
func Cyan(format string, args ...interface{}) {
	DefaultLogger.Cyan(format, args...)
}
