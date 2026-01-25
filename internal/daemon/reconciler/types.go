// Package reconciler implements state reconciliation for devnets.
// It compares desired state (from YAML) with current state (from store)
// and generates execution plans to reconcile differences.
package reconciler

import (
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
)

// ActionType represents the type of reconciliation action
type ActionType int

const (
	ActionCreate ActionType = iota
	ActionUpdate
	ActionDelete
	ActionScale
	ActionUpgrade
	ActionNoop
)

func (a ActionType) String() string {
	switch a {
	case ActionCreate:
		return "CREATE"
	case ActionUpdate:
		return "UPDATE"
	case ActionDelete:
		return "DELETE"
	case ActionScale:
		return "SCALE"
	case ActionUpgrade:
		return "UPGRADE"
	case ActionNoop:
		return "NOOP"
	default:
		return "UNKNOWN"
	}
}

// Action represents a single reconciliation action
type Action struct {
	Type        ActionType
	Resource    string // "devnet" or "node"
	Name        string
	Description string
	Details     map[string]interface{}
}

// Plan represents a reconciliation plan
type Plan struct {
	DevnetName string
	Actions    []Action
	CreatedAt  time.Time
}

// IsEmpty returns true if the plan has no actions
func (p *Plan) IsEmpty() bool {
	return len(p.Actions) == 0
}

// Summary returns a summary of the plan
func (p *Plan) Summary() (creates, updates, deletes int) {
	for _, a := range p.Actions {
		switch a.Type {
		case ActionCreate:
			creates++
		case ActionUpdate, ActionScale, ActionUpgrade:
			updates++
		case ActionDelete:
			deletes++
		}
	}
	return
}

// State represents current or desired state
type State struct {
	Devnets map[string]config.YAMLDevnet
}

// NewState creates an empty state
func NewState() *State {
	return &State{
		Devnets: make(map[string]config.YAMLDevnet),
	}
}
