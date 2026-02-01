// internal/tui/components/progress_test.go
package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressModel_Percentage(t *testing.T) {
	m := NewProgressModel("Downloading", 100, 50)
	assert.Equal(t, 0.5, m.Percentage())
}

func TestProgressModel_View_ShowsPercentage(t *testing.T) {
	m := NewProgressModel("Downloading", 100, 50)
	m.Width = 40
	view := m.View()
	assert.Contains(t, view, "50%")
	assert.Contains(t, view, "Downloading")
}

func TestProgressModel_View_ShowsSpeed(t *testing.T) {
	m := NewProgressModel("Downloading", 100, 50)
	m.Width = 60
	m.ShowSpeed = true
	m.Speed = 1024 * 1024 * 2.5 // 2.5 MB/s
	view := m.View()
	assert.Contains(t, view, "MB/s")
}

func TestProgressModel_View_ShowsETA(t *testing.T) {
	m := NewProgressModel("Downloading", 100*1024*1024, 50*1024*1024)
	m.Width = 80
	m.ShowETA = true
	m.Speed = 1024 * 1024 * 10 // 10 MB/s, ETA = 5s
	view := m.View()
	assert.Contains(t, view, "ETA")
}

func TestProgressModel_SetProgress(t *testing.T) {
	m := NewProgressModel("Downloading", 100, 0)
	m.SetProgress(75)
	assert.Equal(t, int64(75), m.Current)
	assert.Equal(t, 0.75, m.Percentage())
}
