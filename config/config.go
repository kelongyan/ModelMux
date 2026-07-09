package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
)

const (
	// DefaultProviderID 是兼容旧版单 provider 配置时使用的隐式 provider ID。
	DefaultProviderID = "default"
	// DefaultListen 是本地代理默认监听地址。
	DefaultListen = "127.0.0.1:18080"
	// DefaultAdminListen 是本地管理服务默认监听地址。
	DefaultAdminListen = "127.0.0.1:18081"
	// DefaultCoolingSeconds 是 429 后默认冷却秒数。
	DefaultCoolingSeconds = 60
	// DefaultMaxRetries 是 401/429 后默认最大重试次数。
	DefaultMaxRetries = 3
	// DefaultMaxTransientRetries 是网络或 provider 临时失败后的默认最大重试次数。
	DefaultMaxTransientRetries = 1
	// DefaultRequestTimeoutSeconds 是默认上游请求超时秒数。
	DefaultRequestTimeoutSeconds = 120
	// DefaultConnectTimeoutSeconds 是默认单次上游连接超时秒数。
	DefaultConnectTimeoutSeconds = 5
	// DefaultResponseHeaderTimeoutSeconds 是默认等待上游响应头的超时秒数。
	DefaultResponseHeaderTimeoutSeconds = 30
	// DefaultTransientCoolingSeconds 是网络或网关类临时失败后的默认短冷却秒数。
	DefaultTransientCoolingSeconds = 15
	// DefaultWaitForKeyTimeoutMS 是所有 key 暂时 cooling 时允许短等的默认毫秒数。
	DefaultWaitForKeyTimeoutMS = 1000
	// DefaultStreamKeepAliveSeconds 是 SSE 流空闲时发送注释保活的默认秒数。
	DefaultStreamKeepAliveSeconds = 15
	// DefaultStreamIdleTimeoutSeconds 是流式响应长时间无上游数据后的默认保护秒数。
	DefaultStreamIdleTimeoutSeconds = 300
	// DefaultStreamMaxDurationSeconds 是单个流式响应允许持续的默认最大秒数。
	DefaultStreamMaxDurationSeconds = 3600
	// DefaultProviderCircuitFailureThreshold 是 provider 级临时失败连续达到多少次后打开熔断。
	DefaultProviderCircuitFailureThreshold = 3
	// DefaultProviderCircuitOpenSeconds 是 provider 熔断首次打开的默认秒数。
	DefaultProviderCircuitOpenSeconds = 5
	// DefaultProviderCircuitMaxOpenSeconds 是 provider 熔断退避打开的最大秒数。
	DefaultProviderCircuitMaxOpenSeconds = 60
	// DefaultProviderCircuitHalfOpenMax 是 provider 熔断半开时允许的探针并发数。
	DefaultProviderCircuitHalfOpenMax = 1
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
	// DefaultStatsDir 是默认调用统计明细目录，避免与 Go 源码包 stats/ 冲突。
	DefaultStatsDir = "stats_data"
	// DefaultStatsRetentionDays 是默认保留调用统计明细的天数。
	DefaultStatsRetentionDays = 30
	// DefaultStatsMaxRecentRecords 是默认加载到内存的最近调用记录数量。
	DefaultStatsMaxRecentRecords = 10000

	// ProtocolOpenAI 是默认上游协议，注入 OpenAI 专有的 stream_options.include_usage。
	ProtocolOpenAI = "openai"
	// ProtocolAnthropic 表示上游为 Anthropic 原生 /v1/messages 协议，不注入 OpenAI 专有字段。
	ProtocolAnthropic = "anthropic"
	// ProtocolGemini 表示上游为 Gemini 协议，不注入 OpenAI 专有字段。
	ProtocolGemini = "gemini"
	// DefaultProtocol 是 provider 未显式声明协议时的默认值。
	DefaultProtocol = ProtocolOpenAI
)

type ProviderConfig struct {
	ID          string                 `json:"id"`
	TargetURL   string                 `json:"target_url"`
	Keys        []string               `json:"keys"`
	KeyMetadata map[string]KeyMetadata `json:"key_metadata,omitempty"`
	Models      []string               `json:"models,omitempty"`
	StripTools  bool                   `json:"strip_tools,omitempty"`
	Protocol    string                 `json:"protocol,omitempty"`
}

type KeyMetadata struct {
	Label    string `json:"label,omitempty"`
	Note     string `json:"note,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

type Config struct {
	Listen                          string           `json:"listen"`
	AdminListen                     string           `json:"admin_listen"`
	TargetURL                       string           `json:"target_url"`
	Keys                            []string         `json:"keys"`
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
	LogFormat                       string           `json:"log_format"` // "text" (default) or "json"
	LogOutput                       string           `json:"log_output"` // "stdout" (default), "file", or "both"
	LogFile                         string           `json:"log_file"`
	LogMaxSizeMB                    int              `json:"log_max_size_mb"`
	LogMaxBackups                   int              `json:"log_max_backups"`
	LogMaxAgeDays                   int              `json:"log_max_age_days"`
	LogCompress                     bool             `json:"log_compress"`
	PersistState                    *bool            `json:"persist_state"`
	StateFile                       string           `json:"state_file"`
	InvalidTTLHours                 int              `json:"invalid_ttl_hours"`
	StatsEnabled                    *bool            `json:"stats_enabled"`
	StatsDir                        string           `json:"stats_dir"`
	StatsRetentionDays              int              `json:"stats_retention_days"`
	StatsMaxRecentRecords           int              `json:"stats_max_recent_records"`
	AdminAPIKey                     string           `json:"admin_api_key,omitempty"`
}

var (
	current *Config
	mu      sync.RWMutex
)

// Load 读取 JSON 配置、校验必填项并补齐默认值。
func Load(path string) (*Config, error) {
	return load(path, true)
}

// Read 读取并规范化配置，但不替换当前运行时快照，适合在提交热重载前做预检查。
func Read(path string) (*Config, error) {
	return load(path, false)
}

// Get 返回最近一次成功加载的配置快照。
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return nil
	}
	return current.Clone()
}

// SetCurrent 用一份已校验的配置替换当前快照，供热重载成功后原子提交。
func SetCurrent(cfg *Config) {
	mu.Lock()
	current = cfg.Clone()
	mu.Unlock()
}

// Validate 校验启动必须依赖的配置项。
func (c *Config) Validate() error {
	return c.validate()
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
		if strings.ContainsAny(provider.ID, "/?#") {
			return fmt.Errorf("providers[%d].id contains unsupported path characters", i)
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
		for j, key := range provider.Keys {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("providers[%d].keys[%d] is empty", i, j)
			}
		}
		if err := validateProtocol(provider.Protocol); err != nil {
			return fmt.Errorf("providers[%d].protocol: %w", i, err)
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

// validateTargetURL 校验 provider 上游地址必须是 http/https 绝对 URL。
func validateTargetURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("is required")
	}
	target, err := url.Parse(rawURL)
	if err != nil || target.Host == "" {
		return fmt.Errorf("must be an absolute URL with scheme and host")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", target.Scheme)
	}
	return nil
}

// validateProtocol 校验 provider 协议取值合法；空值等价于默认 openai，在补齐默认值前放行。
func validateProtocol(protocol string) error {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", ProtocolOpenAI, ProtocolAnthropic, ProtocolGemini:
		return nil
	default:
		return fmt.Errorf("must be one of: %s, %s, %s", ProtocolOpenAI, ProtocolAnthropic, ProtocolGemini)
	}
}

// ValidateAfterDefaults 校验依赖默认值补齐后的配置项。
func (c *Config) ValidateAfterDefaults() error {
	return c.validateAfterDefaults()
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
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log_level must be one of: debug, info, warn, error")
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("log_format must be one of: text, json")
	}
	if c.MaxRetries < 1 || c.MaxRetries > 100 {
		return fmt.Errorf("max_retries must be between 1 and 100")
	}
	if c.MaxTransientRetries < 0 || c.MaxTransientRetries > 100 {
		return fmt.Errorf("max_transient_retries must be between 0 and 100")
	}
	if c.CoolingSeconds < 1 || c.CoolingSeconds > 86400 {
		return fmt.Errorf("cooling_seconds must be between 1 and 86400")
	}
	if c.RequestTimeoutSeconds < 1 || c.RequestTimeoutSeconds > 86400 {
		return fmt.Errorf("request_timeout_seconds must be between 1 and 86400")
	}
	if c.ConnectTimeoutSeconds < 1 || c.ConnectTimeoutSeconds > 3600 {
		return fmt.Errorf("connect_timeout_seconds must be between 1 and 3600")
	}
	if c.ResponseHeaderTimeoutSeconds < 1 || c.ResponseHeaderTimeoutSeconds > 3600 {
		return fmt.Errorf("response_header_timeout_seconds must be between 1 and 3600")
	}
	if c.MaxBodyBytes < 1 || c.MaxBodyBytes > 1024*1024*1024 {
		return fmt.Errorf("max_body_bytes must be between 1 and 1073741824 (1GB)")
	}
	if c.InvalidTTLHours < 1 || c.InvalidTTLHours > 8760 {
		return fmt.Errorf("invalid_ttl_hours must be between 1 and 8760")
	}
	if c.StatsRetentionDays < 1 || c.StatsRetentionDays > 365 {
		return fmt.Errorf("stats_retention_days must be between 1 and 365")
	}
	if c.AdminAPIKey != "" && len(c.AdminAPIKey) < 8 {
		return fmt.Errorf("admin_api_key must be at least 8 characters when set")
	}
	if c.ConnectTimeoutSeconds > c.RequestTimeoutSeconds {
		return fmt.Errorf("connect_timeout_seconds (%d) must not exceed request_timeout_seconds (%d)", c.ConnectTimeoutSeconds, c.RequestTimeoutSeconds)
	}
	if c.ResponseHeaderTimeoutSeconds > c.RequestTimeoutSeconds {
		return fmt.Errorf("response_header_timeout_seconds (%d) must not exceed request_timeout_seconds (%d)", c.ResponseHeaderTimeoutSeconds, c.RequestTimeoutSeconds)
	}
	if c.StreamIdleTimeoutSeconds > c.StreamMaxDurationSeconds {
		return fmt.Errorf("stream_idle_timeout_seconds (%d) must not exceed stream_max_duration_seconds (%d)", c.StreamIdleTimeoutSeconds, c.StreamMaxDurationSeconds)
	}
	if c.StreamKeepAliveSeconds > c.StreamIdleTimeoutSeconds {
		return fmt.Errorf("stream_keepalive_seconds (%d) must not exceed stream_idle_timeout_seconds (%d)", c.StreamKeepAliveSeconds, c.StreamIdleTimeoutSeconds)
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
	p.KeyMetadata = copyKeyMetadata(p.KeyMetadata)
	p.Models = append([]string(nil), p.Models...)
	return p
}

// ApplyDefaults 为个人使用场景填充安全、稳定的默认值。
func (c *Config) ApplyDefaults() {
	c.applyDefaults()
}

// applyDefaults 为个人使用场景填充安全、稳定的默认值。
func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = DefaultListen
	}
	if c.AdminListen == "" {
		c.AdminListen = DefaultAdminListen
	}
	if c.CoolingSeconds <= 0 {
		c.CoolingSeconds = DefaultCoolingSeconds
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = DefaultMaxRetries
	}
	if c.MaxTransientRetries <= 0 {
		c.MaxTransientRetries = DefaultMaxTransientRetries
	}
	if c.RequestTimeoutSeconds <= 0 {
		c.RequestTimeoutSeconds = DefaultRequestTimeoutSeconds
	}
	if c.ConnectTimeoutSeconds <= 0 {
		c.ConnectTimeoutSeconds = DefaultConnectTimeoutSeconds
	}
	if c.ResponseHeaderTimeoutSeconds <= 0 {
		c.ResponseHeaderTimeoutSeconds = DefaultResponseHeaderTimeoutSeconds
	}
	if c.TransientCoolingSeconds <= 0 {
		c.TransientCoolingSeconds = DefaultTransientCoolingSeconds
	}
	if c.WaitForKeyTimeoutMS <= 0 {
		c.WaitForKeyTimeoutMS = DefaultWaitForKeyTimeoutMS
	}
	if c.StreamKeepAliveSeconds <= 0 {
		c.StreamKeepAliveSeconds = DefaultStreamKeepAliveSeconds
	}
	if c.StreamIdleTimeoutSeconds <= 0 {
		c.StreamIdleTimeoutSeconds = DefaultStreamIdleTimeoutSeconds
	}
	if c.StreamMaxDurationSeconds <= 0 {
		c.StreamMaxDurationSeconds = DefaultStreamMaxDurationSeconds
	}
	if c.ProviderCircuitFailureThreshold <= 0 {
		c.ProviderCircuitFailureThreshold = DefaultProviderCircuitFailureThreshold
	}
	if c.ProviderCircuitOpenSeconds <= 0 {
		c.ProviderCircuitOpenSeconds = DefaultProviderCircuitOpenSeconds
	}
	if c.ProviderCircuitMaxOpenSeconds <= 0 {
		c.ProviderCircuitMaxOpenSeconds = DefaultProviderCircuitMaxOpenSeconds
	}
	if c.ProviderCircuitHalfOpenMax <= 0 {
		c.ProviderCircuitHalfOpenMax = DefaultProviderCircuitHalfOpenMax
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
	if c.StatsDir == "" {
		c.StatsDir = DefaultStatsDir
	}
	if c.StatsRetentionDays <= 0 {
		c.StatsRetentionDays = DefaultStatsRetentionDays
	}
	if c.StatsMaxRecentRecords <= 0 {
		c.StatsMaxRecentRecords = DefaultStatsMaxRecentRecords
	}
	c.normalizeProviders()
	c.normalizeKeyMetadata()
}

// normalizeProviders 把旧版 target_url/keys 配置转换成隐式 provider，并同步 active provider 到旧字段。
func (c *Config) normalizeProviders() {
	providers, active := c.effectiveProviders()
	if len(providers) > 0 {
		c.Providers = providers
		c.ActiveProvider = active
	}
	for i := range c.Providers {
		protocol := strings.ToLower(strings.TrimSpace(c.Providers[i].Protocol))
		if protocol == "" {
			protocol = DefaultProtocol
		}
		c.Providers[i].Protocol = protocol
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

// StatsCollectionEnabled 返回调用统计是否启用；未配置时默认启用。
func (c *Config) StatsCollectionEnabled() bool {
	return c.StatsEnabled == nil || *c.StatsEnabled
}

// Clone 返回完整配置副本，避免调用方误改共享切片或指针字段。
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	out := *c
	out.Keys = append([]string(nil), c.Keys...)
	if c.Providers != nil {
		out.Providers = make([]ProviderConfig, len(c.Providers))
		for i, provider := range c.Providers {
			out.Providers[i] = provider.copy()
		}
	}
	if c.PersistState != nil {
		enabled := *c.PersistState
		out.PersistState = &enabled
	}
	if c.StatsEnabled != nil {
		enabled := *c.StatsEnabled
		out.StatsEnabled = &enabled
	}
	return &out
}

// load 统一封装配置读取与规范化流程，并按需决定是否提交到当前快照。
func load(path string, setCurrent bool) (*Config, error) {
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

	if setCurrent {
		SetCurrent(cfg)
	}
	return cfg, nil
}
