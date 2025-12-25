package export

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHeightResolver(t *testing.T) {
	resolver := NewHeightResolver()

	if resolver == nil {
		t.Fatal("expected non-nil HeightResolver")
	}

	expectedTimeout := 10 * time.Second
	if resolver.timeout != expectedTimeout {
		t.Errorf("expected timeout %v, got %v", expectedTimeout, resolver.timeout)
	}

	if resolver.httpClient == nil {
		t.Fatal("expected non-nil HTTP client")
	}
}

func TestHeightResolver_WithTimeout(t *testing.T) {
	resolver := NewHeightResolver()
	customTimeout := 30 * time.Second

	resolver = resolver.WithTimeout(customTimeout)

	if resolver.timeout != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, resolver.timeout)
	}

	if resolver.httpClient.Timeout != customTimeout {
		t.Errorf("expected HTTP client timeout %v, got %v", customTimeout, resolver.httpClient.Timeout)
	}
}

func TestHeightResolver_GetCurrentHeight_EmptyURL(t *testing.T) {
	ctx := context.Background()
	resolver := NewHeightResolver()

	_, err := resolver.GetCurrentHeight(ctx, "")

	if err == nil {
		t.Fatal("expected error for empty RPC URL")
	}
}

func TestHeightResolver_GetCurrentHeight_ValidResponse(t *testing.T) {
	// Create a test server that returns a valid RPC response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("expected path /status, got %s", r.URL.Path)
		}

		response := map[string]interface{}{
			"result": map[string]interface{}{
				"sync_info": map[string]interface{}{
					"latest_block_height": "1000000",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	resolver := NewHeightResolver()

	height, err := resolver.GetCurrentHeight(ctx, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedHeight := int64(1000000)
	if height != expectedHeight {
		t.Errorf("expected height %d, got %d", expectedHeight, height)
	}
}

func TestHeightResolver_GetCurrentHeight_InvalidStatusCode(t *testing.T) {
	// Create a test server that returns 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	ctx := context.Background()
	resolver := NewHeightResolver()

	_, err := resolver.GetCurrentHeight(ctx, server.URL)

	if err == nil {
		t.Fatal("expected error for non-200 status code")
	}
}

func TestHeightResolver_GetCurrentHeight_InvalidJSON(t *testing.T) {
	// Create a test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	ctx := context.Background()
	resolver := NewHeightResolver()

	_, err := resolver.GetCurrentHeight(ctx, server.URL)

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHeightResolver_GetCurrentHeight_InvalidHeightFormat(t *testing.T) {
	// Create a test server that returns non-numeric height
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"result": map[string]interface{}{
				"sync_info": map[string]interface{}{
					"latest_block_height": "not-a-number",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	resolver := NewHeightResolver()

	_, err := resolver.GetCurrentHeight(ctx, server.URL)

	if err == nil {
		t.Fatal("expected error for invalid height format")
	}
}

func TestHeightResolver_GetCurrentHeight_ZeroHeight(t *testing.T) {
	// Create a test server that returns height 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"result": map[string]interface{}{
				"sync_info": map[string]interface{}{
					"latest_block_height": "0",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	resolver := NewHeightResolver()

	_, err := resolver.GetCurrentHeight(ctx, server.URL)

	if err == nil {
		t.Fatal("expected error for height 0")
	}
}

func TestHeightResolver_GetCurrentHeight_ContextCancelled(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resolver := NewHeightResolver()

	_, err := resolver.GetCurrentHeight(ctx, server.URL)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestHeightResolver_WaitForHeight_AlreadyAtHeight(t *testing.T) {
	// Create a test server that returns the target height
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"result": map[string]interface{}{
				"sync_info": map[string]interface{}{
					"latest_block_height": "1000000",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	resolver := NewHeightResolver()

	err := resolver.WaitForHeight(ctx, server.URL, 1000000, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHeightResolver_WaitForHeight_ContextCancelled(t *testing.T) {
	// Create a test server that never reaches the target height
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"result": map[string]interface{}{
				"sync_info": map[string]interface{}{
					"latest_block_height": "500000",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	resolver := NewHeightResolver()

	err := resolver.WaitForHeight(ctx, server.URL, 1000000, 50*time.Millisecond)

	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestHeightResolver_WaitForHeight_ProgressesToTarget(t *testing.T) {
	// Create a test server that simulates increasing height
	currentHeight := int64(100000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Increment height on each request
		currentHeight += 10000

		response := map[string]interface{}{
			"result": map[string]interface{}{
				"sync_info": map[string]interface{}{
					"latest_block_height": fmt.Sprintf("%d", currentHeight),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resolver := NewHeightResolver()

	// Wait for height 150000 (should reach after a few polls)
	err := resolver.WaitForHeight(ctx, server.URL, 150000, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHeightResolver_WaitForHeight_ZeroPollInterval(t *testing.T) {
	// Create a test server that returns the target height
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"result": map[string]interface{}{
				"sync_info": map[string]interface{}{
					"latest_block_height": "1000000",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	ctx := context.Background()
	resolver := NewHeightResolver()

	// Pass 0 poll interval - should use default 2s
	err := resolver.WaitForHeight(ctx, server.URL, 1000000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
