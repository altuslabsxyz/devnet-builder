// internal/plugin/cosmos/builder.go
package cosmos

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// CosmosBuilder builds Cosmos SDK chain binaries
type CosmosBuilder struct {
	binaryName  string
	defaultRepo string
}

// NewCosmosBuilder creates a new Cosmos builder
func NewCosmosBuilder(binaryName, defaultRepo string) *CosmosBuilder {
	return &CosmosBuilder{
		binaryName:  binaryName,
		defaultRepo: defaultRepo,
	}
}

// DefaultGitRepo returns the default git repository
func (b *CosmosBuilder) DefaultGitRepo() string {
	return b.defaultRepo
}

// BinaryName returns the expected binary name
func (b *CosmosBuilder) BinaryName() string {
	return b.binaryName
}

// DefaultBuildFlags returns default build flags for Cosmos SDK chains
func (b *CosmosBuilder) DefaultBuildFlags() map[string]string {
	return map[string]string{
		"ldflags": "-w -s " +
			"-X github.com/cosmos/cosmos-sdk/version.Name={{.BinaryName}} " +
			"-X github.com/cosmos/cosmos-sdk/version.AppName={{.BinaryName}} " +
			"-X github.com/cosmos/cosmos-sdk/version.Version={{.GitRef}} " +
			"-X github.com/cosmos/cosmos-sdk/version.Commit={{.GitCommit}}",
		"tags": "netgo ledger",
	}
}

// BuildBinary compiles the Cosmos SDK binary
func (b *CosmosBuilder) BuildBinary(ctx context.Context, opts types.BuildOptions) error {
	// Merge default flags with user-provided flags
	flags := b.DefaultBuildFlags()
	for k, v := range opts.Flags {
		flags[k] = v
	}

	// Resolve template variables in ldflags
	ldflags, err := b.resolveTemplate(flags["ldflags"], opts)
	if err != nil {
		return fmt.Errorf("failed to resolve ldflags template: %w", err)
	}

	// Build output path
	outputPath := filepath.Join(opts.OutputDir, b.binaryName)

	// Check if Makefile exists and has install target
	makefilePath := filepath.Join(opts.SourceDir, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil {
		// Use make install with overrides
		return b.buildWithMake(ctx, opts, ldflags, flags["tags"], outputPath)
	}

	// Fallback to direct go build
	return b.buildWithGo(ctx, opts, ldflags, flags["tags"], outputPath)
}

func (b *CosmosBuilder) buildWithMake(ctx context.Context, opts types.BuildOptions, ldflags, tags, outputPath string) error {
	// Many Cosmos chains use `make install` which puts binary in GOBIN
	// We'll set GOBIN to our output dir
	//
	// Note: Most Cosmos SDK Makefiles don't respect LDFLAGS env var directly.
	// They typically use VERSION variable to set the version in ldflags.
	// We pass VERSION as a make variable override to ensure version info is injected.

	makeArgs := []string{"install"}
	// Pass VERSION as make variable - this is respected by most Cosmos Makefiles
	if opts.GitRef != "" {
		makeArgs = append(makeArgs, fmt.Sprintf("VERSION=%s", opts.GitRef))
	}
	// Pass COMMIT as make variable for chains that use it
	if opts.GitCommit != "" {
		makeArgs = append(makeArgs, fmt.Sprintf("COMMIT=%s", opts.GitCommit))
	}

	cmd := exec.CommandContext(ctx, "make", makeArgs...)
	cmd.Dir = opts.SourceDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GO111MODULE=on",
		fmt.Sprintf("GOBIN=%s", opts.OutputDir),
		fmt.Sprintf("LDFLAGS=%s", ldflags),
		fmt.Sprintf("BUILD_TAGS=%s", tags),
		// Also set VERSION and COMMIT as env vars for Makefiles that read them
		fmt.Sprintf("VERSION=%s", opts.GitRef),
		fmt.Sprintf("COMMIT=%s", opts.GitCommit),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if opts.Logger != nil {
		opts.Logger.Info("running make install", "dir", opts.SourceDir)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("make install failed: %w\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Verify binary was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found at %s after make install", outputPath)
	}

	return nil
}

func (b *CosmosBuilder) buildWithGo(ctx context.Context, opts types.BuildOptions, ldflags, tags, outputPath string) error {
	// Direct go build
	args := []string{
		"build",
		"-ldflags", ldflags,
		"-o", outputPath,
	}

	if tags != "" {
		args = append(args, "-tags", tags)
	}

	// Find the main package (usually cmd/{binaryName})
	mainPkg := fmt.Sprintf("./cmd/%s", b.binaryName)
	args = append(args, mainPkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = opts.SourceDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GO111MODULE=on",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if opts.Logger != nil {
		opts.Logger.Info("running go build", "args", args)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	return nil
}

func (b *CosmosBuilder) resolveTemplate(tmplStr string, opts types.BuildOptions) (string, error) {
	tmpl, err := template.New("ldflags").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	data := map[string]string{
		"BinaryName": b.binaryName,
		"GitRef":     opts.GitRef,
		"GitCommit":  opts.GitCommit,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ValidateBinary checks if the built binary is valid
func (b *CosmosBuilder) ValidateBinary(ctx context.Context, binaryPath string) error {
	// Run: {binary} version
	cmd := exec.CommandContext(ctx, binaryPath, "version")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("binary validation failed: %w\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		output = strings.TrimSpace(stderr.String()) // some binaries print to stderr
	}

	if output == "" {
		return fmt.Errorf("binary produced no version output")
	}

	return nil
}
