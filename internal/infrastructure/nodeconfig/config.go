package nodeconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types"
)

// PortConfig is an alias to the canonical types.PortConfig.
//
// Deprecated: Use types.PortConfig directly.
type PortConfig = types.PortConfig

// GetPortConfigForNode returns the port configuration for a node at the given index.
//
// Deprecated: Use types.PortConfigForNode() directly.
func GetPortConfigForNode(index int) PortConfig {
	return types.PortConfigForNode(index)
}

// ConfigEditor modifies config.toml and app.toml files.
type ConfigEditor struct {
	nodeDir string
	logger  *output.Logger
}

// NewConfigEditor creates a new ConfigEditor.
func NewConfigEditor(nodeDir string, logger *output.Logger) *ConfigEditor {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &ConfigEditor{
		nodeDir: nodeDir,
		logger:  logger,
	}
}

// ConfigPath returns the path to config.toml.
func (e *ConfigEditor) ConfigPath() string {
	return filepath.Join(e.nodeDir, "config", "config.toml")
}

// AppConfigPath returns the path to app.toml.
func (e *ConfigEditor) AppConfigPath() string {
	return filepath.Join(e.nodeDir, "config", "app.toml")
}

// SetPersistentPeers sets the persistent_peers in config.toml.
func (e *ConfigEditor) SetPersistentPeers(peers string) error {
	return e.setConfigValue(e.ConfigPath(), "persistent_peers", peers)
}

// SetPorts configures all port-related settings based on node index.
//
// Deprecated: Use SetPortsWithHost instead for loopback subnet support.
func (e *ConfigEditor) SetPorts(nodeIndex int) error {
	return e.SetPortsWithHost(nodeIndex, "")
}

// SetPortsWithHost configures all port-related settings with a specific host IP.
// If host is empty, it defaults to "0.0.0.0" for backward compatibility.
// For loopback subnet mode, pass the node's assigned IP (e.g., "127.0.42.1").
func (e *ConfigEditor) SetPortsWithHost(nodeIndex int, host string) error {
	ports := GetPortConfigForNode(nodeIndex)

	// Default to 0.0.0.0 for backward compatibility
	if host == "" {
		host = "0.0.0.0"
	}

	// config.toml settings
	configPath := e.ConfigPath()

	// P2P laddr - in [p2p] section
	if err := e.setP2PLaddrWithHost(configPath, ports.P2P, host); err != nil {
		return fmt.Errorf("failed to set p2p laddr: %w", err)
	}

	// RPC laddr - in [rpc] section
	if err := e.setRPCLaddrWithHost(configPath, ports.RPC, host); err != nil {
		return fmt.Errorf("failed to set rpc laddr: %w", err)
	}

	// Proxy app - uses the node's IP for ABCI connections
	if err := e.setConfigValue(configPath, "proxy_app", fmt.Sprintf("tcp://%s:%d", host, ports.Proxy)); err != nil {
		return fmt.Errorf("failed to set proxy_app: %w", err)
	}

	// pprof - bind to node's IP
	if err := e.setConfigValue(configPath, "pprof_laddr", fmt.Sprintf("%s:%d", host, ports.PProf)); err != nil {
		return fmt.Errorf("failed to set pprof_laddr: %w", err)
	}

	// app.toml settings
	appPath := e.AppConfigPath()

	// gRPC address
	if err := e.setGRPCAddressWithHost(appPath, ports.GRPC, host); err != nil {
		return fmt.Errorf("failed to set grpc address: %w", err)
	}

	// API address
	if err := e.setAPIAddressWithHost(appPath, ports.API, host); err != nil {
		return fmt.Errorf("failed to set api address: %w", err)
	}

	// EVM JSON-RPC address
	if err := e.setEVMRPCAddressWithHost(appPath, ports.EVMRPC, host); err != nil {
		return fmt.Errorf("failed to set evm rpc address: %w", err)
	}

	// EVM WebSocket address
	if err := e.setEVMWSAddressWithHost(appPath, ports.EVMWS, host); err != nil {
		return fmt.Errorf("failed to set evm ws address: %w", err)
	}

	return nil
}

// EnableNode0Services enables API, gRPC, and JSON-RPC services (for node0).
func (e *ConfigEditor) EnableNode0Services() error {
	appPath := e.AppConfigPath()

	// Enable API
	if err := e.setAppConfigBool(appPath, "api", "enable", true); err != nil {
		return fmt.Errorf("failed to enable api: %w", err)
	}

	// Enable gRPC
	if err := e.setAppConfigBool(appPath, "grpc", "enable", true); err != nil {
		return fmt.Errorf("failed to enable grpc: %w", err)
	}

	// Enable JSON-RPC (EVM)
	if err := e.setAppConfigBool(appPath, "json-rpc", "enable", true); err != nil {
		return fmt.Errorf("failed to enable json-rpc: %w", err)
	}

	// Enable CORS
	if err := e.setAppConfigBool(appPath, "api", "enabled-unsafe-cors", true); err != nil {
		return fmt.Errorf("failed to enable api cors: %w", err)
	}

	return nil
}

// DisableServices disables API, gRPC, and JSON-RPC services (for node1-3).
func (e *ConfigEditor) DisableServices() error {
	appPath := e.AppConfigPath()

	// Disable API
	if err := e.setAppConfigBool(appPath, "api", "enable", false); err != nil {
		return fmt.Errorf("failed to disable api: %w", err)
	}

	// Disable gRPC
	if err := e.setAppConfigBool(appPath, "grpc", "enable", false); err != nil {
		return fmt.Errorf("failed to disable grpc: %w", err)
	}

	// Disable JSON-RPC
	if err := e.setAppConfigBool(appPath, "json-rpc", "enable", false); err != nil {
		return fmt.Errorf("failed to disable json-rpc: %w", err)
	}

	return nil
}

// SetConsensusParams configures fast consensus parameters for devnet.
func (e *ConfigEditor) SetConsensusParams() error {
	configPath := e.ConfigPath()

	// Fast block times
	if err := e.setConfigValue(configPath, "timeout_propose", "1s"); err != nil {
		return err
	}
	if err := e.setConfigValue(configPath, "timeout_prevote", "500ms"); err != nil {
		return err
	}
	if err := e.setConfigValue(configPath, "timeout_precommit", "500ms"); err != nil {
		return err
	}
	if err := e.setConfigValue(configPath, "timeout_commit", "1s"); err != nil {
		return err
	}

	return nil
}

// SetP2PLocalDevnet configures P2P settings for local devnet (allows localhost connections).
func (e *ConfigEditor) SetP2PLocalDevnet() error {
	configPath := e.ConfigPath()

	// Allow non-routable addresses (localhost)
	if err := e.setP2PConfigBool(configPath, "addr_book_strict", false); err != nil {
		return err
	}

	// Allow multiple peers from same IP
	if err := e.setP2PConfigBool(configPath, "allow_duplicate_ip", true); err != nil {
		return err
	}

	return nil
}

// setP2PConfigBool sets a boolean value in the [p2p] section.
func (e *ConfigEditor) setP2PConfigBool(filePath, key string, value bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	inP2PSection := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[p2p]" {
			inP2PSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[p2p]" {
			inP2PSection = false
		}
		if inP2PSection && strings.HasPrefix(trimmed, key) {
			lines[i] = fmt.Sprintf("%s = %t", key, value)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// SetMoniker sets the node moniker in config.toml.
func (e *ConfigEditor) SetMoniker(moniker string) error {
	return e.setConfigValue(e.ConfigPath(), "moniker", moniker)
}

// setConfigValue sets a key=value in a TOML file (line-by-line replacement).
// Only replaces exact key matches (not keys that start with the same prefix).
func (e *ConfigEditor) setConfigValue(filePath, key, value string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	keyPattern := regexp.MustCompile(fmt.Sprintf(`^(\s*)%s\s*=`, regexp.QuoteMeta(key)))

	for i, line := range lines {
		if keyPattern.MatchString(line) {
			// Preserve leading whitespace
			match := keyPattern.FindStringSubmatch(line)
			if len(match) > 1 {
				lines[i] = fmt.Sprintf(`%s%s = "%s"`, match[1], key, value)
			}
			break // Only replace first occurrence
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// setP2PLaddr sets the P2P laddr specifically in the [p2p] section.
//
// Deprecated: Use setP2PLaddrWithHost instead.
func (e *ConfigEditor) setP2PLaddr(filePath string, port int) error {
	return e.setP2PLaddrWithHost(filePath, port, "0.0.0.0")
}

// setP2PLaddrWithHost sets the P2P laddr specifically in the [p2p] section with a specific host.
func (e *ConfigEditor) setP2PLaddrWithHost(filePath string, port int, host string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	inP2PSection := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[p2p]" {
			inP2PSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[p2p]" {
			inP2PSection = false
		}
		if inP2PSection && strings.HasPrefix(trimmed, "laddr") {
			lines[i] = fmt.Sprintf(`laddr = "tcp://%s:%d"`, host, port)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// setRPCLaddr sets the RPC laddr specifically in the [rpc] section.
//
// Deprecated: Use setRPCLaddrWithHost instead.
func (e *ConfigEditor) setRPCLaddr(filePath string, port int) error {
	return e.setRPCLaddrWithHost(filePath, port, "0.0.0.0")
}

// setRPCLaddrWithHost sets the RPC laddr specifically in the [rpc] section with a specific host.
func (e *ConfigEditor) setRPCLaddrWithHost(filePath string, port int, host string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Find [rpc] section and set laddr
	lines := strings.Split(string(content), "\n")
	inRPCSection := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[rpc]" {
			inRPCSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != "[rpc]" {
			inRPCSection = false
		}
		if inRPCSection && strings.HasPrefix(trimmed, "laddr") {
			lines[i] = fmt.Sprintf(`laddr = "tcp://%s:%d"`, host, port)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// setGRPCAddress sets the gRPC address in app.toml.
//
// Deprecated: Use setGRPCAddressWithHost instead.
func (e *ConfigEditor) setGRPCAddress(filePath string, port int) error {
	return e.setGRPCAddressWithHost(filePath, port, "0.0.0.0")
}

// setGRPCAddressWithHost sets the gRPC address in app.toml with a specific host.
func (e *ConfigEditor) setGRPCAddressWithHost(filePath string, port int, host string) error {
	return e.setSectionValue(filePath, "grpc", "address", fmt.Sprintf("%s:%d", host, port))
}

// setAPIAddress sets the API address in app.toml.
//
// Deprecated: Use setAPIAddressWithHost instead.
func (e *ConfigEditor) setAPIAddress(filePath string, port int) error {
	return e.setAPIAddressWithHost(filePath, port, "0.0.0.0")
}

// setAPIAddressWithHost sets the API address in app.toml with a specific host.
func (e *ConfigEditor) setAPIAddressWithHost(filePath string, port int, host string) error {
	return e.setSectionValue(filePath, "api", "address", fmt.Sprintf("tcp://%s:%d", host, port))
}

// setEVMRPCAddress sets the EVM JSON-RPC address in app.toml.
//
// Deprecated: Use setEVMRPCAddressWithHost instead.
func (e *ConfigEditor) setEVMRPCAddress(filePath string, port int) error {
	return e.setEVMRPCAddressWithHost(filePath, port, "0.0.0.0")
}

// setEVMRPCAddressWithHost sets the EVM JSON-RPC address in app.toml with a specific host.
func (e *ConfigEditor) setEVMRPCAddressWithHost(filePath string, port int, host string) error {
	return e.setSectionValue(filePath, "json-rpc", "address", fmt.Sprintf("%s:%d", host, port))
}

// setEVMWSAddress sets the EVM WebSocket address in app.toml.
//
// Deprecated: Use setEVMWSAddressWithHost instead.
func (e *ConfigEditor) setEVMWSAddress(filePath string, port int) error {
	return e.setEVMWSAddressWithHost(filePath, port, "0.0.0.0")
}

// setEVMWSAddressWithHost sets the EVM WebSocket address in app.toml with a specific host.
func (e *ConfigEditor) setEVMWSAddressWithHost(filePath string, port int, host string) error {
	return e.setSectionValue(filePath, "json-rpc", "ws-address", fmt.Sprintf("%s:%d", host, port))
}

// setSectionValue sets a value within a specific TOML section.
func (e *ConfigEditor) setSectionValue(filePath, section, key, value string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	inSection := false
	sectionHeader := fmt.Sprintf("[%s]", section)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sectionHeader {
			inSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != sectionHeader {
			inSection = false
		}
		if inSection && strings.HasPrefix(trimmed, key+" ") || (inSection && strings.HasPrefix(trimmed, key+"=")) {
			lines[i] = fmt.Sprintf(`%s = "%s"`, key, value)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// setAppConfigBool sets a boolean value within a specific TOML section.
func (e *ConfigEditor) setAppConfigBool(filePath, section, key string, value bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	inSection := false
	sectionHeader := fmt.Sprintf("[%s]", section)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == sectionHeader {
			inSection = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && trimmed != sectionHeader {
			inSection = false
		}
		if inSection && (strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=")) {
			lines[i] = fmt.Sprintf(`%s = %t`, key, value)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// ConfigureNode applies all necessary configuration for a node.
//
// Deprecated: Use ConfigureNodeWithHost instead for loopback subnet support.
func ConfigureNode(nodeDir string, nodeIndex int, peers string, isNode0 bool, logger *output.Logger) error {
	return ConfigureNodeWithHost(nodeDir, nodeIndex, peers, isNode0, "", logger)
}

// ConfigureNodeWithHost applies all necessary configuration for a node with a specific host IP.
// If host is empty, it defaults to "0.0.0.0" for backward compatibility.
// For loopback subnet mode, pass the node's assigned IP (e.g., "127.0.42.1").
func ConfigureNodeWithHost(nodeDir string, nodeIndex int, peers string, isNode0 bool, host string, logger *output.Logger) error {
	editor := NewConfigEditor(nodeDir, logger)

	// Set ports based on index with specific host
	if err := editor.SetPortsWithHost(nodeIndex, host); err != nil {
		return fmt.Errorf("failed to set ports: %w", err)
	}

	// Set persistent peers
	if err := editor.SetPersistentPeers(peers); err != nil {
		return fmt.Errorf("failed to set persistent peers: %w", err)
	}

	// Set moniker
	if err := editor.SetMoniker(fmt.Sprintf("node%d", nodeIndex)); err != nil {
		return fmt.Errorf("failed to set moniker: %w", err)
	}

	// Configure P2P for local devnet (allow localhost connections)
	if err := editor.SetP2PLocalDevnet(); err != nil {
		return fmt.Errorf("failed to set P2P local devnet config: %w", err)
	}

	// Configure services
	if isNode0 {
		if err := editor.EnableNode0Services(); err != nil {
			return fmt.Errorf("failed to enable services: %w", err)
		}
	} else {
		// For other nodes, we can still leave services enabled but on different ports
		// This is useful for testing
	}

	// Set fast consensus params
	if err := editor.SetConsensusParams(); err != nil {
		return fmt.Errorf("failed to set consensus params: %w", err)
	}

	return nil
}
