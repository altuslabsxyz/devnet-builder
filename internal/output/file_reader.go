package output

import (
	"bufio"
	"os"
)

// DefaultLogLines is the default number of lines to read from log files.
const DefaultLogLines = 20

// ReadLastLines reads the last n lines from a file.
// Returns empty slice if file doesn't exist or is empty.
// Returns appropriate error messages for edge cases.
func ReadLastLines(filePath string, n int) ([]string, error) {
	if n <= 0 {
		n = DefaultLogLines
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &FileNotFoundError{Path: filePath}
		}
		if os.IsPermission(err) {
			return nil, &PermissionDeniedError{Path: filePath}
		}
		return nil, err
	}
	defer file.Close()

	// Read all lines into a buffer
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return empty check
	if len(lines) == 0 {
		return nil, &EmptyFileError{Path: filePath}
	}

	// Return last n lines
	if len(lines) <= n {
		return lines, nil
	}

	return lines[len(lines)-n:], nil
}

// FileNotFoundError indicates the log file does not exist.
type FileNotFoundError struct {
	Path string
}

func (e *FileNotFoundError) Error() string {
	return "no log file found at " + e.Path
}

// PermissionDeniedError indicates the log file cannot be read due to permissions.
type PermissionDeniedError struct {
	Path string
}

func (e *PermissionDeniedError) Error() string {
	return "cannot read log file: permission denied at " + e.Path
}

// EmptyFileError indicates the log file is empty.
type EmptyFileError struct {
	Path string
}

func (e *EmptyFileError) Error() string {
	return "log file is empty at " + e.Path
}
