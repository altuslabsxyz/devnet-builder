package output

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConfirmPrompt asks for user confirmation and returns true if confirmed.
func ConfirmPrompt(message string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s [y/N]: ", message)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// ConfirmPromptDefault asks for confirmation with a default value.
func ConfirmPromptDefault(message string, defaultYes bool) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	prompt := "[y/N]"
	if defaultYes {
		prompt = "[Y/n]"
	}

	fmt.Printf("%s %s: ", message, prompt)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	// Empty response uses default
	if response == "" {
		return defaultYes, nil
	}

	return response == "y" || response == "yes", nil
}

// StringPrompt asks for a string input.
func StringPrompt(message string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s: ", message)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return strings.TrimSpace(response), nil
}

// StringPromptDefault asks for a string input with a default value.
func StringPromptDefault(message, defaultValue string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s [%s]: ", message, defaultValue)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(response)
	if response == "" {
		return defaultValue, nil
	}

	return response, nil
}

// SelectPrompt asks user to select from a list of options.
func SelectPrompt(message string, options []string) (int, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(message)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}

	fmt.Print("Selection: ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return -1, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(response)
	var selection int
	if _, err := fmt.Sscanf(response, "%d", &selection); err != nil {
		return -1, fmt.Errorf("invalid selection: %s", response)
	}

	if selection < 1 || selection > len(options) {
		return -1, fmt.Errorf("selection out of range: %d", selection)
	}

	return selection - 1, nil
}
