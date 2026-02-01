// internal/tui/components/table_test.go
package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableModel_View_Headers(t *testing.T) {
	m := NewTableModel([]string{"Name", "Status", "Nodes"})
	m.AddRow([]string{"my-devnet", "Running", "3"})
	view := m.View()
	assert.Contains(t, view, "Name")
	assert.Contains(t, view, "Status")
	assert.Contains(t, view, "Nodes")
}

func TestTableModel_View_Rows(t *testing.T) {
	m := NewTableModel([]string{"Name", "Status"})
	m.AddRow([]string{"devnet-1", "Running"})
	m.AddRow([]string{"devnet-2", "Stopped"})
	view := m.View()
	assert.Contains(t, view, "devnet-1")
	assert.Contains(t, view, "devnet-2")
	assert.Contains(t, view, "Running")
	assert.Contains(t, view, "Stopped")
}

func TestTableModel_EmptyTable(t *testing.T) {
	m := NewTableModel([]string{"Name"})
	view := m.View()
	assert.Contains(t, view, "Name")
	assert.Contains(t, view, "No data")
}
