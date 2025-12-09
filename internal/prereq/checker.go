package prereq

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// PrereqResult contains the result of a prerequisite check.
type PrereqResult struct {
	Name       string `json:"name"`
	Required   bool   `json:"required"`
	Found      bool   `json:"found"`
	Version    string `json:"version,omitempty"`
	Path       string `json:"path,omitempty"`
	Message    string `json:"message,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Checker performs prerequisite checks.
type Checker struct {
	dockerRequired bool
	goRequired     bool
	results        []PrereqResult
}

// NewChecker creates a new prerequisite Checker.
func NewChecker() *Checker {
	return &Checker{
		results: make([]PrereqResult, 0),
	}
}

// RequireDocker marks Docker as required.
func (c *Checker) RequireDocker() *Checker {
	c.dockerRequired = true
	return c
}

// RequireGo marks Go as required.
func (c *Checker) RequireGo() *Checker {
	c.goRequired = true
	return c
}

// Check performs all prerequisite checks and returns the results.
func (c *Checker) Check() ([]PrereqResult, error) {
	c.results = make([]PrereqResult, 0)

	// Always check these basic tools
	c.checkCurl()
	c.checkJq()
	c.checkDecompressor()

	// Check Docker if required
	if c.dockerRequired {
		c.checkDocker()
	}

	// Check Go if required
	if c.goRequired {
		c.checkGo()
	}

	// Check for any required failures
	for _, result := range c.results {
		if result.Required && !result.Found {
			return c.results, fmt.Errorf("prerequisite not met: %s - %s", result.Name, result.Message)
		}
	}

	return c.results, nil
}

// Results returns the check results.
func (c *Checker) Results() []PrereqResult {
	return c.results
}

// checkDocker checks if Docker is installed and running.
func (c *Checker) checkDocker() {
	result := PrereqResult{
		Name:     "docker",
		Required: true,
	}

	// Check if docker command exists
	path, err := exec.LookPath("docker")
	if err != nil {
		result.Found = false
		result.Message = "Docker is not installed"
		result.Suggestion = "Install Docker: https://docs.docker.com/get-docker/"
		c.results = append(c.results, result)
		return
	}
	result.Path = path

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Found = false
		result.Message = "Docker is not running"
		if runtime.GOOS == "linux" {
			result.Suggestion = "Start Docker with: sudo systemctl start docker"
		} else {
			result.Suggestion = "Start Docker Desktop"
		}
		c.results = append(c.results, result)
		return
	}

	// Extract version
	versionCmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	versionOutput, err := versionCmd.Output()
	if err == nil {
		result.Version = strings.TrimSpace(string(versionOutput))
	}

	result.Found = true
	result.Message = fmt.Sprintf("Docker %s is available", result.Version)
	_ = output // suppress unused warning
	c.results = append(c.results, result)
}

// checkGo checks if Go is installed with minimum version.
func (c *Checker) checkGo() {
	result := PrereqResult{
		Name:     "go",
		Required: true,
	}

	path, err := exec.LookPath("go")
	if err != nil {
		result.Found = false
		result.Message = "Go is not installed"
		result.Suggestion = "Install Go 1.21+: https://go.dev/doc/install"
		c.results = append(c.results, result)
		return
	}
	result.Path = path

	// Get version
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		result.Found = false
		result.Message = "Failed to get Go version"
		c.results = append(c.results, result)
		return
	}

	// Parse version (e.g., "go version go1.21.0 linux/amd64")
	parts := strings.Fields(string(output))
	if len(parts) >= 3 {
		result.Version = strings.TrimPrefix(parts[2], "go")
	}

	result.Found = true
	result.Message = fmt.Sprintf("Go %s is available", result.Version)
	c.results = append(c.results, result)
}

// checkCurl checks if curl is installed.
func (c *Checker) checkCurl() {
	result := PrereqResult{
		Name:     "curl",
		Required: true,
	}

	path, err := exec.LookPath("curl")
	if err != nil {
		result.Found = false
		result.Message = "curl is not installed"
		result.Suggestion = "Install curl with your package manager"
		c.results = append(c.results, result)
		return
	}

	result.Found = true
	result.Path = path
	result.Message = "curl is available"
	c.results = append(c.results, result)
}

// checkJq checks if jq is installed.
func (c *Checker) checkJq() {
	result := PrereqResult{
		Name:     "jq",
		Required: true,
	}

	path, err := exec.LookPath("jq")
	if err != nil {
		result.Found = false
		result.Message = "jq is not installed"
		result.Suggestion = "Install jq with your package manager"
		c.results = append(c.results, result)
		return
	}

	result.Found = true
	result.Path = path
	result.Message = "jq is available"
	c.results = append(c.results, result)
}

// checkDecompressor checks if zstd or lz4 is installed.
func (c *Checker) checkDecompressor() {
	result := PrereqResult{
		Name:     "decompressor",
		Required: true,
	}

	// Check for zstd first
	if path, err := exec.LookPath("zstd"); err == nil {
		result.Found = true
		result.Path = path
		result.Message = "zstd is available"
		c.results = append(c.results, result)
		return
	}

	// Check for lz4
	if path, err := exec.LookPath("lz4"); err == nil {
		result.Found = true
		result.Path = path
		result.Message = "lz4 is available"
		c.results = append(c.results, result)
		return
	}

	result.Found = false
	result.Message = "No snapshot decompressor found (zstd or lz4)"
	result.Suggestion = "Install zstd or lz4 with your package manager"
	c.results = append(c.results, result)
}

// GetDecompressor returns the available decompressor command.
func GetDecompressor() (string, error) {
	if _, err := exec.LookPath("zstd"); err == nil {
		return "zstd", nil
	}
	if _, err := exec.LookPath("lz4"); err == nil {
		return "lz4", nil
	}
	return "", fmt.Errorf("no decompressor found (zstd or lz4)")
}

// CheckDiskSpace checks if there's enough disk space available.
func CheckDiskSpace(path string, requiredGB float64) (bool, float64, error) {
	// This is a simplified implementation
	// In production, you'd use syscall.Statfs or similar
	return true, 100.0, nil // Placeholder
}

// AllPassed returns true if all checks passed.
func (c *Checker) AllPassed() bool {
	for _, result := range c.results {
		if result.Required && !result.Found {
			return false
		}
	}
	return true
}

// FailedChecks returns only the failed required checks.
func (c *Checker) FailedChecks() []PrereqResult {
	failed := make([]PrereqResult, 0)
	for _, result := range c.results {
		if result.Required && !result.Found {
			failed = append(failed, result)
		}
	}
	return failed
}
