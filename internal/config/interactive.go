package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/manifoldco/promptui"
	"github.com/stablelabs/stable-devnet/internal/network"
	"golang.org/x/term"
)

// InteractiveSetup handles interactive configuration prompts.
type InteractiveSetup struct {
	homeDir  string
	writer   *ConfigWriter
	defaults *FileConfig
}

// NewInteractiveSetup creates a new InteractiveSetup for the given home directory.
func NewInteractiveSetup(homeDir string) *InteractiveSetup {
	return &InteractiveSetup{
		homeDir:  homeDir,
		writer:   NewConfigWriter(homeDir),
		defaults: &FileConfig{},
	}
}

// IsInteractive returns true if the terminal supports interactive input.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// ShouldPrompt returns true if interactive prompts should be shown.
// Returns true if: terminal is interactive AND config doesn't exist.
func (s *InteractiveSetup) ShouldPrompt() bool {
	return IsInteractive() && !s.writer.Exists()
}

// ConfigExists returns true if config.toml exists in homeDir.
func (s *InteractiveSetup) ConfigExists() bool {
	return s.writer.Exists()
}

// LoadDefaults loads existing config values to use as defaults in prompts.
func (s *InteractiveSetup) LoadDefaults() *FileConfig {
	if !s.writer.Exists() {
		return s.defaults
	}

	// Try to load existing config
	loader := NewConfigLoader(s.homeDir, "", nil)
	cfg, _, err := loader.LoadFileConfig()
	if err != nil {
		return s.defaults
	}

	s.defaults = cfg
	return cfg
}

// Run executes the interactive configuration flow.
// Returns the configured FileConfig or error if cancelled.
func (s *InteractiveSetup) Run() (*FileConfig, error) {
	cfg := s.LoadDefaults()

	fmt.Println()
	fmt.Println("Welcome to devnet-builder configuration!")
	fmt.Println("Press Ctrl+C at any time to cancel.")
	fmt.Println()

	// Prompt for blockchain network
	blockchain, err := s.promptBlockchainNetwork(cfg)
	if err != nil {
		return nil, err
	}
	cfg.BlockchainNetwork = &blockchain

	// Prompt for network source
	networkSource, err := s.promptNetworkSource(cfg)
	if err != nil {
		return nil, err
	}
	cfg.Network = &networkSource

	// Prompt for validators
	validators, err := s.promptValidators(cfg)
	if err != nil {
		return nil, err
	}
	cfg.Validators = &validators

	// Prompt for mode
	mode, err := s.promptMode(cfg)
	if err != nil {
		return nil, err
	}
	cfg.Mode = &mode

	// Prompt for version
	version, err := s.promptVersion(cfg)
	if err != nil {
		return nil, err
	}
	cfg.NetworkVersion = &version

	return cfg, nil
}

// RunWithDefaults returns a FileConfig with default values.
// Used when terminal is non-interactive.
func (s *InteractiveSetup) RunWithDefaults() *FileConfig {
	blockchain := "stable"
	networkSource := "mainnet"
	validators := 4
	mode := "docker"
	version := "latest"

	return &FileConfig{
		BlockchainNetwork: &blockchain,
		Network:           &networkSource,
		Validators:        &validators,
		Mode:              &mode,
		NetworkVersion:    &version,
	}
}

// WriteConfig writes the configuration to homeDir/config.toml.
func (s *InteractiveSetup) WriteConfig(cfg *FileConfig) error {
	return s.writer.Write(cfg)
}

// promptBlockchainNetwork prompts the user to select a blockchain network.
func (s *InteractiveSetup) promptBlockchainNetwork(cfg *FileConfig) (string, error) {
	// Get available networks from registry
	networks := network.List()
	if len(networks) == 0 {
		networks = []string{"stable"}
	}

	// Find default index
	defaultIdx := 0
	if cfg.BlockchainNetwork != nil {
		for i, n := range networks {
			if n == *cfg.BlockchainNetwork {
				defaultIdx = i
				break
			}
		}
	}

	prompt := promptui.Select{
		Label:     "Select blockchain network",
		Items:     networks,
		CursorPos: defaultIdx,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "✓ Blockchain: {{ . | green }}",
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return "", handlePromptError(err)
	}

	return result, nil
}

// promptNetworkSource prompts the user to select mainnet or testnet.
func (s *InteractiveSetup) promptNetworkSource(cfg *FileConfig) (string, error) {
	options := []string{"mainnet", "testnet"}

	defaultIdx := 0
	if cfg.Network != nil && *cfg.Network == "testnet" {
		defaultIdx = 1
	}

	prompt := promptui.Select{
		Label:     "Select network source",
		Items:     options,
		CursorPos: defaultIdx,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "✓ Network: {{ . | green }}",
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return "", handlePromptError(err)
	}

	return result, nil
}

// promptValidators prompts the user to enter validators count (1-4).
func (s *InteractiveSetup) promptValidators(cfg *FileConfig) (int, error) {
	defaultValue := "4"
	if cfg.Validators != nil {
		defaultValue = strconv.Itoa(*cfg.Validators)
	}

	validate := func(input string) error {
		val, err := strconv.Atoi(input)
		if err != nil {
			return fmt.Errorf("please enter a number")
		}
		if val < 1 || val > 4 {
			return fmt.Errorf("validators must be between 1 and 4")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Number of validators (1-4)",
		Default:  defaultValue,
		Validate: validate,
		Templates: &promptui.PromptTemplates{
			Prompt:  "{{ . }}: ",
			Valid:   "{{ . | green }}: ",
			Invalid: "{{ . | red }}: ",
			Success: "✓ Validators: ",
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return 0, handlePromptError(err)
	}

	val, _ := strconv.Atoi(result)
	return val, nil
}

// promptMode prompts the user to select docker or local mode.
func (s *InteractiveSetup) promptMode(cfg *FileConfig) (string, error) {
	options := []string{"docker", "local"}

	defaultIdx := 0
	if cfg.Mode != nil && *cfg.Mode == "local" {
		defaultIdx = 1
	}

	prompt := promptui.Select{
		Label:     "Select execution mode",
		Items:     options,
		CursorPos: defaultIdx,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "✓ Mode: {{ . | green }}",
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return "", handlePromptError(err)
	}

	return result, nil
}

// promptVersion prompts the user to enter network version.
func (s *InteractiveSetup) promptVersion(cfg *FileConfig) (string, error) {
	defaultValue := "latest"
	if cfg.NetworkVersion != nil && *cfg.NetworkVersion != "" {
		defaultValue = *cfg.NetworkVersion
	}

	prompt := promptui.Prompt{
		Label:   "Network version",
		Default: defaultValue,
		Templates: &promptui.PromptTemplates{
			Prompt:  "{{ . }}: ",
			Valid:   "{{ . | green }}: ",
			Invalid: "{{ . | red }}: ",
			Success: "✓ Version: ",
		},
	}

	result, err := prompt.Run()
	if err != nil {
		return "", handlePromptError(err)
	}

	if result == "" {
		result = "latest"
	}

	return result, nil
}

// handlePromptError converts promptui errors to user-friendly messages.
func handlePromptError(err error) error {
	if err == promptui.ErrInterrupt {
		return fmt.Errorf("configuration cancelled")
	}
	if err == promptui.ErrEOF {
		return fmt.Errorf("configuration cancelled (EOF)")
	}
	return err
}

// MissingFieldsError represents an error when required config fields are missing.
type MissingFieldsError struct {
	Fields []string
}

func (e *MissingFieldsError) Error() string {
	return fmt.Sprintf("missing required configuration: %v", e.Fields)
}

// RunPartial executes partial interactive configuration flow.
// Only prompts for fields that are nil in the provided config.
// Fields that already have values are preserved.
func (s *InteractiveSetup) RunPartial(cfg *FileConfig) (*FileConfig, error) {
	if cfg == nil {
		cfg = &FileConfig{}
	}

	// Check if any prompts are needed
	needsPrompt := cfg.BlockchainNetwork == nil || cfg.Network == nil ||
		cfg.Validators == nil || cfg.Mode == nil

	if needsPrompt && IsInteractive() {
		fmt.Println()
		fmt.Println("Some configuration values are missing. Please provide them:")
		fmt.Println("Press Ctrl+C at any time to cancel.")
		fmt.Println()
	}

	// Prompt for blockchain network if not set
	if cfg.BlockchainNetwork == nil {
		if !IsInteractive() {
			return nil, &MissingFieldsError{Fields: s.getMissingFields(cfg)}
		}
		blockchain, err := s.promptBlockchainNetwork(cfg)
		if err != nil {
			return nil, err
		}
		cfg.BlockchainNetwork = &blockchain
	}

	// Prompt for network source if not set
	if cfg.Network == nil {
		if !IsInteractive() {
			return nil, &MissingFieldsError{Fields: s.getMissingFields(cfg)}
		}
		networkSource, err := s.promptNetworkSource(cfg)
		if err != nil {
			return nil, err
		}
		cfg.Network = &networkSource
	}

	// Prompt for validators if not set
	if cfg.Validators == nil {
		if !IsInteractive() {
			return nil, &MissingFieldsError{Fields: s.getMissingFields(cfg)}
		}
		validators, err := s.promptValidators(cfg)
		if err != nil {
			return nil, err
		}
		cfg.Validators = &validators
	}

	// Prompt for mode if not set
	if cfg.Mode == nil {
		if !IsInteractive() {
			return nil, &MissingFieldsError{Fields: s.getMissingFields(cfg)}
		}
		mode, err := s.promptMode(cfg)
		if err != nil {
			return nil, err
		}
		cfg.Mode = &mode
	}

	// NetworkVersion is optional - use default if not set
	if cfg.NetworkVersion == nil {
		defaultVersion := "latest"
		cfg.NetworkVersion = &defaultVersion
	}

	return cfg, nil
}

// getMissingFields returns a list of field names that are nil in the config.
func (s *InteractiveSetup) getMissingFields(cfg *FileConfig) []string {
	var missing []string
	if cfg.BlockchainNetwork == nil {
		missing = append(missing, "blockchain_network")
	}
	if cfg.Network == nil {
		missing = append(missing, "network")
	}
	if cfg.Validators == nil {
		missing = append(missing, "validators")
	}
	if cfg.Mode == nil {
		missing = append(missing, "mode")
	}
	return missing
}

// CheckRequiredFields validates that all required fields are set in the config.
// Returns MissingFieldsError if any required fields are nil.
// This is useful for non-interactive validation before proceeding.
func (s *InteractiveSetup) CheckRequiredFields(cfg *FileConfig) error {
	if cfg == nil {
		return &MissingFieldsError{Fields: []string{"blockchain_network", "network", "validators", "mode"}}
	}

	missing := s.getMissingFields(cfg)
	if len(missing) > 0 {
		return &MissingFieldsError{Fields: missing}
	}
	return nil
}

// HasAllRequiredFields returns true if all required fields are set.
func (s *InteractiveSetup) HasAllRequiredFields(cfg *FileConfig) bool {
	return s.CheckRequiredFields(cfg) == nil
}
