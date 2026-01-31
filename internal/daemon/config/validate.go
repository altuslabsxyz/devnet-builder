// internal/daemon/config/validate.go
package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidLogLevels are the allowed log level values.
var ValidLogLevels = []string{"debug", "info", "warn", "error"}

// Validate validates the configuration and returns an error if invalid.
func Validate(cfg *Config) error {
	var errs []string

	// Validate log level
	validLevel := false
	for _, level := range ValidLogLevels {
		if cfg.Server.LogLevel == level {
			validLevel = true
			break
		}
	}
	if !validLevel {
		errs = append(errs, fmt.Sprintf("invalid log_level %q (must be one of: %s)",
			cfg.Server.LogLevel, strings.Join(ValidLogLevels, ", ")))
	}

	// Validate workers
	if cfg.Server.Workers < 1 {
		errs = append(errs, "workers must be at least 1")
	}

	// Validate TLS settings: if Listen is set, TLS cert and key are required
	if cfg.Server.Listen != "" {
		if cfg.Server.TLSCert == "" {
			errs = append(errs, "tls_cert is required when listen is set")
		}
		if cfg.Server.TLSKey == "" {
			errs = append(errs, "tls_key is required when listen is set")
		}
		// Validate certificate file exists
		if cfg.Server.TLSCert != "" {
			if _, err := os.Stat(cfg.Server.TLSCert); os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("tls_cert file not found: %s", cfg.Server.TLSCert))
			}
		}
		// Validate key file exists
		if cfg.Server.TLSKey != "" {
			if _, err := os.Stat(cfg.Server.TLSKey); os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("tls_key file not found: %s", cfg.Server.TLSKey))
			}
		}
	}

	// Validate timeouts
	if cfg.Timeouts.Shutdown < 0 {
		errs = append(errs, "shutdown timeout must be non-negative")
	}
	if cfg.Timeouts.HealthCheck < 0 {
		errs = append(errs, "health_check timeout must be non-negative")
	}
	if cfg.Timeouts.SnapshotDownload < 0 {
		errs = append(errs, "snapshot_download timeout must be non-negative")
	}

	// Validate snapshot
	if cfg.Snapshot.MaxRetries < 0 {
		errs = append(errs, "max_retries must be non-negative")
	}
	if cfg.Snapshot.RetryDelay < 0 {
		errs = append(errs, "retry_delay must be non-negative")
	}
	if cfg.Snapshot.CacheTTL < 0 {
		errs = append(errs, "cache_ttl must be non-negative")
	}

	// Validate network ports
	if cfg.Network.PortOffset < 0 {
		errs = append(errs, "port_offset must be non-negative")
	}
	if cfg.Network.BaseRPCPort < 1 || cfg.Network.BaseRPCPort > 65535 {
		errs = append(errs, "base_rpc_port must be between 1 and 65535")
	}
	if cfg.Network.BaseP2PPort < 1 || cfg.Network.BaseP2PPort > 65535 {
		errs = append(errs, "base_p2p_port must be between 1 and 65535")
	}
	if cfg.Network.BaseRESTPort < 1 || cfg.Network.BaseRESTPort > 65535 {
		errs = append(errs, "base_rest_port must be between 1 and 65535")
	}
	if cfg.Network.BaseGRPCPort < 1 || cfg.Network.BaseGRPCPort > 65535 {
		errs = append(errs, "base_grpc_port must be between 1 and 65535")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
