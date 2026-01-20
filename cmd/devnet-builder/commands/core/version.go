package core

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
	"github.com/altuslabsxyz/devnet-builder/internal"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/spf13/cobra"
)

// VersionInfo contains version details.
type VersionInfo struct {
	Version       string   `json:"version"`
	GitCommit     string   `json:"git_commit"`
	BuildDate     string   `json:"build_date"`
	GoVersion     string   `json:"go_version"`
	Platform      string   `json:"platform"`
	BuildNetworks string   `json:"build_networks"`
	Networks      []string `json:"networks"` // Actual registered networks
}

// NewVersionCmd creates the version command.
func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Show version information including build details.",
		RunE:  runVersion,
	}

	return cmd
}

func runVersion(cmd *cobra.Command, args []string) error {
	jsonMode := shared.GetJSONMode()

	// Get actual registered networks
	registeredNetworks := network.List()

	info := VersionInfo{
		Version:       internal.Version,
		GitCommit:     internal.GitCommit,
		BuildDate:     internal.BuildDate,
		GoVersion:     runtime.Version(),
		Platform:      fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		BuildNetworks: "plugin-based",
		Networks:      registeredNetworks,
	}

	if jsonMode {
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("devnet-builder %s\n", info.Version)
	fmt.Printf("  Git commit: %s\n", info.GitCommit)
	fmt.Printf("  Build date: %s\n", info.BuildDate)
	fmt.Printf("  Go version: %s\n", info.GoVersion)
	fmt.Printf("  Platform:   %s\n", info.Platform)
	fmt.Printf("  Networks:   %s\n", strings.Join(registeredNetworks, ", "))

	return nil
}
