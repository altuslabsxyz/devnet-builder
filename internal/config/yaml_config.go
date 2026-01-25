// internal/config/yaml_config.go
package config

// YAMLDevnet represents a Kubernetes-style devnet definition
type YAMLDevnet struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   YAMLMetadata   `yaml:"metadata"`
	Spec       YAMLDevnetSpec `yaml:"spec"`
}

// YAMLMetadata contains resource identification
type YAMLMetadata struct {
	Name        string            `yaml:"name"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// YAMLDevnetSpec defines the desired devnet state
type YAMLDevnetSpec struct {
	Network        string             `yaml:"network"`
	NetworkType    string             `yaml:"networkType,omitempty"`
	NetworkVersion string             `yaml:"networkVersion,omitempty"`
	Mode           string             `yaml:"mode,omitempty"`
	Validators     int                `yaml:"validators,omitempty"`
	FullNodes      int                `yaml:"fullNodes,omitempty"`
	Accounts       int                `yaml:"accounts,omitempty"`
	Resources      *YAMLResources     `yaml:"resources,omitempty"`
	Nodes          []YAMLNodeOverride `yaml:"nodes,omitempty"`
	Daemon         *YAMLDaemonConfig  `yaml:"daemon,omitempty"`
}

// YAMLResources defines resource limits
type YAMLResources struct {
	CPU     string `yaml:"cpu,omitempty"`
	Memory  string `yaml:"memory,omitempty"`
	Storage string `yaml:"storage,omitempty"`
}

// YAMLNodeOverride allows per-node configuration
type YAMLNodeOverride struct {
	Index     int            `yaml:"index"`
	Role      string         `yaml:"role,omitempty"`
	Resources *YAMLResources `yaml:"resources,omitempty"`
}

// YAMLDaemonConfig configures daemon behavior
type YAMLDaemonConfig struct {
	AutoStart   bool            `yaml:"autoStart,omitempty"`
	IdleTimeout string          `yaml:"idleTimeout,omitempty"`
	Logs        *YAMLLogsConfig `yaml:"logs,omitempty"`
}

// YAMLLogsConfig configures log storage
type YAMLLogsConfig struct {
	BufferSize string `yaml:"bufferSize,omitempty"`
	Retention  string `yaml:"retention,omitempty"`
}
