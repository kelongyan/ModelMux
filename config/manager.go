package config

import (
	"fmt"
	"sync"
)

var (
	// HotReloadFields 列出当前架构下保存后可直接热生效的配置字段。
	HotReloadFields = []string{
		"active_provider",
		"providers",
		"cooling_seconds",
		"max_retries",
		"max_transient_retries",
		"request_timeout_seconds",
		"connect_timeout_seconds",
		"response_header_timeout_seconds",
		"transient_cooling_seconds",
		"wait_for_key_timeout_ms",
		"max_body_bytes",
	}
	// RestartRequiredFields 列出保存后需要重启进程才能完全生效的字段。
	RestartRequiredFields = []string{
		"listen",
		"admin_listen",
		"log_level",
		"log_format",
		"log_output",
		"log_file",
		"log_max_size_mb",
		"log_max_backups",
		"log_max_age_days",
		"log_compress",
		"persist_state",
		"state_file",
		"invalid_ttl_hours",
		"stats_enabled",
		"stats_dir",
		"stats_retention_days",
		"stats_max_recent_records",
	}
)

// Manager 负责配置文件读取、原子写入、回滚与热重载提交。
type Manager struct {
	path     string
	reloadFn func(string) error
	mu       sync.Mutex
}

// UpdateResult 描述一次配置变更的结果，便于前端提示热生效与重启影响。
type UpdateResult struct {
	Config                 *Config  `json:"config"`
	ChangedFields          []string `json:"changed_fields"`
	HotReloadedFields      []string `json:"hot_reloaded_fields"`
	RestartRequiredFields  []string `json:"restart_required_fields"`
	ReloadTriggered        bool     `json:"reload_triggered"`
	RollbackAppliedOnError bool     `json:"rollback_applied_on_error,omitempty"`
}

// NewManager 创建绑定到指定配置文件路径的配置管理器。
func NewManager(path string, reloadFn func(string) error) *Manager {
	return &Manager{
		path:     path,
		reloadFn: reloadFn,
	}
}

// Path 返回当前配置文件路径，方便管理接口展示和导出。
func (m *Manager) Path() string {
	return m.path
}

// Snapshot 读取当前磁盘配置快照，返回一份可安全修改的副本。
func (m *Manager) Snapshot() (*Config, error) {
	cfg, err := Read(m.path)
	if err != nil {
		return nil, err
	}
	return cfg.Clone(), nil
}

// Update 在互斥保护下执行配置变更、原子写盘和热重载提交。
func (m *Manager) Update(mutator func(*Config) error) (*UpdateResult, error) {
	if mutator == nil {
		return nil, fmt.Errorf("mutator is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	before, err := Read(m.path)
	if err != nil {
		return nil, err
	}
	next := before.Clone()
	if err := mutator(next); err != nil {
		return nil, err
	}
	if err := next.validate(); err != nil {
		return nil, err
	}
	next.applyDefaults()
	if err := next.validateAfterDefaults(); err != nil {
		return nil, err
	}

	changedFields := diffFields(before, next)
	result := &UpdateResult{
		Config:                next.Clone(),
		ChangedFields:         changedFields,
		HotReloadedFields:     categorizeFields(changedFields, HotReloadFields),
		RestartRequiredFields: categorizeFields(changedFields, RestartRequiredFields),
	}
	if len(changedFields) == 0 {
		return result, nil
	}

	if err := writeFileAtomic(m.path, next); err != nil {
		return nil, err
	}
	if m.reloadFn == nil {
		SetCurrent(next)
		return result, nil
	}
	if err := m.reloadFn(m.path); err != nil {
		result.RollbackAppliedOnError = true
		if rollbackErr := writeFileAtomic(m.path, before); rollbackErr != nil {
			return nil, fmt.Errorf("reload config: %w (rollback config: %v)", err, rollbackErr)
		}
		if restoreErr := m.reloadFn(m.path); restoreErr != nil {
			return nil, fmt.Errorf("reload config: %w (restore runtime: %v)", err, restoreErr)
		}
		return nil, err
	}
	result.ReloadTriggered = true
	return result, nil
}

// diffFields 比较两份归一化配置，返回发生变化的顶层字段名。
func diffFields(before, after *Config) []string {
	if before == nil || after == nil {
		return nil
	}

	changed := make([]string, 0, 8)
	appendIfChanged := func(name string, equal bool) {
		if !equal {
			changed = append(changed, name)
		}
	}

	appendIfChanged("listen", before.Listen == after.Listen)
	appendIfChanged("admin_listen", before.AdminListen == after.AdminListen)
	appendIfChanged("active_provider", before.ActiveProvider == after.ActiveProvider)
	appendIfChanged("providers", equalProviders(before.ProviderConfigs(), after.ProviderConfigs()))
	appendIfChanged("cooling_seconds", before.CoolingSeconds == after.CoolingSeconds)
	appendIfChanged("max_retries", before.MaxRetries == after.MaxRetries)
	appendIfChanged("max_transient_retries", before.MaxTransientRetries == after.MaxTransientRetries)
	appendIfChanged("request_timeout_seconds", before.RequestTimeoutSeconds == after.RequestTimeoutSeconds)
	appendIfChanged("connect_timeout_seconds", before.ConnectTimeoutSeconds == after.ConnectTimeoutSeconds)
	appendIfChanged("response_header_timeout_seconds", before.ResponseHeaderTimeoutSeconds == after.ResponseHeaderTimeoutSeconds)
	appendIfChanged("transient_cooling_seconds", before.TransientCoolingSeconds == after.TransientCoolingSeconds)
	appendIfChanged("wait_for_key_timeout_ms", before.WaitForKeyTimeoutMS == after.WaitForKeyTimeoutMS)
	appendIfChanged("max_body_bytes", before.MaxBodyBytes == after.MaxBodyBytes)
	appendIfChanged("log_level", before.LogLevel == after.LogLevel)
	appendIfChanged("log_format", before.LogFormat == after.LogFormat)
	appendIfChanged("log_output", before.LogOutput == after.LogOutput)
	appendIfChanged("log_file", before.LogFile == after.LogFile)
	appendIfChanged("log_max_size_mb", before.LogMaxSizeMB == after.LogMaxSizeMB)
	appendIfChanged("log_max_backups", before.LogMaxBackups == after.LogMaxBackups)
	appendIfChanged("log_max_age_days", before.LogMaxAgeDays == after.LogMaxAgeDays)
	appendIfChanged("log_compress", before.LogCompress == after.LogCompress)
	appendIfChanged("persist_state", before.StatePersistenceEnabled() == after.StatePersistenceEnabled())
	appendIfChanged("state_file", before.StateFile == after.StateFile)
	appendIfChanged("invalid_ttl_hours", before.InvalidTTLHours == after.InvalidTTLHours)
	appendIfChanged("stats_enabled", before.StatsCollectionEnabled() == after.StatsCollectionEnabled())
	appendIfChanged("stats_dir", before.StatsDir == after.StatsDir)
	appendIfChanged("stats_retention_days", before.StatsRetentionDays == after.StatsRetentionDays)
	appendIfChanged("stats_max_recent_records", before.StatsMaxRecentRecords == after.StatsMaxRecentRecords)
	return changed
}

// equalProviders 比较 provider 列表是否一致，避免为切片地址差异误报变更。
func equalProviders(a, b []ProviderConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].TargetURL != b[i].TargetURL || a[i].StripTools != b[i].StripTools {
			return false
		}
		if len(a[i].Keys) != len(b[i].Keys) {
			return false
		}
		for j := range a[i].Keys {
			if a[i].Keys[j] != b[i].Keys[j] {
				return false
			}
		}
		if len(a[i].Models) != len(b[i].Models) {
			return false
		}
		for j := range a[i].Models {
			if a[i].Models[j] != b[i].Models[j] {
				return false
			}
		}
	}
	return true
}

// categorizeFields 根据候选名单筛出需要归类展示的字段集合。
func categorizeFields(changedFields []string, bucket []string) []string {
	allow := make(map[string]struct{}, len(bucket))
	for _, field := range bucket {
		allow[field] = struct{}{}
	}

	out := make([]string, 0, len(changedFields))
	for _, field := range changedFields {
		if _, ok := allow[field]; ok {
			out = append(out, field)
		}
	}
	return out
}
