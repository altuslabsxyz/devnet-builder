package output

import "io"

// LoggerInterface defines the logging interface for services.
// This allows for dependency injection and easier testing.
type LoggerInterface interface {
	// Core logging methods
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Success(format string, args ...interface{})

	// Output methods
	Print(format string, args ...interface{})
	Println(format string, args ...interface{})
	Bold(format string, args ...interface{})
	Cyan(format string, args ...interface{})

	// Configuration methods
	SetVerbose(verbose bool)
	SetNoColor(noColor bool)
	SetJSONMode(jsonMode bool)
	IsVerbose() bool

	// Writer access
	Writer() io.Writer
	ErrWriter() io.Writer

	// Error info printing
	PrintNodeError(info *NodeErrorInfo)
	PrintCommandError(info *CommandErrorInfo)
}

// Verify that Logger implements LoggerInterface at compile time.
var _ LoggerInterface = (*Logger)(nil)
