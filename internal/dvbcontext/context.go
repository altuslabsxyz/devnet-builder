// Package dvbcontext provides context management for dvb CLI.
// It allows setting a default namespace/devnet so users don't need to
// specify them on every command.
package dvbcontext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const contextFileName = "context"

// Context represents the current dvb context (namespace/devnet).
type Context struct {
	Namespace string
	Devnet    string
}

// String returns the context as "namespace/devnet".
func (c *Context) String() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Devnet)
}

// contextFilePath returns the path to the context file.
func contextFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".devnet-builder", contextFileName), nil
}

// Load reads the current context from file.
// Returns nil (not error) if no context is set.
func Load() (*Context, error) {
	path, err := contextFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // No context set
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read context file: %w", err)
	}

	ref := strings.TrimSpace(string(data))
	if ref == "" {
		return nil, nil
	}

	ns, devnet := ParseRef(ref)
	return &Context{Namespace: ns, Devnet: devnet}, nil
}

// Save writes the context to file.
func Save(namespace, devnet string) error {
	path, err := contextFilePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	content := fmt.Sprintf("%s/%s\n", namespace, devnet)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}

	return nil
}

// Clear removes the context file.
func Clear() error {
	path, err := contextFilePath()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Already cleared
	}
	if err != nil {
		return fmt.Errorf("failed to clear context: %w", err)
	}
	return nil
}

// ParseRef parses "namespace/devnet" or "devnet" into components.
// If no namespace is specified, "default" is used.
func ParseRef(ref string) (namespace, devnet string) {
	ref = strings.TrimSpace(ref)
	if idx := strings.Index(ref, "/"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}
	return "default", ref
}
