// internal/config/yaml_config.go
package config

import (
	"fmt"
	"strings"
)

const (
	// SupportedAPIVersionV1 is the legacy API version
	SupportedAPIVersionV1 = "devnet.lagos/v1"
	// SupportedAPIVersion is the current API version with namespace support
	SupportedAPIVersion = "devnet.lagos/v2"
	// SupportedKind is the resource kind
	SupportedKind = "Devnet"
)

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
	Namespace   string            `yaml:"namespace,omitempty"` // Defaults to "default" if not specified
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

// Validate checks if the YAMLDevnet is valid
func (d *YAMLDevnet) Validate() error {
	var errs []string

	// API version check - accept v1 or v2
	if d.APIVersion != SupportedAPIVersion && d.APIVersion != SupportedAPIVersionV1 {
		errs = append(errs, fmt.Sprintf("unsupported apiVersion %q, expected %q or %q",
			d.APIVersion, SupportedAPIVersion, SupportedAPIVersionV1))
	}

	// Kind check
	if d.Kind != SupportedKind {
		errs = append(errs, fmt.Sprintf("unsupported kind %q, expected %q", d.Kind, SupportedKind))
	}

	// Metadata validation
	if d.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	}

	// Spec validation
	if err := d.Spec.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Validate checks if the spec is valid
func (s *YAMLDevnetSpec) Validate() error {
	var errs []string

	if s.Network == "" {
		errs = append(errs, "spec.network is required")
	}

	if s.Validators < 1 {
		errs = append(errs, "spec.validators must be at least 1")
	}

	if s.Mode != "" && s.Mode != "docker" && s.Mode != "local" {
		errs = append(errs, fmt.Sprintf("spec.mode must be 'docker' or 'local', got %q", s.Mode))
	}

	if s.NetworkType != "" && s.NetworkType != "mainnet" && s.NetworkType != "testnet" {
		errs = append(errs, fmt.Sprintf("spec.networkType must be 'mainnet' or 'testnet', got %q", s.NetworkType))
	}

	if len(errs) > 0 {
		return fmt.Errorf("spec validation errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
