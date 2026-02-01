// internal/tui/components/steplist_test.go
package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStepList_AddStep(t *testing.T) {
	m := NewStepListModel()
	m.AddStep("Build binary")
	m.AddStep("Download snapshot")
	assert.Len(t, m.Steps, 2)
}

func TestStepList_View_PendingStep(t *testing.T) {
	m := NewStepListModel()
	m.AddStep("Build binary")
	view := m.View()
	assert.Contains(t, view, "Build binary")
	assert.Contains(t, view, "  ") // pending has space prefix
}

func TestStepList_View_RunningStep(t *testing.T) {
	m := NewStepListModel()
	m.AddStep("Build binary")
	m.SetStatus(0, StepRunning)
	view := m.View()
	assert.Contains(t, view, "→")
}

func TestStepList_View_CompletedStep(t *testing.T) {
	m := NewStepListModel()
	m.AddStep("Build binary")
	m.SetStatus(0, StepCompleted)
	view := m.View()
	assert.Contains(t, view, "✓")
}

func TestStepList_View_FailedStep(t *testing.T) {
	m := NewStepListModel()
	m.AddStep("Build binary")
	m.SetStatus(0, StepFailed)
	view := m.View()
	assert.Contains(t, view, "✗")
}

func TestStepList_SetDetail(t *testing.T) {
	m := NewStepListModel()
	m.AddStep("Download snapshot")
	m.SetDetail(0, "from cache")
	view := m.View()
	assert.Contains(t, view, "from cache")
}
