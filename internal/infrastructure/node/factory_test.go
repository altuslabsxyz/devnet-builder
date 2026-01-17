package node

import (
	"testing"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/types"
)

func TestFactoryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  FactoryConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid docker mode",
			config:  FactoryConfig{Mode: types.ExecutionModeDocker},
			wantErr: false,
		},
		{
			name:    "valid local mode",
			config:  FactoryConfig{Mode: types.ExecutionModeLocal},
			wantErr: false,
		},
		{
			name:    "empty mode",
			config:  FactoryConfig{Mode: ""},
			wantErr: true,
			errMsg:  "execution mode is required",
		},
		{
			name:    "unknown mode",
			config:  FactoryConfig{Mode: "kubernetes"},
			wantErr: true,
			errMsg:  "unknown execution mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr = true")
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestNodeManagerFactory_Create(t *testing.T) {
	logger := output.NewLogger()

	tests := []struct {
		name       string
		config     FactoryConfig
		wantErr    bool
		wantDocker bool
		wantLocal  bool
	}{
		{
			name: "docker mode - default image",
			config: FactoryConfig{
				Mode:   types.ExecutionModeDocker,
				Logger: logger,
			},
			wantDocker: true,
		},
		{
			name: "docker mode - custom image",
			config: FactoryConfig{
				Mode:        types.ExecutionModeDocker,
				DockerImage: "custom/image:v1.0",
				Logger:      logger,
			},
			wantDocker: true,
		},
		{
			name: "docker mode - with EVM chain ID",
			config: FactoryConfig{
				Mode:        types.ExecutionModeDocker,
				DockerImage: "stablelabs/stabled:latest",
				EVMChainID:  "988",
				Logger:      logger,
			},
			wantDocker: true,
		},
		{
			name: "local mode - default binary",
			config: FactoryConfig{
				Mode:   types.ExecutionModeLocal,
				Logger: logger,
			},
			wantLocal: true,
		},
		{
			name: "local mode - custom binary",
			config: FactoryConfig{
				Mode:       types.ExecutionModeLocal,
				BinaryPath: "/custom/path/stabled",
				Logger:     logger,
			},
			wantLocal: true,
		},
		{
			name: "local mode - with EVM chain ID",
			config: FactoryConfig{
				Mode:       types.ExecutionModeLocal,
				BinaryPath: "/usr/local/bin/stabled",
				EVMChainID: "988",
				Logger:     logger,
			},
			wantLocal: true,
		},
		{
			name: "nil logger uses default",
			config: FactoryConfig{
				Mode:   types.ExecutionModeDocker,
				Logger: nil,
			},
			wantDocker: true,
		},
		{
			name: "empty mode fails validation",
			config: FactoryConfig{
				Mode:   "",
				Logger: logger,
			},
			wantErr: true,
		},
		{
			name: "invalid mode fails validation",
			config: FactoryConfig{
				Mode:   "invalid",
				Logger: logger,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewNodeManagerFactory(tt.config)
			manager, err := factory.Create()

			if tt.wantErr {
				if err == nil {
					t.Error("Create() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("Create() unexpected error = %v", err)
				return
			}

			if manager == nil {
				t.Error("Create() returned nil manager")
				return
			}

			// Verify correct type was created
			_, isDocker := manager.(*DockerManager)
			_, isLocal := manager.(*LocalManager)

			if tt.wantDocker && !isDocker {
				t.Errorf("Create() returned %T, want *DockerManager", manager)
			}
			if tt.wantLocal && !isLocal {
				t.Errorf("Create() returned %T, want *LocalManager", manager)
			}
		})
	}
}

func TestNodeManagerFactory_Mode(t *testing.T) {
	tests := []struct {
		name string
		mode types.ExecutionMode
	}{
		{"docker mode", types.ExecutionModeDocker},
		{"local mode", types.ExecutionModeLocal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewNodeManagerFactory(FactoryConfig{Mode: tt.mode})
			if got := factory.Mode(); got != tt.mode {
				t.Errorf("ExecutionMode() = %q, want %q", got, tt.mode)
			}
		})
	}
}

func TestNodeManagerFactory_IsDocker(t *testing.T) {
	dockerFactory := NewNodeManagerFactory(FactoryConfig{Mode: types.ExecutionModeDocker})
	localFactory := NewNodeManagerFactory(FactoryConfig{Mode: types.ExecutionModeLocal})

	if !dockerFactory.IsDocker() {
		t.Error("IsDocker() = false for docker factory, want true")
	}
	if dockerFactory.IsLocal() {
		t.Error("IsLocal() = true for docker factory, want false")
	}

	if localFactory.IsDocker() {
		t.Error("IsDocker() = true for local factory, want false")
	}
	if !localFactory.IsLocal() {
		t.Error("IsLocal() = false for local factory, want true")
	}
}

func TestDockerManager_CreatedWithCorrectConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    FactoryConfig
		wantImage string
		wantEVMID string
	}{
		{
			name: "default image when empty",
			config: FactoryConfig{
				Mode:        types.ExecutionModeDocker,
				DockerImage: "",
			},
			wantImage: DefaultDockerImage,
			wantEVMID: "",
		},
		{
			name: "custom image",
			config: FactoryConfig{
				Mode:        types.ExecutionModeDocker,
				DockerImage: "my/image:v2",
			},
			wantImage: "my/image:v2",
			wantEVMID: "",
		},
		{
			name: "with EVM chain ID",
			config: FactoryConfig{
				Mode:        types.ExecutionModeDocker,
				DockerImage: "stablelabs/stabled:1.1.3",
				EVMChainID:  "988",
			},
			wantImage: "stablelabs/stabled:1.1.3",
			wantEVMID: "988",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewNodeManagerFactory(tt.config)
			manager, err := factory.Create()
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			dm, ok := manager.(*DockerManager)
			if !ok {
				t.Fatalf("Create() returned %T, want *DockerManager", manager)
			}

			if dm.Image != tt.wantImage {
				t.Errorf("DockerManager.Image = %q, want %q", dm.Image, tt.wantImage)
			}
			if dm.EVMChainID != tt.wantEVMID {
				t.Errorf("DockerManager.EVMChainID = %q, want %q", dm.EVMChainID, tt.wantEVMID)
			}
		})
	}
}

func TestLocalManager_CreatedWithCorrectConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     FactoryConfig
		wantBinary string
		wantEVMID  string
	}{
		{
			name: "default binary when empty",
			config: FactoryConfig{
				Mode:       types.ExecutionModeLocal,
				BinaryPath: "",
			},
			wantBinary: DefaultLocalBinary,
			wantEVMID:  "",
		},
		{
			name: "custom binary",
			config: FactoryConfig{
				Mode:       types.ExecutionModeLocal,
				BinaryPath: "/opt/stabled",
			},
			wantBinary: "/opt/stabled",
			wantEVMID:  "",
		},
		{
			name: "with EVM chain ID",
			config: FactoryConfig{
				Mode:       types.ExecutionModeLocal,
				BinaryPath: "/usr/local/bin/stabled",
				EVMChainID: "988",
			},
			wantBinary: "/usr/local/bin/stabled",
			wantEVMID:  "988",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewNodeManagerFactory(tt.config)
			manager, err := factory.Create()
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			lm, ok := manager.(*LocalManager)
			if !ok {
				t.Fatalf("Create() returned %T, want *LocalManager", manager)
			}

			if lm.Binary != tt.wantBinary {
				t.Errorf("LocalManager.Binary = %q, want %q", lm.Binary, tt.wantBinary)
			}
			if lm.EVMChainID != tt.wantEVMID {
				t.Errorf("LocalManager.EVMChainID = %q, want %q", lm.EVMChainID, tt.wantEVMID)
			}
		})
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
