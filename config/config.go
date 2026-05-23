package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Config struct {
	Listen                string   `json:"listen"`
	AdminListen           string   `json:"admin_listen"`
	TargetURL             string   `json:"target_url"`
	Keys                  []string `json:"keys"`
	CoolingSeconds        int      `json:"cooling_seconds"`
	MaxRetries            int      `json:"max_retries"`
	RequestTimeoutSeconds int      `json:"request_timeout_seconds"`
	LogLevel              string   `json:"log_level"`
	LogFormat             string   `json:"log_format"` // "text" (default) or "json"
}

var (
	current *Config
	mu      sync.RWMutex
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	mu.Lock()
	current = cfg
	mu.Unlock()

	return cfg, nil
}

func Reload(path string) (*Config, error) {
	return Load(path)
}

func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

func (c *Config) validate() error {
	if c.TargetURL == "" {
		return fmt.Errorf("target_url is required")
	}
	if len(c.Keys) == 0 {
		return fmt.Errorf("at least one key is required")
	}
	return nil
}

func (c *Config) TotalKeys() int {
	return len(c.Keys)
}

func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.AdminListen == "" {
		c.AdminListen = ":8081"
	}
	if c.CoolingSeconds <= 0 {
		c.CoolingSeconds = 60
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.RequestTimeoutSeconds <= 0 {
		c.RequestTimeoutSeconds = 120
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFormat == "" {
		c.LogFormat = "text"
	}
}
