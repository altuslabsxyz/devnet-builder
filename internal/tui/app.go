// internal/tui/app.go
package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// Run starts the TUI program with the given model
func Run(model tea.Model) error {
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunInline starts the TUI without alt screen (stays inline)
func RunInline(model tea.Model) error {
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}

// RunSimple renders the model once without interactivity (for non-TTY)
func RunSimple(model tea.Model) error {
	fmt.Println(model.View())
	return nil
}

// IsInteractive returns true if stdout is a TTY
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// RunAuto chooses between TUI and simple mode based on TTY
func RunAuto(model tea.Model) error {
	if IsInteractive() {
		return RunInline(model)
	}
	return RunSimple(model)
}
