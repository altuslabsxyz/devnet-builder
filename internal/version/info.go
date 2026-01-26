// Package version provides version information and CLI command support for devnet-builder tools.
package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Build-time variables injected via ldflags
// These are set by GoReleaser or Makefile during the build process:
//
//	-X github.com/altuslabsxyz/devnet-builder/internal/version.Version={{.Version}}
//	-X github.com/altuslabsxyz/devnet-builder/internal/version.GitCommit={{.FullCommit}}
//	-X github.com/altuslabsxyz/devnet-builder/internal/version.BuildDate={{.Date}}
var (
	// Version is the semantic version of the application.
	// Set at build time via ldflags, defaults to "0.1.0-dev" for local builds.
	Version = "0.1.0-dev"

	// GitCommit is the git commit hash of the build.
	// Set at build time via ldflags.
	GitCommit = "unknown"

	// BuildDate is the date when the binary was built.
	// Set at build time via ldflags.
	BuildDate = "unknown"
)

// Info contains all version and build information.
type Info struct {
	Name       string   `json:"name" yaml:"name"`
	ServerName string   `json:"server_name" yaml:"server_name"`
	Version    string   `json:"version" yaml:"version"`
	GitCommit  string   `json:"commit" yaml:"commit"`
	BuildDate  string   `json:"build_date,omitempty" yaml:"build_date,omitempty"`
	GoVersion  string   `json:"go" yaml:"go"`
	BuildTags  string   `json:"build_tags,omitempty" yaml:"build_tags,omitempty"`
	BuildDeps  []string `json:"build_deps,omitempty" yaml:"build_deps,omitempty"`
}

// NewInfo creates a new Info struct with the given app name and server name.
func NewInfo(name, serverName string) Info {
	return Info{
		Name:       name,
		ServerName: serverName,
		Version:    Version,
		GitCommit:  GitCommit,
		BuildDate:  BuildDate,
		GoVersion:  fmt.Sprintf("go version %s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	}
}

// WithBuildDeps populates the build dependencies from runtime/debug.
func (i Info) WithBuildDeps() Info {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return i
	}

	// Collect build settings for tags
	var buildTags []string
	for _, setting := range buildInfo.Settings {
		if setting.Key == "-tags" && setting.Value != "" {
			buildTags = append(buildTags, setting.Value)
		}
	}
	if len(buildTags) > 0 {
		i.BuildTags = strings.Join(buildTags, ",")
	}

	// Collect dependencies
	deps := make([]string, 0, len(buildInfo.Deps))
	for _, dep := range buildInfo.Deps {
		depStr := fmt.Sprintf("%s@%s", dep.Path, dep.Version)
		if dep.Replace != nil {
			depStr = fmt.Sprintf("%s@%s => %s@%s", dep.Path, dep.Version, dep.Replace.Path, dep.Replace.Version)
		}
		deps = append(deps, depStr)
	}

	// Sort dependencies alphabetically
	sort.Strings(deps)
	i.BuildDeps = deps

	return i
}

// String returns a formatted string representation of the version info.
func (i Info) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s version %s\n", i.Name, i.Version))
	sb.WriteString(fmt.Sprintf("  commit:     %s\n", i.GitCommit))
	sb.WriteString(fmt.Sprintf("  build date: %s\n", i.BuildDate))
	sb.WriteString(fmt.Sprintf("  go:         %s\n", i.GoVersion))
	return sb.String()
}

// LongString returns a detailed YAML-formatted string including build dependencies.
func (i Info) LongString() string {
	// Use YAML format like Cosmos SDK apps
	data, err := yaml.Marshal(i)
	if err != nil {
		return i.String()
	}
	return string(data)
}

// JSON returns the version info as a JSON string.
func (i Info) JSON() (string, error) {
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// NewCmd creates a version command for the given app name and server name.
// The command supports:
//   - --long: Show detailed version info including build dependencies
//   - --json: Output in JSON format
func NewCmd(name, serverName string) *cobra.Command {
	var (
		long       bool
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print version information including build details. Use --long for detailed dependency info.",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := NewInfo(name, serverName)

			if long {
				info = info.WithBuildDeps()
			}

			if jsonOutput {
				output, err := info.JSON()
				if err != nil {
					return err
				}
				fmt.Println(output)
				return nil
			}

			if long {
				fmt.Print(info.LongString())
			} else {
				fmt.Print(info.String())
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&long, "long", false, "Show detailed version info including build dependencies")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output version info in JSON format")

	return cmd
}
