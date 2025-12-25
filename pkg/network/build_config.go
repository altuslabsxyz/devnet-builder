// Package network provides the public SDK for developing devnet-builder network plugins.
package network

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// BuildConfig contains network-specific build configuration for compiling binaries.
// This enables plugins to customize binary compilation for different deployment targets
// (mainnet, testnet, devnet) by providing custom build tags, linker flags, and
// environment variables.
//
// Example usage:
//
//	config := &BuildConfig{
//	    Tags:    []string{"netgo", "ledger"},
//	    LDFlags: []string{"-X github.com/example/app.EVMChainID=988", "-w", "-s"},
//	    Env:     map[string]string{"CGO_ENABLED": "0"},
//	}
//
// All fields are optional. If a plugin doesn't need custom build configuration,
// it can return an empty BuildConfig{}.
type BuildConfig struct {
	// Tags are Go build tags passed to the compiler.
	// These enable conditional compilation of code.
	// Examples: ["netgo", "ledger", "osusergo", "no_dynamic_precompiles"]
	Tags []string `json:"tags,omitempty"`

	// LDFlags are linker flags passed to the Go linker.
	// These are used to inject values at compile-time using -X flag.
	// Format: ["-X package.Variable=value", "-w", "-s"]
	// Examples:
	//   - "-X github.com/stablelabs/stable/app.EVMChainID=988" (set EVM chain ID)
	//   - "-w" (omit DWARF symbol table)
	//   - "-s" (omit symbol table and debug information)
	LDFlags []string `json:"ldflags,omitempty"`

	// Env contains environment variables for the build process.
	// These are set before running the build command.
	// Examples:
	//   - {"CGO_ENABLED": "0"} (disable CGO for static binary)
	//   - {"GOOS": "linux", "GOARCH": "amd64"} (cross-compilation)
	Env map[string]string `json:"env,omitempty"`

	// ExtraArgs are additional arguments passed to the build tool (goreleaser).
	// Examples: ["--skip-validate", "--debug", "--clean"]
	ExtraArgs []string `json:"extra_args,omitempty"`
}

// Validate checks if the BuildConfig is valid.
// It performs comprehensive validation of all fields to catch configuration
// errors early before attempting to build binaries.
//
// Returns:
//   - nil if valid
//   - error describing the validation failure
func (b *BuildConfig) Validate() error {
	if b == nil {
		return nil // nil config is valid (treated as empty config)
	}

	// Validate build tags for duplicates
	if err := b.validateTags(); err != nil {
		return fmt.Errorf("invalid tags: %w", err)
	}

	// Validate ldflags format and safety
	if err := b.validateLDFlags(); err != nil {
		return fmt.Errorf("invalid ldflags: %w", err)
	}

	// Validate environment variables
	if err := b.validateEnv(); err != nil {
		return fmt.Errorf("invalid env: %w", err)
	}

	return nil
}

// validateTags checks build tags for duplicates.
func (b *BuildConfig) validateTags() error {
	seen := make(map[string]bool, len(b.Tags))
	for _, tag := range b.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return fmt.Errorf("empty build tag")
		}
		if seen[tag] {
			return fmt.Errorf("duplicate build tag: %s", tag)
		}
		seen[tag] = true
	}
	return nil
}

// validateLDFlags checks linker flags for proper format and dangerous patterns.
func (b *BuildConfig) validateLDFlags() error {
	for _, flag := range b.LDFlags {
		flag = strings.TrimSpace(flag)
		if flag == "" {
			return fmt.Errorf("empty ldflag")
		}

		// Ldflags must start with hyphen
		if !strings.HasPrefix(flag, "-") {
			return fmt.Errorf("ldflag must start with '-': %s", flag)
		}

		// Check for dangerous flags that could execute arbitrary code
		dangerousPatterns := []string{
			"--exec", "--run", "--command", // Command execution
			"../",                          // Path traversal
		}
		for _, pattern := range dangerousPatterns {
			if strings.Contains(strings.ToLower(flag), pattern) {
				return fmt.Errorf("dangerous ldflag pattern detected: %s (contains %s)", flag, pattern)
			}
		}
	}
	return nil
}

// List of environment variables allowed in build configurations.
// This whitelist prevents injection of malicious or unexpected environment variables
// that could compromise the build process.
var allowedEnvVars = map[string]bool{
	// Go toolchain
	"CGO_ENABLED": true,
	"GOOS":        true,
	"GOARCH":      true,
	"GOARM":       true,
	"GO111MODULE": true,
	"GOPROXY":     true,
	"GOSUMDB":     true,
	"GOPRIVATE":   true,

	// C/C++ compiler (for CGO)
	"CC":     true,
	"CXX":    true,
	"AR":     true,
	"CFLAGS": true,

	// Build tags and flags
	"GOFLAGS": true,
}

// validateEnv checks environment variables against a whitelist.
func (b *BuildConfig) validateEnv() error {
	for key, value := range b.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("empty environment variable key")
		}

		if !allowedEnvVars[key] {
			return fmt.Errorf("environment variable not allowed: %s (allowed: %v)",
				key, getAllowedEnvVarNames())
		}

		// Check for suspicious values (shell injection attempts)
		if strings.ContainsAny(value, "$`;|&\n") {
			return fmt.Errorf("environment variable contains suspicious characters: %s=%s", key, value)
		}
	}
	return nil
}

// getAllowedEnvVarNames returns a sorted list of allowed environment variable names.
func getAllowedEnvVarNames() []string {
	names := make([]string, 0, len(allowedEnvVars))
	for name := range allowedEnvVars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Merge combines two BuildConfig instances, with other taking precedence.
// This is useful for combining default configurations with plugin-specific overrides.
//
// Merge behavior:
//   - Tags: Appends other's tags to this config's tags (no deduplication)
//   - LDFlags: Appends other's ldflags to this config's ldflags
//   - Env: Merges environment variables, with other's values overriding conflicts
//   - ExtraArgs: Appends other's args to this config's args
//
// Example:
//
//	base := &BuildConfig{
//	    Tags:    []string{"netgo"},
//	    LDFlags: []string{"-w"},
//	    Env:     map[string]string{"CGO_ENABLED": "0"},
//	}
//	override := &BuildConfig{
//	    Tags:    []string{"ledger"},
//	    LDFlags: []string{"-X main.Version=1.0"},
//	    Env:     map[string]string{"GOOS": "linux"},
//	}
//	merged := base.Merge(override)
//	// Result:
//	// Tags:    ["netgo", "ledger"]
//	// LDFlags: ["-w", "-X main.Version=1.0"]
//	// Env:     {"CGO_ENABLED": "0", "GOOS": "linux"}
func (b *BuildConfig) Merge(other *BuildConfig) *BuildConfig {
	if b == nil && other == nil {
		return &BuildConfig{}
	}
	if b == nil {
		return other.Clone()
	}
	if other == nil {
		return b.Clone()
	}

	result := &BuildConfig{
		Tags:      make([]string, 0, len(b.Tags)+len(other.Tags)),
		LDFlags:   make([]string, 0, len(b.LDFlags)+len(other.LDFlags)),
		Env:       make(map[string]string, len(b.Env)+len(other.Env)),
		ExtraArgs: make([]string, 0, len(b.ExtraArgs)+len(other.ExtraArgs)),
	}

	// Merge tags (append)
	result.Tags = append(result.Tags, b.Tags...)
	result.Tags = append(result.Tags, other.Tags...)

	// Merge ldflags (append)
	result.LDFlags = append(result.LDFlags, b.LDFlags...)
	result.LDFlags = append(result.LDFlags, other.LDFlags...)

	// Merge environment variables (base first, then override)
	for k, v := range b.Env {
		result.Env[k] = v
	}
	for k, v := range other.Env {
		result.Env[k] = v
	}

	// Merge extra args (append)
	result.ExtraArgs = append(result.ExtraArgs, b.ExtraArgs...)
	result.ExtraArgs = append(result.ExtraArgs, other.ExtraArgs...)

	return result
}

// Clone creates a deep copy of the BuildConfig.
// This is useful for creating independent configurations that can be modified
// without affecting the original.
func (b *BuildConfig) Clone() *BuildConfig {
	if b == nil {
		return &BuildConfig{}
	}

	result := &BuildConfig{
		Tags:      make([]string, len(b.Tags)),
		LDFlags:   make([]string, len(b.LDFlags)),
		Env:       make(map[string]string, len(b.Env)),
		ExtraArgs: make([]string, len(b.ExtraArgs)),
	}

	copy(result.Tags, b.Tags)
	copy(result.LDFlags, b.LDFlags)
	copy(result.ExtraArgs, b.ExtraArgs)

	for k, v := range b.Env {
		result.Env[k] = v
	}

	return result
}

// IsEmpty returns true if the BuildConfig has no configuration.
// This is useful for checking if a plugin provided any custom build configuration.
func (b *BuildConfig) IsEmpty() bool {
	if b == nil {
		return true
	}
	return len(b.Tags) == 0 &&
		len(b.LDFlags) == 0 &&
		len(b.Env) == 0 &&
		len(b.ExtraArgs) == 0
}

// Hash computes a unique hash of the BuildConfig.
// This is used for cache key generation to ensure binaries built with different
// configurations are cached separately.
//
// The hash includes all configuration fields:
//   - Tags (sorted for deterministic hashing)
//   - LDFlags (sorted for deterministic hashing)
//   - Env (sorted by key for deterministic hashing)
//   - ExtraArgs (sorted for deterministic hashing)
//
// Returns: 16-character hex string (first 64 bits of SHA256 hash)
func (b *BuildConfig) Hash() string {
	if b == nil || b.IsEmpty() {
		return "empty"
	}

	h := sha256.New()

	// Hash tags (sorted)
	tags := make([]string, len(b.Tags))
	copy(tags, b.Tags)
	sort.Strings(tags)
	for _, tag := range tags {
		h.Write([]byte("tag:" + tag + "\n"))
	}

	// Hash ldflags (sorted)
	ldflags := make([]string, len(b.LDFlags))
	copy(ldflags, b.LDFlags)
	sort.Strings(ldflags)
	for _, flag := range ldflags {
		h.Write([]byte("ldflag:" + flag + "\n"))
	}

	// Hash env vars (sorted by key)
	envKeys := make([]string, 0, len(b.Env))
	for k := range b.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)
	for _, k := range envKeys {
		h.Write([]byte(fmt.Sprintf("env:%s=%s\n", k, b.Env[k])))
	}

	// Hash extra args (sorted)
	args := make([]string, len(b.ExtraArgs))
	copy(args, b.ExtraArgs)
	sort.Strings(args)
	for _, arg := range args {
		h.Write([]byte("arg:" + arg + "\n"))
	}

	// Return first 16 hex characters (64 bits) of SHA256 hash
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8])
}

// String returns a human-readable representation of the BuildConfig.
// This is useful for logging and debugging.
func (b *BuildConfig) String() string {
	if b == nil || b.IsEmpty() {
		return "BuildConfig{empty}"
	}

	var parts []string

	if len(b.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags=%v", b.Tags))
	}
	if len(b.LDFlags) > 0 {
		parts = append(parts, fmt.Sprintf("ldflags=%v", b.LDFlags))
	}
	if len(b.Env) > 0 {
		parts = append(parts, fmt.Sprintf("env=%v", b.Env))
	}
	if len(b.ExtraArgs) > 0 {
		parts = append(parts, fmt.Sprintf("args=%v", b.ExtraArgs))
	}

	return fmt.Sprintf("BuildConfig{%s}", strings.Join(parts, ", "))
}
