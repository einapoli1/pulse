package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type HostConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Port     int    `yaml:"port"`
	KeyFile  string `yaml:"key_file"`
	Password string `yaml:"password"`
	Label    string `yaml:"label"`
}

type NotifyConfig struct {
	Webhook string `yaml:"webhook"` // POST URL for state changes
	Command string `yaml:"command"` // shell command, {host} {label} {state} replaced
}

type Config struct {
	Interval int          `yaml:"interval"` // seconds
	Hosts    []HostConfig `yaml:"hosts"`
	Notify   NotifyConfig `yaml:"notify"`
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pulse", "hosts.yaml")
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Interval <= 0 {
		cfg.Interval = 30
	}

	for i := range cfg.Hosts {
		if cfg.Hosts[i].Port == 0 {
			cfg.Hosts[i].Port = 22
		}
		if cfg.Hosts[i].Label == "" {
			cfg.Hosts[i].Label = cfg.Hosts[i].Name
		}
	}

	return &cfg, nil
}

func writeDefaultConfig(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	sample := `# Pulse - Host Monitor Configuration
interval: 30  # seconds between checks

hosts:
  - name: example
    host: 192.168.1.100
    user: admin
    port: 22
    label: "Example Server"
    # key_file: ~/.ssh/id_ed25519
    # password: use key_file instead
`
	return os.WriteFile(path, []byte(sample), 0644)
}
