// Package ports defines the interfaces (ports) that the application layer
// requires from the infrastructure layer. This follows the Ports and Adapters
// (Hexagonal) architecture pattern, enabling dependency inversion.
package ports

import "io"

// Logger defines the logging interface for the application layer.
// Infrastructure implementations can provide console, file, or structured logging.
type Logger interface {
	// Core logging methods
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Success(format string, args ...interface{})

	// Output methods
	Print(format string, args ...interface{})
	Println(format string, args ...interface{})

	// Configuration
	SetVerbose(verbose bool)
	IsVerbose() bool

	// Writer access for external commands
	Writer() io.Writer
	ErrWriter() io.Writer
}
