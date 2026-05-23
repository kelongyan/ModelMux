package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
)

const (
	// DefaultProviderID 是兼容旧版单 provider 配置时使用的隐式 provider ID。
	DefaultProviderID = "default"
	// DefaultCoolingSeconds 是 429 后默认冷却秒数。
	DefaultCoolingSeconds = 60
	// DefaultMaxRetries 是 401/429 后默认最大重试次数。
	DefaultMaxRetries = 3
	// DefaultRequestTimeoutSeconds 是默认上游请求超时秒数。
	DefaultRequestTimeoutSeconds = 120
	// DefaultMaxBodyBytes 是默认请求体上限，避免异常大请求占满内存。
	DefaultMaxBodyBytes int64 = 32 * 1024 * 1024
	// DefaultLogOutput 是默认日志输出目标，仅输出到控制台。
	DefaultLogOutput = "stdout"
	// DefaultLogMaxSizeMB 是单个日志文件默认最大体积。
	DefaultLogMaxSizeMB = 20
	// DefaultLogMaxBackups 是默认保留的旧日志文件数量。
	DefaultLogMaxBackups = 5
	// DefaultLogMaxAgeDays 是默认日志保留天数。
	DefaultLogMaxAgeDays = 30
	// DefaultStateFile 是默认 key 池状态持久化文件。
	DefaultStateFile = "state.json"
	// DefaultInvalidTTLHours 是 invalid 状态默认保留小时数。
	DefaultInvalidTTLHours = 24
)

type ProviderConfig struct {
	ID        string   `json:"id"`
	TargetURL string   `json:"target_url"`
	Keys      []string `json:"keys"`
}

type Config struct {
	Listen                string           `json:"listen"`
	AdminListen           string           `json:"admin_listen"`
	TargetURL             string           `json:"target_url"`
	Keys                  []string         `json:"keys"`
	ActiveProvider        string           `json:"active_provider"`
	Providers             []ProviderConfig `json:"providers"`
	CoolingSeconds        int              `json:"cooling_seconds"`
	MaxRetries            int              `json:"max_retries"`
	RequestTimeoutSeconds int              `json:"request_timeout_seconds"`
	MaxBodyBytes          int64            `json:"max_body_bytes"`
	LogLevel              string           `json:"log_level"`
	LogFormat             string           `json:"log_format"` // "text" (default) or "json"
	LogOutput             string           `json:"log_output"` // "stdout" (default), "file", or "both"
	LogFile               string           `json:"log_file"`
	LogMaxSizeMB          int              `json:"log_max_size_mb"`
	LogMaxBackups         int              `json:"log_max_backups"`
	LogMaxAgeDays         int              `json:"log_max_age_days"`
	LogCompress           bool             `json:"log_compress"`
	PersistState          *bool            `json:"persist_state"`
	StateFile             string           `json:"state_file"`
	InvalidTTLHours       int              `json:"invalid_ttl_hours"`
}

var (
	current *Config
	mu      sync.RWMutex
)

// Load 读取 JSON 配置、校验必填项并补齐默认值。
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
	if err := cfg.validateAfterDefaults(); err != nil {
		return nil, err
	}

	mu.Lock()
	current = cfg
	mu.Unlock()

	return cfg, nil
}

// Reload 重新读取配置文件，当前实现复用 Load 的校验和默认值逻辑。
func Reload(path string) (*Config, error) {
	return Load(path)
}

// Get 返回最近一次成功加载的配置快照。
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// validate 校验启动必须依赖的配置项。
func (c *Config) validate() error {
	providers := c.Providers
	activeProvider := c.ActiveProvider
	if len(providers) == 0 {
		if c.TargetURL == "" {
			return fmt.Errorf("target_url is required")
		}
		if err := validateTargetURL(c.TargetURL); err != nil {
			return fmt.Errorf("target_url: %w", err)
		}
		if len(c.Keys) == 0 {
			return fmt.Errorf("at least one key is required")
		}
		providers = []ProviderConfig{{
			ID:        DefaultProviderID,
			TargetURL: c.TargetURL,
			Keys:      c.Keys,
		}}
		if activeProvider == "" {
			activeProvider = DefaultProviderID
		}
	} else if activeProvider == "" {
		activeProvider = providers[0].ID
	}

	seen := make(map[string]struct{}, len(providers))
	activeFound := false
	for i, provider := range providers {
		if provider.ID == "" {
			return fmt.Errorf("providers[%d].id is required", i)
		}
		if _, ok := seen[provider.ID]; ok {
			return fmt.Errorf("duplicate provider id %q", provider.ID)
		}
		seen[provider.ID] = struct{}{}
		if provider.ID == activeProvider {
			activeFound = true
		}
		if err := validateTargetURL(provider.TargetURL); err != nil {
			return fmt.Errorf("providers[%d].target_url: %w", i, err)
		}
		if len(provider.Keys) == 0 {
			return fmt.Errorf("providers[%d].keys must contain at least one key", i)
		}
	}
	if activeProvider == "" {
		return fmt.Errorf("active_provider is required")
	}
	if !activeFound {
		return fmt.Errorf("active_provider %q does not match any provider id", activeProvider)
	}
	return nil
}

// validateTargetURL 校验 provider 上游地址必须是绝对 URL。
func validateTargetURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("is required")
	}
	target, err := url.Parse(rawURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("must be an absolute URL with scheme and host")
	}
	return nil
}

// validateAfterDefaults 校验依赖默认值补齐后的配置项。
func (c *Config) validateAfterDefaults() error {
	switch c.LogOutput {
	case "stdout", "file", "both":
	default:
		return fmt.Errorf("log_output must be one of: stdout, file, both")
	}
	if (c.LogOutput == "file" || c.LogOutput == "both") && c.LogFile == "" {
		return fmt.Errorf("log_file is required when log_output is file or both")
	}
	return nil
}

// TotalKeys 返回当前配置中的 key 数量。
func (c *Config) TotalKeys() int {
	provider, ok := c.ActiveProviderConfig()
	if !ok {
		return 0
	}
	return len(provider.Keys)
}

// TotalProviderKeys 返回所有 provider 的 key 总数。
func (c *Config) TotalProviderKeys() int {
	providers, _ := c.effectiveProviders()
	total := 0
	for _, provider := range providers {
		total += len(provider.Keys)
	}
	return total
}

// ActiveProviderConfig 返回当前选中的 provider 配置副本。
func (c *Config) ActiveProviderConfig() (ProviderConfig, bool) {
	providers, activeProvider := c.effectiveProviders()
	for _, provider := range providers {
		if provider.ID == activeProvider {
			return provider.copy(), true
		}
	}
	return ProviderConfig{}, false
}

// ProviderIDs 返回配置中的 provider ID 列表，方便日志和管理接口展示。
func (c *Config) ProviderIDs() []string {
	providers, _ := c.effectiveProviders()
	ids := make([]string, 0, len(providers))
	for _, provider := range providers {
		ids = append(ids, provider.ID)
	}
	return ids
}

// ProviderConfigs 返回当前生效的 provider 配置副本，供调用方构建运行时池使用。
func (c *Config) ProviderConfigs() []ProviderConfig {
	providers, _ := c.effectiveProviders()
	out := make([]ProviderConfig, len(providers))
	for i, provider := range providers {
		out[i] = provider.copy()
	}
	return out
}

// effectiveProviders 返回一份可用 provider 列表，并补齐默认 active provider。
func (c *Config) effectiveProviders() ([]ProviderConfig, string) {
	if len(c.Providers) > 0 {
		providers := make([]ProviderConfig, 0, len(c.Providers))
		for _, provider := range c.Providers {
			providers = append(providers, provider.copy())
		}
		active := c.ActiveProvider
		if active == "" && len(providers) > 0 {
			active = providers[0].ID
		}
		return providers, active
	}

	if c.TargetURL == "" && len(c.Keys) == 0 {
		return nil, c.ActiveProvider
	}
	providers := []ProviderConfig{{
		ID:        DefaultProviderID,
		TargetURL: c.TargetURL,
		Keys:      append([]string(nil), c.Keys...),
	}}
	active := c.ActiveProvider
	if active == "" {
		active = DefaultProviderID
	}
	return providers, active
}

// copy 返回 provider 配置副本，避免调用方误改共享 key 切片。
func (p ProviderConfig) copy() ProviderConfig {
	p.Keys = append([]string(nil), p.Keys...)
	return p
}

// applyDefaults 为个人使用场景填充安全、稳定的默认值。
func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.AdminListen == "" {
		c.AdminListen = "127.0.0.1:8081"
	}
	if c.CoolingSeconds <= 0 {
		c.CoolingSeconds = DefaultCoolingSeconds
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = DefaultMaxRetries
	}
	if c.RequestTimeoutSeconds <= 0 {
		c.RequestTimeoutSeconds = DefaultRequestTimeoutSeconds
	}
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFormat == "" {
		c.LogFormat = "text"
	}
	if c.LogOutput == "" {
		if c.LogFile != "" {
			c.LogOutput = "both"
		} else {
			c.LogOutput = DefaultLogOutput
		}
	}
	if c.LogMaxSizeMB <= 0 {
		c.LogMaxSizeMB = DefaultLogMaxSizeMB
	}
	if c.LogMaxBackups <= 0 {
		c.LogMaxBackups = DefaultLogMaxBackups
	}
	if c.LogMaxAgeDays <= 0 {
		c.LogMaxAgeDays = DefaultLogMaxAgeDays
	}
	if c.StateFile == "" {
		c.StateFile = DefaultStateFile
	}
	if c.InvalidTTLHours <= 0 {
		c.InvalidTTLHours = DefaultInvalidTTLHours
	}
	c.normalizeProviders()
}

// normalizeProviders 把旧版 target_url/keys 配置转换成隐式 provider，并同步 active provider 到旧字段。
func (c *Config) normalizeProviders() {
	providers, active := c.effectiveProviders()
	if len(providers) > 0 {
		c.Providers = providers
		c.ActiveProvider = active
	}
	if provider, ok := c.ActiveProviderConfig(); ok {
		c.TargetURL = provider.TargetURL
		c.Keys = append([]string(nil), provider.Keys...)
	}
}

// StatePersistenceEnabled 返回状态持久化是否启用；未配置时默认启用。
func (c *Config) StatePersistenceEnabled() bool {
	return c.PersistState == nil || *c.PersistState
}
