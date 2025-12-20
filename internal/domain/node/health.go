package node

import "time"

// SyncStatus represents the synchronization state.
type SyncStatus string

const (
	SyncStatusUnknown  SyncStatus = "unknown"
	SyncStatusSyncing  SyncStatus = "syncing"
	SyncStatusCaughtUp SyncStatus = "caught_up"
)

// Health represents the health status of a node.
type Health struct {
	IsRunning   bool       `json:"is_running"`
	BlockHeight int64      `json:"block_height"`
	SyncStatus  SyncStatus `json:"sync_status"`
	CatchingUp  bool       `json:"catching_up"`
	PeerCount   int        `json:"peer_count"`
	LastChecked time.Time  `json:"last_checked"`
	Error       string     `json:"error,omitempty"`
}

// NewHealth creates a new Health with default values.
func NewHealth() Health {
	return Health{
		SyncStatus:  SyncStatusUnknown,
		LastChecked: time.Now(),
	}
}

// SetRunning updates health for a running node.
func (h *Health) SetRunning(height int64, catchingUp bool, peerCount int) {
	h.IsRunning = true
	h.BlockHeight = height
	h.CatchingUp = catchingUp
	h.PeerCount = peerCount
	h.LastChecked = time.Now()
	h.Error = ""

	if catchingUp {
		h.SyncStatus = SyncStatusSyncing
	} else {
		h.SyncStatus = SyncStatusCaughtUp
	}
}

// SetStopped updates health for a stopped node.
func (h *Health) SetStopped() {
	h.IsRunning = false
	h.SyncStatus = SyncStatusUnknown
	h.LastChecked = time.Now()
}

// SetError updates health with an error.
func (h *Health) SetError(err error) {
	h.IsRunning = false
	h.SyncStatus = SyncStatusUnknown
	h.LastChecked = time.Now()
	if err != nil {
		h.Error = err.Error()
	}
}

// IsHealthy returns true if the node is running and not catching up.
func (h *Health) IsHealthy() bool {
	return h.IsRunning && h.SyncStatus == SyncStatusCaughtUp
}

// IsSyncing returns true if the node is syncing.
func (h *Health) IsSyncing() bool {
	return h.IsRunning && h.CatchingUp
}
