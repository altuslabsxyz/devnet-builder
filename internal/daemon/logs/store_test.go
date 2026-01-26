package logs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLogStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "logs.db")

	store, err := NewLogStore(dbPath)
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestLogStore_StoreAndRetrieve(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create test entries
	entries := []LogEntry{
		{
			Timestamp: time.Now().Add(-2 * time.Minute),
			NodeIndex: 0,
			Level:     "info",
			Module:    "server",
			Message:   "Starting server",
		},
		{
			Timestamp: time.Now().Add(-1 * time.Minute),
			NodeIndex: 0,
			Level:     "debug",
			Module:    "consensus",
			Message:   "Processing block",
		},
		{
			Timestamp: time.Now(),
			NodeIndex: 1,
			Level:     "error",
			Module:    "network",
			Message:   "Connection failed",
		},
	}

	// Store entries
	for _, entry := range entries {
		if err := store.Store(ctx, entry); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	// Retrieve all entries
	query := LogQuery{
		Limit: 100,
	}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Query() returned %d entries, want 3", len(results))
	}
}

func TestLogStore_QueryByNode(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store entries for different nodes
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "info", Message: "node0 msg1"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "info", Message: "node0 msg2"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 1, Level: "info", Message: "node1 msg1"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 2, Level: "info", Message: "node2 msg1"})

	// Query for node 0 only
	nodeIndex := 0
	query := LogQuery{
		NodeIndex: &nodeIndex,
		Limit:     100,
	}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Query(NodeIndex=0) returned %d entries, want 2", len(results))
	}

	for _, entry := range results {
		if entry.NodeIndex != 0 {
			t.Errorf("Query returned entry with NodeIndex=%d, want 0", entry.NodeIndex)
		}
	}
}

func TestLogStore_QueryByLevel(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store entries with different levels
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "debug", Message: "debug msg"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "info", Message: "info msg"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "warn", Message: "warn msg"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "error", Message: "error msg"})

	// Query for error level only
	query := LogQuery{
		Level: "error",
		Limit: 100,
	}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Query(Level=error) returned %d entries, want 1", len(results))
	}

	if results[0].Level != "error" {
		t.Errorf("Query returned entry with Level=%q, want 'error'", results[0].Level)
	}
}

func TestLogStore_QueryByTimeRange(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	now := time.Now()

	// Store entries at different times
	store.Store(ctx, LogEntry{Timestamp: now.Add(-3 * time.Hour), NodeIndex: 0, Level: "info", Message: "old msg"})
	store.Store(ctx, LogEntry{Timestamp: now.Add(-1 * time.Hour), NodeIndex: 0, Level: "info", Message: "recent msg"})
	store.Store(ctx, LogEntry{Timestamp: now, NodeIndex: 0, Level: "info", Message: "current msg"})

	// Query for last 2 hours
	since := now.Add(-2 * time.Hour)
	query := LogQuery{
		Since: &since,
		Limit: 100,
	}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Query(Since=2h ago) returned %d entries, want 2", len(results))
	}
}

func TestLogStore_QueryLimit(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store 10 entries
	for i := 0; i < 10; i++ {
		store.Store(ctx, LogEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			NodeIndex: 0,
			Level:     "info",
			Message:   "msg",
		})
	}

	// Query with limit 5
	query := LogQuery{
		Limit: 5,
	}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 5 {
		t.Errorf("Query(Limit=5) returned %d entries, want 5", len(results))
	}
}

func TestLogStore_QueryContains(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store entries with different messages
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "info", Message: "server starting on port 8080"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "info", Message: "database connected"})
	store.Store(ctx, LogEntry{Timestamp: time.Now(), NodeIndex: 0, Level: "error", Message: "connection timeout"})

	// Query for messages containing "connect"
	query := LogQuery{
		Contains: "connect",
		Limit:    100,
	}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Query(Contains='connect') returned %d entries, want 2", len(results))
	}
}

func TestLogStore_Prune(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now()

	// Store entries at different times
	store.Store(ctx, LogEntry{Timestamp: now.Add(-48 * time.Hour), NodeIndex: 0, Level: "info", Message: "very old"})
	store.Store(ctx, LogEntry{Timestamp: now.Add(-25 * time.Hour), NodeIndex: 0, Level: "info", Message: "old"})
	store.Store(ctx, LogEntry{Timestamp: now.Add(-1 * time.Hour), NodeIndex: 0, Level: "info", Message: "recent"})
	store.Store(ctx, LogEntry{Timestamp: now, NodeIndex: 0, Level: "info", Message: "current"})

	// Prune entries older than 24 hours
	cutoff := now.Add(-24 * time.Hour)
	pruned, err := store.Prune(ctx, cutoff)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}

	if pruned != 2 {
		t.Errorf("Prune() returned %d pruned, want 2", pruned)
	}

	// Verify only recent entries remain
	query := LogQuery{Limit: 100}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("After prune, Query() returned %d entries, want 2", len(results))
	}
}

func TestLogStore_StoreBatch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create batch of entries
	entries := make([]LogEntry, 100)
	for i := 0; i < 100; i++ {
		entries[i] = LogEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
			NodeIndex: i % 4,
			Level:     "info",
			Message:   "batch message",
		}
	}

	// Store batch
	if err := store.StoreBatch(ctx, entries); err != nil {
		t.Fatalf("StoreBatch() error = %v", err)
	}

	// Verify all entries were stored
	query := LogQuery{Limit: 200}
	results, err := store.Query(ctx, query)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(results) != 100 {
		t.Errorf("After StoreBatch, Query() returned %d entries, want 100", len(results))
	}
}

func TestLogStore_Count(t *testing.T) {
	dir := t.TempDir()
	store, err := NewLogStore(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store some entries
	for i := 0; i < 5; i++ {
		store.Store(ctx, LogEntry{
			Timestamp: time.Now(),
			NodeIndex: 0,
			Level:     "info",
			Message:   "msg",
		})
	}

	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 5 {
		t.Errorf("Count() = %d, want 5", count)
	}
}
