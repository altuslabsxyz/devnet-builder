package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

func main() {
	// Enable color output
	color.NoColor = false

	// Initialize root command
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
