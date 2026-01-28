// cmd/dvb/wizard.go
// Package main provides interactive wizard functionality for CLI commands.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
)

// WizardOptions holds the collected options from the provision wizard.
// Note: Only fields that can be transmitted to the daemon via proto are included.
// ChainID is auto-generated from devnet name, and binary/data dir are daemon-level config.
type WizardOptions struct {
	Name        string
	Network     string
	Validators  int
	FullNodes   int
	ForkNetwork string // Network to fork from (e.g., "mainnet", "testnet", ""). Empty means fresh genesis.
}

// RunProvisionWizard runs an interactive wizard to collect provision options.
// Returns nil if the user cancels.
func RunProvisionWizard() (*WizardOptions, error) {
	opts := &WizardOptions{}

	fmt.Println()
	fmt.Println("🚀 Devnet Provisioning Wizard")
	fmt.Println("─────────────────────────────────")
	fmt.Println()

	// 1. Devnet Name (required)
	namePrompt := promptui.Prompt{
		Label:    "Devnet name",
		Validate: validateNonEmpty,
	}
	name, err := namePrompt.Run()
	if err != nil {
		return nil, handlePromptError(err, "devnet name")
	}
	opts.Name = strings.TrimSpace(name)

	// 2. Network (select from available options)
	networkPrompt := promptui.Select{
		Label: "Network type",
		Items: []string{"stable", "cosmos"},
		Templates: &promptui.SelectTemplates{
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "✔ Network: {{ . | green }}",
		},
	}
	_, network, err := networkPrompt.Run()
	if err != nil {
		return nil, handlePromptError(err, "network type")
	}
	opts.Network = network

	// 2b. Fork from existing network?
	forkPrompt := promptui.Select{
		Label: "Genesis configuration",
		Items: []string{
			"Fresh genesis (start from scratch)",
			"Fork from existing network (mainnet/testnet state)",
		},
		Templates: &promptui.SelectTemplates{
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "✔ Genesis: {{ . | green }}",
		},
	}
	forkIdx, _, err := forkPrompt.Run()
	if err != nil {
		return nil, handlePromptError(err, "genesis configuration")
	}

	if forkIdx == 1 {
		// User wants to fork from an existing network
		forkNetworkPrompt := promptui.Select{
			Label: "Fork from which network",
			Items: []string{"mainnet", "testnet"},
			Templates: &promptui.SelectTemplates{
				Active:   "▸ {{ . | cyan }}",
				Inactive: "  {{ . }}",
				Selected: "✔ Fork from: {{ . | green }}",
			},
		}
		_, forkNetwork, err := forkNetworkPrompt.Run()
		if err != nil {
			return nil, handlePromptError(err, "fork network")
		}
		opts.ForkNetwork = forkNetwork
	}

	// 3. Number of Validators
	validatorsPrompt := promptui.Prompt{
		Label:    "Number of validators",
		Default:  "1",
		Validate: validatePositiveInt,
	}
	validatorsStr, err := validatorsPrompt.Run()
	if err != nil {
		return nil, handlePromptError(err, "validators")
	}
	opts.Validators, _ = strconv.Atoi(validatorsStr)

	// 4. Number of Full Nodes
	fullNodesPrompt := promptui.Prompt{
		Label:    "Number of full nodes",
		Default:  "0",
		Validate: validateNonNegativeInt,
	}
	fullNodesStr, err := fullNodesPrompt.Run()
	if err != nil {
		return nil, handlePromptError(err, "full nodes")
	}
	opts.FullNodes, _ = strconv.Atoi(fullNodesStr)

	// Summary and confirmation
	// Note: Chain ID is auto-generated from devnet name by the daemon.
	// Binary is built from source by default. Data dir is daemon-level config.
	fmt.Println()
	fmt.Println("─────────────────────────────────")
	fmt.Println("📋 Configuration Summary")
	fmt.Println("─────────────────────────────────")
	fmt.Printf("  Name:       %s\n", opts.Name)
	fmt.Printf("  Network:    %s\n", opts.Network)
	fmt.Printf("  Validators: %d\n", opts.Validators)
	fmt.Printf("  Full Nodes: %d\n", opts.FullNodes)
	if opts.ForkNetwork != "" {
		fmt.Printf("  Genesis:    fork from %s\n", opts.ForkNetwork)
	} else {
		fmt.Printf("  Genesis:    fresh (new chain)\n")
	}
	fmt.Printf("  Binary:     (build from source)\n")
	fmt.Println("─────────────────────────────────")
	fmt.Println()

	confirmPrompt := promptui.Select{
		Label: "Proceed with provisioning?",
		Items: []string{"Yes, provision now", "No, cancel"},
		Templates: &promptui.SelectTemplates{
			Active:   "▸ {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "{{ . }}",
		},
	}
	confirmIdx, _, err := confirmPrompt.Run()
	if err != nil {
		return nil, handlePromptError(err, "confirmation")
	}

	if confirmIdx != 0 {
		fmt.Println("Provisioning cancelled.")
		return nil, nil // User cancelled
	}

	return opts, nil
}

// ConfirmDestroy shows a confirmation prompt before destroying a devnet.
// Returns true if the user confirms destruction.
func ConfirmDestroy(devnetName string) (bool, error) {
	fmt.Println()
	fmt.Println("⚠️  Warning: Destroying a devnet is irreversible!")
	fmt.Printf("   This will stop all nodes and delete all data for '%s'.\n", devnetName)
	fmt.Println()

	// Require typing the devnet name for dangerous operation
	confirmPrompt := promptui.Prompt{
		Label: fmt.Sprintf("Type '%s' to confirm destruction", devnetName),
		Validate: func(input string) error {
			if strings.TrimSpace(input) != devnetName {
				return fmt.Errorf("name doesn't match")
			}
			return nil
		},
	}

	_, err := confirmPrompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt || err == promptui.ErrEOF {
			fmt.Println("Destruction cancelled.")
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Validation functions

func validateNonEmpty(input string) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("cannot be empty")
	}
	return nil
}

func validatePositiveInt(input string) error {
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if n < 1 {
		return fmt.Errorf("must be at least 1")
	}
	return nil
}

func validateNonNegativeInt(input string) error {
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if n < 0 {
		return fmt.Errorf("cannot be negative")
	}
	return nil
}

func validateFilePath(input string) error {
	path := strings.TrimSpace(input)
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", path)
	}
	return nil
}

func handlePromptError(err error, context string) error {
	if err == promptui.ErrInterrupt || err == promptui.ErrEOF {
		return fmt.Errorf("cancelled")
	}
	return fmt.Errorf("failed to get %s: %w", context, err)
}
