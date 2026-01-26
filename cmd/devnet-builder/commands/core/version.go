package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/version"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

// ExtendedVersionInfo contains version details with network plugin info.
type ExtendedVersionInfo struct {
	version.Info `yaml:",inline"`
	Networks     []string `json:"networks,omitempty" yaml:"networks,omitempty"`
}

// NewVersionCmd creates the version command.
func NewVersionCmd() *cobra.Command {
	var long bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Show version information including build details. Use --long for detailed dependency info.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := ctxconfig.FromContext(cmd.Context())
			jsonMode := cfg.JSONMode()

			// Get actual registered networks
			registeredNetworks := network.List()

			// Create base version info
			info := version.NewInfo("devnet-builder", "devnet-builder")
			if long {
				info = info.WithBuildDeps()
			}

			// Extend with network info
			extInfo := ExtendedVersionInfo{
				Info:     info,
				Networks: registeredNetworks,
			}

			if jsonMode {
				data, err := json.MarshalIndent(extInfo, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				return nil
			}

			if long {
				fmt.Print(extInfo.Info.LongString())
				if len(registeredNetworks) > 0 {
					fmt.Printf("networks: %s\n", strings.Join(registeredNetworks, ", "))
				}
			} else {
				fmt.Print(extInfo.Info.String())
				if len(registeredNetworks) > 0 {
					fmt.Printf("  networks: %s\n", strings.Join(registeredNetworks, ", "))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&long, "long", false, "Show detailed version info including build dependencies")

	return cmd
}
