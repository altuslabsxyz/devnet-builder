// pkg/network/cosmos/broadcast.go
package cosmos

// BroadcastMode specifies how to broadcast a transaction.
type BroadcastMode string

const (
	BroadcastModeSync  BroadcastMode = "sync"
	BroadcastModeAsync BroadcastMode = "async"
)

// BroadcastRequest is the JSON-RPC request for broadcast_tx_sync.
type BroadcastRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      int               `json:"id"`
	Method  string            `json:"method"`
	Params  map[string]string `json:"params"`
}

// BroadcastResponse is the JSON-RPC response for broadcast.
type BroadcastResponse struct {
	Result struct {
		Code uint32 `json:"code"`
		Data string `json:"data"`
		Log  string `json:"log"`
		Hash string `json:"hash"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error,omitempty"`
}
