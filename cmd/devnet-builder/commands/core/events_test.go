package core

import (
	"testing"
	"time"
)

func TestProcessDockerEvent(t *testing.T) {
	tests := []struct {
		name       string
		event      DockerEvent
		prefix     string
		wantNil    bool
		wantNode   string
		wantAction string
	}{
		{
			name: "start event matching prefix",
			event: DockerEvent{
				Action: "start",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name": "devnet-node0",
					},
				},
			},
			prefix:     "devnet-",
			wantNil:    false,
			wantNode:   "node0",
			wantAction: "start",
		},
		{
			name: "stop event matching prefix",
			event: DockerEvent{
				Action: "stop",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name": "devnet-node1",
					},
				},
			},
			prefix:     "devnet-",
			wantNil:    false,
			wantNode:   "node1",
			wantAction: "stop",
		},
		{
			name: "die event with exit code",
			event: DockerEvent{
				Action: "die",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name":     "devnet-node0",
						"exitCode": "137",
					},
				},
			},
			prefix:     "devnet-",
			wantNil:    false,
			wantNode:   "node0",
			wantAction: "die",
		},
		{
			name: "health_status event",
			event: DockerEvent{
				Action: "health_status",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name":          "devnet-node0",
						"health_status": "healthy",
					},
				},
			},
			prefix:     "devnet-",
			wantNil:    false,
			wantNode:   "node0",
			wantAction: "health_status",
		},
		{
			name: "event not matching prefix - filtered out",
			event: DockerEvent{
				Action: "start",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name": "other-container",
					},
				},
			},
			prefix:  "devnet-",
			wantNil: true,
		},
		{
			name: "event with empty container name - filtered out",
			event: DockerEvent{
				Action: "start",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{},
				},
			},
			prefix:  "devnet-",
			wantNil: true,
		},
		{
			name: "different prefix",
			event: DockerEvent{
				Action: "restart",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name": "testnet-node2",
					},
				},
			},
			prefix:     "testnet-",
			wantNil:    false,
			wantNode:   "node2",
			wantAction: "restart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processDockerEvent(tt.event, tt.prefix)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Node != tt.wantNode {
				t.Errorf("Node = %q, want %q", result.Node, tt.wantNode)
			}

			if result.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q", result.Action, tt.wantAction)
			}
		})
	}
}

func TestProcessDockerEvent_Details(t *testing.T) {
	tests := []struct {
		name        string
		event       DockerEvent
		prefix      string
		wantDetails string
	}{
		{
			name: "die event has exit code details",
			event: DockerEvent{
				Action: "die",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name":     "devnet-node0",
						"exitCode": "0",
					},
				},
			},
			prefix:      "devnet-",
			wantDetails: "exit code: 0",
		},
		{
			name: "health_status event has health details",
			event: DockerEvent{
				Action: "health_status",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name":          "devnet-node0",
						"health_status": "unhealthy",
					},
				},
			},
			prefix:      "devnet-",
			wantDetails: "health: unhealthy",
		},
		{
			name: "exec_create has exec details",
			event: DockerEvent{
				Action: "exec_create",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name": "devnet-node0",
					},
				},
			},
			prefix:      "devnet-",
			wantDetails: "exec command",
		},
		{
			name: "start event has no details",
			event: DockerEvent{
				Action: "start",
				Type:   "container",
				Time:   time.Now().Unix(),
				Actor: DockerEventActor{
					Attributes: map[string]string{
						"name": "devnet-node0",
					},
				},
			},
			prefix:      "devnet-",
			wantDetails: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processDockerEvent(tt.event, tt.prefix)
			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Details != tt.wantDetails {
				t.Errorf("Details = %q, want %q", result.Details, tt.wantDetails)
			}
		})
	}
}

func TestDockerEventTimestamp(t *testing.T) {
	// Test that timestamp is formatted correctly
	// Use local time to match how processDockerEvent formats timestamps
	testTime := time.Date(2024, 1, 15, 10, 30, 45, 0, time.Local)
	event := DockerEvent{
		Action: "start",
		Type:   "container",
		Time:   testTime.Unix(),
		Actor: DockerEventActor{
			Attributes: map[string]string{
				"name": "devnet-node0",
			},
		},
	}

	result := processDockerEvent(event, "devnet-")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// The timestamp should be formatted as "2006-01-02 15:04:05" in local time
	expectedFormat := testTime.Format("2006-01-02 15:04:05")
	if result.Timestamp != expectedFormat {
		t.Errorf("Timestamp = %q, want %q", result.Timestamp, expectedFormat)
	}
}
