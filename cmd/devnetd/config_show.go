// cmd/devnetd/config_show.go
package main

import (
	"fmt"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/config"
	"github.com/spf13/cobra"
)

func newConfigShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show effective configuration",
		Long:  `Displays the effective configuration after merging defaults, file, and environment variables.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir := config.DefaultDataDir()
			loader := config.NewLoader(dataDir, "")
			cfg, err := loader.Load()
			if err != nil {
				return err
			}

			fmt.Println("Effective devnetd configuration:")
			fmt.Println(strings.Repeat("-", 50))
			fmt.Println()
			fmt.Println("[server]")
			fmt.Printf("  socket      = %q\n", cfg.Server.Socket)
			fmt.Printf("  data_dir    = %q\n", cfg.Server.DataDir)
			fmt.Printf("  log_level   = %q\n", cfg.Server.LogLevel)
			fmt.Printf("  workers     = %d\n", cfg.Server.Workers)
			fmt.Printf("  foreground  = %v\n", cfg.Server.Foreground)
			fmt.Println()
			fmt.Println("[docker]")
			fmt.Printf("  enabled     = %v\n", cfg.Docker.Enabled)
			fmt.Printf("  image       = %q\n", cfg.Docker.Image)
			fmt.Println()
			fmt.Println("[github]")
			token := cfg.GitHub.Token
			if token != "" {
				if len(token) > 4 {
					token = token[:4] + "****"
				} else {
					token = "****"
				}
			}
			fmt.Printf("  token       = %q\n", token)
			fmt.Println()
			fmt.Println("[timeouts]")
			fmt.Printf("  shutdown          = %s\n", cfg.Timeouts.Shutdown)
			fmt.Printf("  health_check      = %s\n", cfg.Timeouts.HealthCheck)
			fmt.Printf("  snapshot_download = %s\n", cfg.Timeouts.SnapshotDownload)
			fmt.Println()
			fmt.Println("[snapshot]")
			fmt.Printf("  cache_ttl    = %s\n", cfg.Snapshot.CacheTTL)
			fmt.Printf("  max_retries  = %d\n", cfg.Snapshot.MaxRetries)
			fmt.Printf("  retry_delay  = %s\n", cfg.Snapshot.RetryDelay)
			fmt.Println()
			fmt.Println("[network]")
			fmt.Printf("  port_offset    = %d\n", cfg.Network.PortOffset)
			fmt.Printf("  base_rpc_port  = %d\n", cfg.Network.BaseRPCPort)
			fmt.Printf("  base_p2p_port  = %d\n", cfg.Network.BaseP2PPort)
			fmt.Printf("  base_rest_port = %d\n", cfg.Network.BaseRESTPort)
			fmt.Printf("  base_grpc_port = %d\n", cfg.Network.BaseGRPCPort)

			return nil
		},
	}

	return cmd
}
