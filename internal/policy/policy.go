package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Policy represents a Kage security policy loaded from YAML.
type Policy struct {
	Version    int              `yaml:"version"`
	Network    *NetworkPolicy   `yaml:"network,omitempty"`
	Filesystem *FSPolicy        `yaml:"filesystem,omitempty"`
	Process    *ProcessPolicy   `yaml:"process,omitempty"`
	Resources  *ResourceLimits  `yaml:"resources,omitempty"`
}

// NetworkPolicy defines allowed, denied, and ask-prompted network destinations.
type NetworkPolicy struct {
	Allow []string   `yaml:"allow"`
	Deny  []string   `yaml:"deny"`
	Ask   *AskConfig `yaml:"ask,omitempty"`
}

// FSPolicy defines filesystem access rules.
type FSPolicy struct {
	AllowRead  []string `yaml:"allow_read"`
	AllowWrite []string `yaml:"allow_write"`
	Deny       []string `yaml:"deny"`
	Ask        []string `yaml:"ask"`
}

// ProcessPolicy defines which binaries can be executed.
type ProcessPolicy struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
	Ask   []string `yaml:"ask"`
}

// ResourceLimits defines resource constraints for the sandboxed process.
type ResourceLimits struct {
	MaxMemory     string `yaml:"max_memory"`
	MaxCPUPercent int    `yaml:"max_cpu_percent"`
	MaxDiskWrite  string `yaml:"max_disk_write"`
}

// AskConfig controls the default behavior for unmatched resources.
type AskConfig struct {
	Default string `yaml:"default"` // "ask" or "deny"
}

// LoadFromFile reads and parses a policy YAML file.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading policy file %s: %w", path, err)
	}
	return LoadFromBytes(data)
}

// LoadFromBytes parses a policy from raw YAML bytes.
func LoadFromBytes(data []byte) (*Policy, error) {
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing policy YAML: %w", err)
	}
	if p.Version == 0 {
		return nil, fmt.Errorf("policy must have a 'version' field")
	}
	if p.Version != 1 {
		return nil, fmt.Errorf("unsupported policy version: %d", p.Version)
	}
	return &p, nil
}

// DefaultPolicy returns a permissive policy used when no --policy flag is provided.
func DefaultPolicy() *Policy {
	return &Policy{
		Version: 1,
	}
}
