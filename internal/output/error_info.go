package output

// NodeErrorInfo contains error information for a failed node.
type NodeErrorInfo struct {
	NodeName string   // Node identifier (e.g., "node0", "node1")
	NodeDir  string   // Node home directory path
	LogPath  string   // Full path to log file
	LogLines []string // Last N lines from log file
	Error    error    // The error that occurred
	Command  string   // Executed command (for verbose mode)
	WorkDir  string   // Working directory (for verbose mode)
	PID      int      // Process ID (if available)
}

// CommandErrorInfo contains error information for a failed external command.
type CommandErrorInfo struct {
	Command  string   // Full command that was executed
	Args     []string // Command arguments
	WorkDir  string   // Working directory
	Stdout   string   // Standard output content
	Stderr   string   // Standard error content
	ExitCode int      // Exit code
	Error    error    // The error that occurred
}
