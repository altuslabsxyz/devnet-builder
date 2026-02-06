// internal/tui/components/table.go
package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/altuslabsxyz/devnet-builder/internal/tui"
)

// TableModel wraps bubbles table
type TableModel struct {
	table   table.Model
	headers []string
	rows    [][]string
	Width   int
	Height  int
}

// NewTableModel creates a table with headers
func NewTableModel(headers []string) TableModel {
	columns := make([]table.Column, len(headers))
	for i, h := range headers {
		columns[i] = table.Column{Title: h, Width: 20}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(tui.ColorMuted).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return TableModel{
		table:   t,
		headers: headers,
		rows:    [][]string{},
		Width:   80,
		Height:  10,
	}
}

// AddRow adds a row to the table
func (m *TableModel) AddRow(row []string) {
	m.rows = append(m.rows, row)
	m.updateTableRows()
}

// SetRows replaces all rows
func (m *TableModel) SetRows(rows [][]string) {
	m.rows = rows
	m.updateTableRows()
}

func (m *TableModel) updateTableRows() {
	tableRows := make([]table.Row, len(m.rows))
	for i, row := range m.rows {
		tableRows[i] = table.Row(row)
	}
	m.table.SetRows(tableRows)
}

// Init implements tea.Model
func (m TableModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m TableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		m.Width = wsm.Width - 4
		m.table.SetWidth(m.Width)
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View implements tea.Model
func (m TableModel) View() string {
	if len(m.rows) == 0 {
		// Show headers with "No data" message
		var b strings.Builder
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorInfo)
		b.WriteString(headerStyle.Render(strings.Join(m.headers, "  ")))
		b.WriteString("\n")
		b.WriteString(tui.MutedStyle.Render("No data"))
		return b.String()
	}
	return m.table.View()
}
