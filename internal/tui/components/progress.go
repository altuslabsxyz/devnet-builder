// internal/tui/components/progress.go
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altuslabsxyz/devnet-builder/internal/tui"
)

// ProgressModel wraps bubbles progress with metadata
type ProgressModel struct {
	progress  progress.Model
	Label     string
	Current   int64
	Total     int64
	Width     int
	ShowSpeed bool
	ShowETA   bool
	Speed     float64 // bytes per second
}

// NewProgressModel creates a new progress bar
func NewProgressModel(label string, total, current int64) ProgressModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(30),
	)
	return ProgressModel{
		progress: p,
		Label:    label,
		Total:    total,
		Current:  current,
		Width:    50,
	}
}

// Init implements tea.Model
func (m ProgressModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width - 10
		if m.Width < 30 {
			m.Width = 30
		}
		m.progress.Width = m.Width - len(m.Label) - 20
	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model
func (m ProgressModel) View() string {
	var b strings.Builder

	// Label
	labelStyle := lipgloss.NewStyle().Foreground(tui.ColorInfo)
	b.WriteString(labelStyle.Render(m.Label))
	b.WriteString("  ")

	// Progress bar
	pct := m.Percentage()
	b.WriteString(m.progress.ViewAs(pct))

	// Percentage
	b.WriteString(fmt.Sprintf("  %3.0f%%", pct*100))

	// Speed (optional)
	if m.ShowSpeed && m.Speed > 0 {
		speedMB := m.Speed / (1024 * 1024)
		b.WriteString(fmt.Sprintf(" | %.1f MB/s", speedMB))
	}

	// ETA (optional)
	if m.ShowETA && m.Speed > 0 {
		remaining := float64(m.Total - m.Current)
		etaSeconds := remaining / m.Speed
		if etaSeconds < 60 {
			b.WriteString(fmt.Sprintf(" | ETA: %.0fs", etaSeconds))
		} else {
			b.WriteString(fmt.Sprintf(" | ETA: %.1fm", etaSeconds/60))
		}
	}

	return b.String()
}

// Percentage returns current progress as 0.0-1.0
func (m ProgressModel) Percentage() float64 {
	if m.Total <= 0 {
		return 0
	}
	return float64(m.Current) / float64(m.Total)
}

// SetProgress updates current progress
func (m *ProgressModel) SetProgress(current int64) {
	m.Current = current
}

// SetSpeed updates the speed for display
func (m *ProgressModel) SetSpeed(bytesPerSec float64) {
	m.Speed = bytesPerSec
}
