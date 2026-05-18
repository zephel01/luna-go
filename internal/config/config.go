package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SandboxConfig controls optional Docker-based isolation for bash execution.
// When Enabled is true, bash commands run inside a Docker container instead of
// the host shell.  Docker must be installed and running on the host.
// If Docker is unavailable at runtime, luna falls back to unsandboxed execution
// with a warning rather than exiting.
type SandboxConfig struct {
	// Enabled switches on Docker sandboxing.  Can also be set via --sandbox flag.
	Enabled bool `yaml:"enabled"`
	// Image is the Docker image used for the container (e.g. "ubuntu:22.04").
	Image string `yaml:"image"`
	// Network is the Docker network mode ("none", "bridge", "host").
	// "none" (default) disables network access from within the sandbox.
	Network string `yaml:"network"`
	// Memory is the container memory limit accepted by Docker (e.g. "256m", "1g").
	Memory string `yaml:"memory"`
	// CPUs is the fractional CPU quota (e.g. "0.5" = half a core).
	CPUs string `yaml:"cpus"`
}

// Config holds all runtime configuration for luna.
type Config struct {
	Model      string `yaml:"model"`
	Provider   string `yaml:"provider"`
	BaseURL    string `yaml:"base_url"`
	APIKey     string `yaml:"api_key"`
	MaxIter    int    `yaml:"max_iter"`
	Unsafe     bool   `yaml:"unsafe"`
	Stream     bool   `yaml:"stream"`
	// Ollama-specific context window settings.
	// 0 means "use luna's auto-default" (32768 for num_ctx, 4096 for num_predict).
	// Set to -1 to disable auto-injection entirely.
	NumCtx     int    `yaml:"num_ctx"`
	NumPredict int    `yaml:"num_predict"`
	// LoopTimeoutMin is the wall-clock limit (minutes) for a single Run/loop call.
	// 0 means "use default (30 min)". Set to -1 to disable entirely.
	LoopTimeoutMin int `yaml:"loop_timeout_min"`
	// CompressModel is the Ollama model used for context compression summarisation.
	// Empty string means "use the same model as Model".
	// Compression only fires when CompressThreshold > 0.
	CompressModel string `yaml:"compress_model"`
	// CompressThreshold is the estimated token count at which context compression
	// triggers. 0 (default) = disabled. Set to e.g. 24000 to enable.
	CompressThreshold int `yaml:"compress_threshold"`
	// Sandbox controls Docker-based isolation for bash commands.
	// Disabled by default; opt in via config.yaml or --sandbox flag.
	Sandbox SandboxConfig `yaml:"sandbox"`
}

func defaults() Config {
	return Config{
		Model:    "qwen2.5-coder:7b",
		Provider: "ollama",
		BaseURL:  "http://localhost:11434",
		MaxIter:  20,
		Stream:   false, // non-streaming by default for reliable tool_calls parsing
		// NumCtx / NumPredict: 0 → auto (applied by OpenAIClient for Ollama endpoints)
		// LoopTimeoutMin: 0 → default (30 min)
		Sandbox: SandboxConfig{
			Image:   "ubuntu:22.04",
			Network: "none",
			Memory:  "256m",
			CPUs:    "0.5",
		},
	}
}

// GetBaseURL implements the cfgProvider interface used by memory subcommands.
func (c *Config) GetBaseURL() string { return c.BaseURL }

// GetModel implements the cfgProvider interface used by memory subcommands.
func (c *Config) GetModel() string { return c.Model }

// Path returns the canonical config file path (~/.luna-go/config.yaml).
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".luna-go", "config.yaml"), nil
}

// Exists reports whether the config file exists on disk.
func Exists() bool {
	p, err := Path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Save writes cfg to ~/.luna-go/config.yaml (creates directories as needed).
func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	header := "# luna-go configuration\n# https://github.com/zephel01/luna-go\n\n"
	return os.WriteFile(p, append([]byte(header), data...), 0o644)
}

// Show prints the current effective config with source annotations.
func Show(cfg *Config) {
	exists := Exists()
	p, _ := Path()
	if exists {
		fmt.Printf("# config file: %s\n\n", p)
	} else {
		fmt.Printf("# config file: %s (not found — using defaults)\n\n", p)
	}
	data, _ := yaml.Marshal(cfg)
	fmt.Print(string(data))
}

// Load reads ~/.luna-go/config.yaml (if it exists) and returns the merged Config.
// Missing file is not an error — defaults are returned instead.
func Load() (*Config, error) {
	cfg := defaults()

	home, err := os.UserHomeDir()
	if err != nil {
		return &cfg, nil
	}

	path := filepath.Join(home, ".luna-go", "config.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &cfg, nil
	}
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
