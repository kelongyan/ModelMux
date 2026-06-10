package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// persistedConfig 是写回磁盘时采用的首选 schema，避免再次写出 legacy 单 provider 字段。
type persistedConfig struct {
	Listen                          string           `json:"listen"`
	AdminListen                     string           `json:"admin_listen"`
	ActiveProvider                  string           `json:"active_provider"`
	Providers                       []ProviderConfig `json:"providers"`
	CoolingSeconds                  int              `json:"cooling_seconds"`
	MaxRetries                      int              `json:"max_retries"`
	MaxTransientRetries             int              `json:"max_transient_retries"`
	RequestTimeoutSeconds           int              `json:"request_timeout_seconds"`
	ConnectTimeoutSeconds           int              `json:"connect_timeout_seconds"`
	ResponseHeaderTimeoutSeconds    int              `json:"response_header_timeout_seconds"`
	TransientCoolingSeconds         int              `json:"transient_cooling_seconds"`
	WaitForKeyTimeoutMS             int              `json:"wait_for_key_timeout_ms"`
	StreamKeepAliveSeconds          int              `json:"stream_keepalive_seconds"`
	StreamIdleTimeoutSeconds        int              `json:"stream_idle_timeout_seconds"`
	StreamMaxDurationSeconds        int              `json:"stream_max_duration_seconds"`
	ProviderCircuitFailureThreshold int              `json:"provider_circuit_failure_threshold"`
	ProviderCircuitOpenSeconds      int              `json:"provider_circuit_open_seconds"`
	ProviderCircuitMaxOpenSeconds   int              `json:"provider_circuit_max_open_seconds"`
	ProviderCircuitHalfOpenMax      int              `json:"provider_circuit_half_open_max"`
	MaxBodyBytes                    int64            `json:"max_body_bytes"`
	LogLevel                        string           `json:"log_level"`
	LogFormat                       string           `json:"log_format"`
	LogOutput                       string           `json:"log_output"`
	LogFile                         string           `json:"log_file"`
	LogMaxSizeMB                    int              `json:"log_max_size_mb"`
	LogMaxBackups                   int              `json:"log_max_backups"`
	LogMaxAgeDays                   int              `json:"log_max_age_days"`
	LogCompress                     bool             `json:"log_compress"`
	PersistState                    bool             `json:"persist_state"`
	StateFile                       string           `json:"state_file"`
	InvalidTTLHours                 int              `json:"invalid_ttl_hours"`
	StatsEnabled                    bool             `json:"stats_enabled"`
	StatsDir                        string           `json:"stats_dir"`
	StatsRetentionDays              int              `json:"stats_retention_days"`
	StatsMaxRecentRecords           int              `json:"stats_max_recent_records"`
}

// writeFileAtomic 把配置以首选 schema 原子写回磁盘，避免保存半截 JSON。
func writeFileAtomic(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	payload := persistedConfig{
		Listen:                          cfg.Listen,
		AdminListen:                     cfg.AdminListen,
		ActiveProvider:                  cfg.ActiveProvider,
		Providers:                       cfg.ProviderConfigs(),
		CoolingSeconds:                  cfg.CoolingSeconds,
		MaxRetries:                      cfg.MaxRetries,
		MaxTransientRetries:             cfg.MaxTransientRetries,
		RequestTimeoutSeconds:           cfg.RequestTimeoutSeconds,
		ConnectTimeoutSeconds:           cfg.ConnectTimeoutSeconds,
		ResponseHeaderTimeoutSeconds:    cfg.ResponseHeaderTimeoutSeconds,
		TransientCoolingSeconds:         cfg.TransientCoolingSeconds,
		WaitForKeyTimeoutMS:             cfg.WaitForKeyTimeoutMS,
		StreamKeepAliveSeconds:          cfg.StreamKeepAliveSeconds,
		StreamIdleTimeoutSeconds:        cfg.StreamIdleTimeoutSeconds,
		StreamMaxDurationSeconds:        cfg.StreamMaxDurationSeconds,
		ProviderCircuitFailureThreshold: cfg.ProviderCircuitFailureThreshold,
		ProviderCircuitOpenSeconds:      cfg.ProviderCircuitOpenSeconds,
		ProviderCircuitMaxOpenSeconds:   cfg.ProviderCircuitMaxOpenSeconds,
		ProviderCircuitHalfOpenMax:      cfg.ProviderCircuitHalfOpenMax,
		MaxBodyBytes:                    cfg.MaxBodyBytes,
		LogLevel:                        cfg.LogLevel,
		LogFormat:                       cfg.LogFormat,
		LogOutput:                       cfg.LogOutput,
		LogFile:                         cfg.LogFile,
		LogMaxSizeMB:                    cfg.LogMaxSizeMB,
		LogMaxBackups:                   cfg.LogMaxBackups,
		LogMaxAgeDays:                   cfg.LogMaxAgeDays,
		LogCompress:                     cfg.LogCompress,
		PersistState:                    cfg.StatePersistenceEnabled(),
		StateFile:                       cfg.StateFile,
		InvalidTTLHours:                 cfg.InvalidTTLHours,
		StatsEnabled:                    cfg.StatsCollectionEnabled(),
		StatsDir:                        cfg.StatsDir,
		StatsRetentionDays:              cfg.StatsRetentionDays,
		StatsMaxRecentRecords:           cfg.StatsMaxRecentRecords,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
	}

	tmpPath := path + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("write config tmp: %w", err)
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write config tmp: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync config tmp: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close config tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("remove old config: %w", removeErr)
		}
		if renameErr := os.Rename(tmpPath, path); renameErr != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("replace config: %w", renameErr)
		}
	}
	return nil
}
