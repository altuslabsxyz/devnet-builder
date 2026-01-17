package nodeconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/types"
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
func (e *ConfigEditor) SetPorts(nodeIndex int) error {
	ports := GetPortConfigForNode(nodeIndex)

	// config.toml settings
	configPath := e.ConfigPath()

	// P2P laddr - in [p2p] section
	if err := e.setP2PLaddr(configPath, ports.P2P); err != nil {
		return fmt.Errorf("failed to set p2p laddr: %w", err)
	}

	// RPC laddr - in [rpc] section
	if err := e.setRPCLaddr(configPath, ports.RPC); err != nil {
		return fmt.Errorf("failed to set rpc laddr: %w", err)
	}

	// Proxy app
	if err := e.setConfigValue(configPath, "proxy_app", fmt.Sprintf("tcp://127.0.0.1:%d", ports.Proxy)); err != nil {
		return fmt.Errorf("failed to set proxy_app: %w", err)
	}

	// pprof
	if err := e.setConfigValue(configPath, "pprof_laddr", fmt.Sprintf("localhost:%d", ports.PProf)); err != nil {
		return fmt.Errorf("failed to set pprof_laddr: %w", err)
	}

	// app.toml settings
	appPath := e.AppConfigPath()

	// gRPC address
	if err := e.setGRPCAddress(appPath, ports.GRPC); err != nil {
		return fmt.Errorf("failed to set grpc address: %w", err)
	}

	// API address
	if err := e.setAPIAddress(appPath, ports.API); err != nil {
		return fmt.Errorf("failed to set api address: %w", err)
	}

	// EVM JSON-RPC address
	if err := e.setEVMRPCAddress(appPath, ports.EVMRPC); err != nil {
		return fmt.Errorf("failed to set evm rpc address: %w", err)
	}

	// EVM WebSocket address
	if err := e.setEVMWSAddress(appPath, ports.EVMWS); err != nil {
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
func (e *ConfigEditor) setP2PLaddr(filePath string, port int) error {
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
			lines[i] = fmt.Sprintf(`laddr = "tcp://0.0.0.0:%d"`, port)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// setRPCLaddr sets the RPC laddr specifically in the [rpc] section.
func (e *ConfigEditor) setRPCLaddr(filePath string, port int) error {
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
			lines[i] = fmt.Sprintf(`laddr = "tcp://0.0.0.0:%d"`, port)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// setGRPCAddress sets the gRPC address in app.toml.
func (e *ConfigEditor) setGRPCAddress(filePath string, port int) error {
	return e.setSectionValue(filePath, "grpc", "address", fmt.Sprintf("0.0.0.0:%d", port))
}

// setAPIAddress sets the API address in app.toml.
func (e *ConfigEditor) setAPIAddress(filePath string, port int) error {
	return e.setSectionValue(filePath, "api", "address", fmt.Sprintf("tcp://0.0.0.0:%d", port))
}

// setEVMRPCAddress sets the EVM JSON-RPC address in app.toml.
func (e *ConfigEditor) setEVMRPCAddress(filePath string, port int) error {
	return e.setSectionValue(filePath, "json-rpc", "address", fmt.Sprintf("0.0.0.0:%d", port))
}

// setEVMWSAddress sets the EVM WebSocket address in app.toml.
func (e *ConfigEditor) setEVMWSAddress(filePath string, port int) error {
	return e.setSectionValue(filePath, "json-rpc", "ws-address", fmt.Sprintf("0.0.0.0:%d", port))
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
func ConfigureNode(nodeDir string, nodeIndex int, peers string, isNode0 bool, logger *output.Logger) error {
	editor := NewConfigEditor(nodeDir, logger)

	// Set ports based on index
	if err := editor.SetPorts(nodeIndex); err != nil {
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
