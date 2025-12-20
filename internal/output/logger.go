package output

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Logger provides colored output functions for CLI feedback.
type Logger struct {
	out      io.Writer
	errOut   io.Writer
	noColor  bool
	verbose  bool
	jsonMode bool
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
//
// Deprecated: DefaultLogger is provided for backward compatibility.
// New code should use dependency injection by passing *Logger through
// constructors or configuration structs. Use NewLogger() to create
// a new logger instance instead of relying on this global.
var DefaultLogger = NewLogger()

// Info prints an informational message using the default logger.
// Deprecated: Use (*Logger).Info() with an injected logger instead.
func Info(format string, args ...interface{}) {
	DefaultLogger.Info(format, args...)
}

// Warn prints a warning message using the default logger.
// Deprecated: Use (*Logger).Warn() with an injected logger instead.
func Warn(format string, args ...interface{}) {
	DefaultLogger.Warn(format, args...)
}

// Error prints an error message using the default logger.
// Deprecated: Use (*Logger).Error() with an injected logger instead.
func Error(format string, args ...interface{}) {
	DefaultLogger.Error(format, args...)
}

// Success prints a success message using the default logger.
// Deprecated: Use (*Logger).Success() with an injected logger instead.
func Success(format string, args ...interface{}) {
	DefaultLogger.Success(format, args...)
}

// Debug prints a debug message using the default logger.
// Deprecated: Use (*Logger).Debug() with an injected logger instead.
func Debug(format string, args ...interface{}) {
	DefaultLogger.Debug(format, args...)
}

// Bold prints a bold message using the default logger.
// Deprecated: Use (*Logger).Bold() with an injected logger instead.
func Bold(format string, args ...interface{}) {
	DefaultLogger.Bold(format, args...)
}

// Cyan prints a cyan message using the default logger.
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
