package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/proxy"
	"github.com/kelongyan/ModelMux/state"
)

type apiSettingsPayload struct {
	Listen                          string `json:"listen"`
	AdminListen                     string `json:"admin_listen"`
	ActiveProvider                  string `json:"active_provider"`
	CoolingSeconds                  int    `json:"cooling_seconds"`
	MaxRetries                      int    `json:"max_retries"`
	MaxTransientRetries             *int   `json:"max_transient_retries,omitempty"`
	RequestTimeoutSeconds           int    `json:"request_timeout_seconds"`
	ConnectTimeoutSeconds           *int   `json:"connect_timeout_seconds,omitempty"`
	ResponseHeaderTimeoutSeconds    *int   `json:"response_header_timeout_seconds,omitempty"`
	TransientCoolingSeconds         *int   `json:"transient_cooling_seconds,omitempty"`
	WaitForKeyTimeoutMS             *int   `json:"wait_for_key_timeout_ms,omitempty"`
	StreamKeepAliveSeconds          *int   `json:"stream_keepalive_seconds,omitempty"`
	StreamIdleTimeoutSeconds        *int   `json:"stream_idle_timeout_seconds,omitempty"`
	StreamMaxDurationSeconds        *int   `json:"stream_max_duration_seconds,omitempty"`
	ProviderCircuitFailureThreshold *int   `json:"provider_circuit_failure_threshold,omitempty"`
	ProviderCircuitOpenSeconds      *int   `json:"provider_circuit_open_seconds,omitempty"`
	ProviderCircuitMaxOpenSeconds   *int   `json:"provider_circuit_max_open_seconds,omitempty"`
	ProviderCircuitHalfOpenMax      *int   `json:"provider_circuit_half_open_max,omitempty"`
	MaxBodyBytes                    int64  `json:"max_body_bytes"`
	LogLevel                        string `json:"log_level"`
	LogFormat                       string `json:"log_format"`
	LogOutput                       string `json:"log_output"`
	LogFile                         string `json:"log_file"`
	LogMaxSizeMB                    int    `json:"log_max_size_mb"`
	LogMaxBackups                   int    `json:"log_max_backups"`
	LogMaxAgeDays                   int    `json:"log_max_age_days"`
	LogCompress                     bool   `json:"log_compress"`
	PersistState                    bool   `json:"persist_state"`
	StateFile                       string `json:"state_file"`
	InvalidTTLHours                 int    `json:"invalid_ttl_hours"`
	StatsEnabled                    bool   `json:"stats_enabled"`
	StatsDir                        string `json:"stats_dir"`
	StatsRetentionDays              int    `json:"stats_retention_days"`
	StatsMaxRecentRecords           int    `json:"stats_max_recent_records"`
	HasAdminAPIKey                  bool   `json:"has_admin_api_key"`
	AdminAPIKey                     string `json:"admin_api_key,omitempty"`
}

type apiProviderSummary struct {
	ID                 string   `json:"id"`
	Active             bool     `json:"active"`
	TargetURL          string   `json:"target_url"`
	TotalKeys          int      `json:"total_keys"`
	DisabledKeys       int      `json:"disabled_keys"`
	QuotaExhaustedKeys int      `json:"quota_exhausted_keys"`
	ActiveKeys         int      `json:"active_keys"`
	CoolingKeys        int      `json:"cooling_keys"`
	InvalidKeys        int      `json:"invalid_keys"`
	Models             []string `json:"models"`
}

type apiProviderDetail struct {
	ID                 string                 `json:"id"`
	Active             bool                   `json:"active"`
	TargetURL          string                 `json:"target_url"`
	TotalKeys          int                    `json:"total_keys"`
	DisabledKeys       int                    `json:"disabled_keys"`
	QuotaExhaustedKeys int                    `json:"quota_exhausted_keys"`
	ActiveKeys         int                    `json:"active_keys"`
	CoolingKeys        int                    `json:"cooling_keys"`
	InvalidKeys        int                    `json:"invalid_keys"`
	Keys               []apiProviderKeyDetail `json:"keys"`
	Models             []string               `json:"models"`
	StripTools         bool                   `json:"strip_tools"`
}

type apiProviderKeyDetail struct {
	KeyID         string    `json:"key_id"`
	MaskedKey     string    `json:"masked_key"`
	State         string    `json:"state"`
	ReqCount      int64     `json:"req_count"`
	ErrCount      int64     `json:"err_count"`
	InFlight      int64     `json:"in_flight"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	CoolUntil     time.Time `json:"cool_until,omitempty"`
	Last401At     time.Time `json:"last_401_at,omitempty"`
	InvalidReason string    `json:"invalid_reason,omitempty"`
	Label         string    `json:"label,omitempty"`
	Note          string    `json:"note,omitempty"`
	Disabled      bool      `json:"disabled,omitempty"`
}

type apiProviderCreatePayload struct {
	ID        string   `json:"id"`
	TargetURL string   `json:"target_url"`
	Keys      []string `json:"keys"`
}

type apiProviderUpdatePayload struct {
	TargetURL string `json:"target_url"`
}

type apiKeysPayload struct {
	Keys []string `json:"keys"`
}

type apiKeyMetadataPayload struct {
	Label    *string `json:"label,omitempty"`
	Note     *string `json:"note,omitempty"`
	Disabled *bool   `json:"disabled,omitempty"`
}

type apiKeysPreviewPayload struct {
	Mode string   `json:"mode"`
	Keys []string `json:"keys"`
}

type apiKeyPreviewEntry struct {
	KeyID     string `json:"key_id"`
	MaskedKey string `json:"masked_key"`
	Label     string `json:"label,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
}

type apiKeysPreviewResponse struct {
	Mode            string               `json:"mode"`
	InputCount      int                  `json:"input_count"`
	NormalizedCount int                  `json:"normalized_count"`
	DuplicateCount  int                  `json:"duplicate_count"`
	ExistingCount   int                  `json:"existing_count"`
	NewCount        int                  `json:"new_count"`
	RemovedCount    int                  `json:"removed_count"`
	ExistingKeys    []apiKeyPreviewEntry `json:"existing_keys"`
	NewKeys         []apiKeyPreviewEntry `json:"new_keys"`
	RemovedKeys     []apiKeyPreviewEntry `json:"removed_keys"`
}

type apiKeyTestResponse struct {
	OK                bool   `json:"ok"`
	StatusCode        int    `json:"status_code"`
	LatencyMs         int64  `json:"latency_ms"`
	Scope             string `json:"scope,omitempty"`
	Error             string `json:"error,omitempty"`
	RetryAfterSeconds int64  `json:"retry_after_seconds,omitempty"`
}

type apiKeysResetAllResponse struct {
	OK         bool `json:"ok"`
	ResetCount int  `json:"reset_count"`
}

type apiDeleteKeysPayload struct {
	KeyIDs []string `json:"key_ids"`
}

type apiModelsPayload struct {
	Models []string `json:"models"`
}

type apiDashboardResponse struct {
	ActiveProvider  string                         `json:"active_provider"`
	ProviderCount   int                            `json:"provider_count"`
	ActiveKeys      int                            `json:"active_keys"`
	CoolingKeys     int                            `json:"cooling_keys"`
	InvalidKeys     int                            `json:"invalid_keys"`
	ProviderCircuit *proxy.ProviderCircuitSnapshot `json:"provider_circuit,omitempty"`
	Stats           apiStatsHealth                 `json:"stats"`
	Providers       []apiProviderSummary           `json:"providers"`
	Events          []AdminEvent                   `json:"events"`
}

type apiStatsHealth struct {
	Enabled        bool   `json:"enabled"`
	DroppedRecords uint64 `json:"dropped_records"`
	QueueDepth     int    `json:"queue_depth"`
	QueueCapacity  int    `json:"queue_capacity"`
}

type apiProvidersResponse struct {
	ActiveProvider string               `json:"active_provider"`
	Providers      []apiProviderSummary `json:"providers"`
}

type apiSettingsResponse struct {
	Settings              apiSettingsPayload `json:"settings"`
	HotReloadFields       []string           `json:"hot_reload_fields"`
	RestartRequiredFields []string           `json:"restart_required_fields"`
}

type apiChangeResponse struct {
	OK                    bool     `json:"ok"`
	ActiveProvider        string   `json:"active_provider,omitempty"`
	ChangedFields         []string `json:"changed_fields,omitempty"`
	HotReloadedFields     []string `json:"hot_reloaded_fields,omitempty"`
	RestartRequiredFields []string `json:"restart_required_fields,omitempty"`
}

type apiAboutResponse struct {
	AppName         string   `json:"app_name"`
	Version         string   `json:"version"`
	GoVersion       string   `json:"go_version"`
	Platform        string   `json:"platform"`
	BuildTime       string   `json:"build_time"`
	ConfigPath      string   `json:"config_path"`
	Listen          string   `json:"listen"`
	AdminListen     string   `json:"admin_listen"`
	StateFile       string   `json:"state_file"`
	ActiveProvider  string   `json:"active_provider"`
	ProviderCount   int      `json:"provider_count"`
	Features        []string `json:"features"`
	ApiEndpoints    []string `json:"api_endpoints"`
	BackupEndpoints []string `json:"backup_endpoints"`
}

const adminMaxRequestBody = 1 << 20 // 1 MB

func decodeJSONBody[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, adminMaxRequestBody)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "request body too large"})
			return false
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return false
	}
	return true
}

func (h *Handler) requireConfigManager(w http.ResponseWriter) bool {
	if h.cfgManager != nil {
		return true
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
	return false
}

// registerAPIRoutes 注册管理台 API 路由。
func (h *Handler) registerAPIRoutes(mux *http.ServeMux) {
	auth := h.requireAuth
	mux.HandleFunc("/admin/api/v1/dashboard", auth(h.dashboard))
	mux.HandleFunc("/admin/api/v1/providers", auth(h.providersIndex))
	mux.HandleFunc("/admin/api/v1/providers/", auth(h.providersDetail))
	mux.HandleFunc("/admin/api/v1/settings", auth(h.settings))
	mux.HandleFunc("/admin/api/v1/events", auth(h.eventsIndex))
	mux.HandleFunc("/admin/api/v1/about", auth(h.about))
	mux.HandleFunc("/admin/api/v1/reload", auth(h.apiReload))
	mux.HandleFunc("/admin/api/v1/stats/summary", auth(h.statsSummary))
	mux.HandleFunc("/admin/api/v1/stats/models", auth(h.statsModels))
	mux.HandleFunc("/admin/api/v1/stats/recent", auth(h.statsRecent))
	mux.HandleFunc("/admin/api/v1/stats/logs", auth(h.statsLogs))
	mux.HandleFunc("/admin/api/v1/config/backup", auth(h.backupConfig))
	mux.HandleFunc("/admin/api/v1/state/backup", auth(h.backupState))
}

// dashboard 输出面向前端首页的聚合视图。
func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	activeStatus, err := h.pools.ActiveStatus()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	providers := h.buildProviderSummaries()
	resp := apiDashboardResponse{
		ActiveProvider:  activeStatus.ID,
		ProviderCount:   len(providers),
		ActiveKeys:      activeStatus.ActiveKeys,
		CoolingKeys:     activeStatus.CoolingKeys,
		InvalidKeys:     activeStatus.InvalidKeys,
		ProviderCircuit: h.providerCircuitSnapshot(),
		Stats:           h.statsHealth(),
		Providers:       providers,
		Events:          h.listEvents(10),
	}
	writeJSON(w, http.StatusOK, resp)
}

// providersIndex 输出 provider 列表与状态汇总。
func (h *Handler) providersIndex(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listProviders(w, r)
	case http.MethodPost:
		h.createProvider(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

// listProviders 输出 provider 列表与状态汇总。
func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	activeID := h.pools.ActiveID()
	resp := apiProvidersResponse{
		ActiveProvider: activeID,
		Providers:      h.buildProviderSummaries(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// providersDetail 处理 provider 明细与写操作。
func (h *Handler) providersDetail(w http.ResponseWriter, r *http.Request) {
	id, action, keyID, ok := h.parseProviderAction(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		switch r.Method {
		case http.MethodGet:
			h.providerDetail(w, r, id)
		case http.MethodPut:
			h.updateProvider(w, r, id)
		case http.MethodDelete:
			h.deleteProvider(w, r, id)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		}
	case "activate":
		h.activateProvider(w, r, id)
	case "keys:append":
		h.appendProviderKeys(w, r, id)
	case "keys:replace":
		h.replaceProviderKeys(w, r, id)
	case "keys:delete":
		h.deleteProviderKeys(w, r, id)
	case "keys:preview":
		h.previewProviderKeys(w, r, id)
	case "keys:reset-all":
		h.resetAllProviderKeys(w, r, id)
	case "key:reset":
		h.resetProviderKey(w, r, id, keyID)
	case "key:metadata":
		h.updateProviderKeyMetadata(w, r, id, keyID)
	case "key:test":
		h.testProviderKey(w, r, id, keyID)
	case "models:replace":
		h.replaceProviderModels(w, r, id)
	case "models:fetch":
		h.fetchProviderModels(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

// providerDetail 返回单个 provider 的详细状态。
func (h *Handler) providerDetail(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, err, "failed to read configuration")
		return
	}
	providerCfg, ok := findProviderConfig(cfg.ProviderConfigs(), id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}
	poolStatus, err := h.pools.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}

	keyStatuses := poolStatus.Status()
	keyDetails := buildProviderKeyDetails(providerCfg, keyStatuses)
	summary := buildProviderSummary(providerCfg, id == h.pools.ActiveID(), keyStatuses)
	writeJSON(w, http.StatusOK, apiProviderDetail{
		ID:                 summary.ID,
		Active:             summary.Active,
		TargetURL:          summary.TargetURL,
		TotalKeys:          summary.TotalKeys,
		DisabledKeys:       summary.DisabledKeys,
		QuotaExhaustedKeys: summary.QuotaExhaustedKeys,
		ActiveKeys:         summary.ActiveKeys,
		CoolingKeys:        summary.CoolingKeys,
		InvalidKeys:        summary.InvalidKeys,
		Keys:               keyDetails,
		Models:             safeModels(providerCfg.Models),
		StripTools:         providerCfg.StripTools,
	})
}

// activateProvider 切换 active provider，并通过配置管理器提交变更。
func (h *Handler) activateProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		cfg.ActiveProvider = id
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.activate_failed", "provider activation failed", map[string]any{
			"provider_id": id,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.activate_ok", "provider activated", map[string]any{
		"provider_id": id,
	})
	writeJSON(w, http.StatusOK, apiChangeResponse{
		OK:                    true,
		ActiveProvider:        id,
		ChangedFields:         result.ChangedFields,
		HotReloadedFields:     result.HotReloadedFields,
		RestartRequiredFields: result.RestartRequiredFields,
	})
}

// createProvider 新增一个 provider，并触发配置热重载。
func (h *Handler) createProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	var req apiProviderCreatePayload
	if !decodeJSONBody(w, r, &req) {
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		keys := normalizeKeys(req.Keys)
		if req.ID == "" {
			return fmt.Errorf("provider id is required")
		}
		if req.TargetURL == "" {
			return fmt.Errorf("target_url is required")
		}
		if len(keys) == 0 {
			return fmt.Errorf("at least one key is required")
		}
		if _, exists := findProviderConfig(cfg.Providers, req.ID); exists {
			return fmt.Errorf("provider %q already exists", req.ID)
		}
		cfg.Providers = append(cfg.Providers, config.ProviderConfig{
			ID:        req.ID,
			TargetURL: req.TargetURL,
			Keys:      keys,
		})
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.provider_create_failed", "provider create failed", map[string]any{
			"provider_id": req.ID,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.provider_created", "provider created", map[string]any{
		"provider_id": req.ID,
	})
	writeJSON(w, http.StatusCreated, apiChangeResponse{
		OK:                    true,
		ChangedFields:         result.ChangedFields,
		HotReloadedFields:     result.HotReloadedFields,
		RestartRequiredFields: result.RestartRequiredFields,
	})
}

// updateProvider 更新 provider 的基础信息，阶段 3 只开放 target_url 编辑。
func (h *Handler) updateProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	var req apiProviderUpdatePayload
	if !decodeJSONBody(w, r, &req) {
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		if req.TargetURL == "" {
			return fmt.Errorf("target_url is required")
		}
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return fmt.Errorf("provider not found")
		}
		cfg.Providers[idx].TargetURL = req.TargetURL
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.provider_update_failed", "provider update failed", map[string]any{
			"provider_id": id,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.provider_updated", "provider updated", map[string]any{
		"provider_id": id,
	})
	writeJSON(w, http.StatusOK, apiChangeResponse{
		OK:                    true,
		ChangedFields:         result.ChangedFields,
		HotReloadedFields:     result.HotReloadedFields,
		RestartRequiredFields: result.RestartRequiredFields,
	})
}

// deleteProvider 删除非活跃 provider，避免误删当前生效目标。
func (h *Handler) deleteProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		if cfg.ActiveProvider == id {
			return fmt.Errorf("cannot delete active provider")
		}
		if len(cfg.Providers) <= 1 {
			return fmt.Errorf("cannot delete the last provider")
		}
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return fmt.Errorf("provider not found")
		}
		cfg.Providers = append(cfg.Providers[:idx], cfg.Providers[idx+1:]...)
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.provider_delete_failed", "provider delete failed", map[string]any{
			"provider_id": id,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.provider_deleted", "provider deleted", map[string]any{
		"provider_id": id,
	})
	writeJSON(w, http.StatusOK, apiChangeResponse{
		OK:                    true,
		ChangedFields:         result.ChangedFields,
		HotReloadedFields:     result.HotReloadedFields,
		RestartRequiredFields: result.RestartRequiredFields,
	})
}

// appendProviderKeys 追加新的 keys，并自动去重保留原顺序。
func (h *Handler) appendProviderKeys(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	result, err := h.updateProviderKeys(id, r, func(existing []string, incoming []string) ([]string, error) {
		if len(incoming) == 0 {
			return nil, fmt.Errorf("at least one key is required")
		}
		return mergeKeys(existing, incoming), nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.keys_append_failed", "append provider keys failed", map[string]any{
			"provider_id": id,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.keys_appended", "provider keys appended", map[string]any{
		"provider_id": id,
	})
	writeJSON(w, http.StatusOK, h.toChangeResponse(result))
}

// replaceProviderKeys 全量替换 provider keys。
func (h *Handler) replaceProviderKeys(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	result, err := h.updateProviderKeys(id, r, func(_ []string, incoming []string) ([]string, error) {
		if len(incoming) == 0 {
			return nil, fmt.Errorf("at least one key is required")
		}
		return incoming, nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.keys_replace_failed", "replace provider keys failed", map[string]any{
			"provider_id": id,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.keys_replaced", "provider keys replaced", map[string]any{
		"provider_id": id,
	})
	writeJSON(w, http.StatusOK, h.toChangeResponse(result))
}

// deleteProviderKeys 按 key_id 删除 provider 内的 keys，但必须至少保留一个。
func (h *Handler) deleteProviderKeys(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	var req apiDeleteKeysPayload
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.KeyIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "at least one key_id is required"})
		return
	}

	deleteSet := make(map[string]struct{}, len(req.KeyIDs))
	for _, keyID := range req.KeyIDs {
		if keyID == "" {
			continue
		}
		deleteSet[keyID] = struct{}{}
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return fmt.Errorf("provider not found")
		}

		nextKeys := make([]string, 0, len(cfg.Providers[idx].Keys))
		for _, key := range cfg.Providers[idx].Keys {
			if _, shouldDelete := deleteSet[poolKeyID(key)]; shouldDelete {
				continue
			}
			nextKeys = append(nextKeys, key)
		}
		if len(nextKeys) == len(cfg.Providers[idx].Keys) {
			return fmt.Errorf("no matching keys were found")
		}
		if len(nextKeys) == 0 {
			return fmt.Errorf("provider must keep at least one key")
		}
		cfg.Providers[idx].Keys = nextKeys
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.keys_delete_failed", "delete provider keys failed", map[string]any{
			"provider_id": id,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.keys_deleted", "provider keys deleted", map[string]any{
		"provider_id": id,
	})
	writeJSON(w, http.StatusOK, h.toChangeResponse(result))
}

// resetProviderKey 手动恢复单个 key 为 active，并立即触发状态保存。
func (h *Handler) resetProviderKey(w http.ResponseWriter, r *http.Request, id, keyID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if keyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "key_id is required"})
		return
	}

	keyPool, err := h.pools.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	if err := keyPool.ResetKeyByID(keyID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	h.stateChanged(true)
	h.recordEvent("info", logx.CategoryAdmin, "admin.key_reset", "provider key reset", map[string]any{
		"provider_id": id,
		"key_id":      keyID,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// replaceProviderModels 替换 provider 的模型 ID 记录列表。
func (h *Handler) replaceProviderModels(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	var req apiModelsPayload
	if !decodeJSONBody(w, r, &req) {
		return
	}

	models := normalizeModels(req.Models)

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return fmt.Errorf("provider not found")
		}
		cfg.Providers[idx].Models = models
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryAdmin, "admin.models_replace_failed", "replace provider models failed", map[string]any{
			"provider_id": id,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryAdmin, "admin.models_replaced", "provider models replaced", map[string]any{
		"provider_id": id,
		"count":       len(models),
	})
	writeJSON(w, http.StatusOK, h.toChangeResponse(result))
}

// fetchProviderModels 从上游 API 拉取可用模型列表并返回（不自动保存）。
func (h *Handler) fetchProviderModels(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeAdminError(w, http.StatusServiceUnavailable, err, "failed to read configuration")
		return
	}
	providerCfg, ok := findProviderConfig(cfg.ProviderConfigs(), id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "provider not found"})
		return
	}

	keyPool, err := h.pools.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}

	// 取一把可用 key 用于认证上游请求。
	keyValue := keyPool.AnyKeyValue()
	if keyValue == "" && len(providerCfg.Keys) > 0 {
		keyValue = providerCfg.Keys[0]
	}
	if keyValue == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no available key for upstream request"})
		return
	}

	modelsURL, err := url.JoinPath(strings.TrimRight(providerCfg.TargetURL, "/"), "/models")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("build models url: %v", err)})
		return
	}
	allowedIPs, err := validateUpstreamURL(modelsURL)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "upstream URL is not allowed for security reasons"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("build request: %v", err)})
		return
	}
	reqHTTP.Header.Set("Authorization", "Bearer "+keyValue)
	reqHTTP.Header.Set("Accept", "application/json")

	client := safeUpstreamClient(allowedIPs, 15*time.Second)
	resp, err := client.Do(reqHTTP)
	if err != nil {
		h.recordEvent("warn", logx.CategoryAdmin, "admin.models_fetch_failed", "fetch upstream models failed", map[string]any{
			"provider_id": id,
			"url":         modelsURL,
			"error":       err.Error(),
		})
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("upstream request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		h.recordEvent("warn", logx.CategoryAdmin, "admin.models_fetch_failed", "fetch upstream models returned error", map[string]any{
			"provider_id": id,
			"status":      resp.StatusCode,
		})
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  fmt.Sprintf("upstream returned %d", resp.StatusCode),
			"detail": string(body),
		})
		return
	}

	var modelsResp openAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": fmt.Sprintf("parse upstream response: %v", err)})
		return
	}

	ids := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	sort.Strings(ids)

	h.recordEvent("info", logx.CategoryAdmin, "admin.models_fetched", "fetched upstream models", map[string]any{
		"provider_id": id,
		"count":       len(ids),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"models": ids,
		"count":  len(ids),
	})
}

// openAIModelsResponse 是 OpenAI 兼容 /v1/models 接口的标准响应结构。
type openAIModelsResponse struct {
	Data []openAIModel `json:"data"`
}

type openAIModel struct {
	ID string `json:"id"`
}

// normalizeModels 去重并排序模型 ID 列表，忽略空字符串。
func normalizeModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, m := range models {
		trimmed := strings.TrimSpace(m)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

// safeModels 返回模型列表的安全副本，nil 时返回空切片以避免前端处理 null。
func safeModels(models []string) []string {
	if models == nil {
		return []string{}
	}
	return append([]string(nil), models...)
}

// settings 输出当前配置的管理台可编辑视图。
func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSettings(w, r)
	case http.MethodPut:
		h.updateSettings(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

// getSettings 返回当前配置及字段生效分类。
func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiSettingsResponse{
		Settings: apiSettingsPayload{
			Listen:                          cfg.Listen,
			AdminListen:                     cfg.AdminListen,
			ActiveProvider:                  cfg.ActiveProvider,
			CoolingSeconds:                  cfg.CoolingSeconds,
			MaxRetries:                      cfg.MaxRetries,
			MaxTransientRetries:             intPtr(cfg.MaxTransientRetries),
			RequestTimeoutSeconds:           cfg.RequestTimeoutSeconds,
			ConnectTimeoutSeconds:           intPtr(cfg.ConnectTimeoutSeconds),
			ResponseHeaderTimeoutSeconds:    intPtr(cfg.ResponseHeaderTimeoutSeconds),
			TransientCoolingSeconds:         intPtr(cfg.TransientCoolingSeconds),
			WaitForKeyTimeoutMS:             intPtr(cfg.WaitForKeyTimeoutMS),
			StreamKeepAliveSeconds:          intPtr(cfg.StreamKeepAliveSeconds),
			StreamIdleTimeoutSeconds:        intPtr(cfg.StreamIdleTimeoutSeconds),
			StreamMaxDurationSeconds:        intPtr(cfg.StreamMaxDurationSeconds),
			ProviderCircuitFailureThreshold: intPtr(cfg.ProviderCircuitFailureThreshold),
			ProviderCircuitOpenSeconds:      intPtr(cfg.ProviderCircuitOpenSeconds),
			ProviderCircuitMaxOpenSeconds:   intPtr(cfg.ProviderCircuitMaxOpenSeconds),
			ProviderCircuitHalfOpenMax:      intPtr(cfg.ProviderCircuitHalfOpenMax),
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
			HasAdminAPIKey:                  cfg.AdminAPIKey != "",
		},
		HotReloadFields:       append([]string(nil), config.HotReloadFields...),
		RestartRequiredFields: append([]string(nil), config.RestartRequiredFields...),
	})
}

// updateSettings 保存管理台编辑后的配置，并触发热重载。
func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	var req apiSettingsPayload
	if !decodeJSONBody(w, r, &req) {
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		cfg.Listen = req.Listen
		cfg.AdminListen = req.AdminListen
		cfg.CoolingSeconds = req.CoolingSeconds
		cfg.MaxRetries = req.MaxRetries
		if req.MaxTransientRetries != nil {
			cfg.MaxTransientRetries = *req.MaxTransientRetries
		}
		cfg.RequestTimeoutSeconds = req.RequestTimeoutSeconds
		if req.ConnectTimeoutSeconds != nil {
			cfg.ConnectTimeoutSeconds = *req.ConnectTimeoutSeconds
		}
		if req.ResponseHeaderTimeoutSeconds != nil {
			cfg.ResponseHeaderTimeoutSeconds = *req.ResponseHeaderTimeoutSeconds
		}
		if req.TransientCoolingSeconds != nil {
			cfg.TransientCoolingSeconds = *req.TransientCoolingSeconds
		}
		if req.WaitForKeyTimeoutMS != nil {
			cfg.WaitForKeyTimeoutMS = *req.WaitForKeyTimeoutMS
		}
		if req.StreamKeepAliveSeconds != nil {
			cfg.StreamKeepAliveSeconds = *req.StreamKeepAliveSeconds
		}
		if req.StreamIdleTimeoutSeconds != nil {
			cfg.StreamIdleTimeoutSeconds = *req.StreamIdleTimeoutSeconds
		}
		if req.StreamMaxDurationSeconds != nil {
			cfg.StreamMaxDurationSeconds = *req.StreamMaxDurationSeconds
		}
		if req.ProviderCircuitFailureThreshold != nil {
			cfg.ProviderCircuitFailureThreshold = *req.ProviderCircuitFailureThreshold
		}
		if req.ProviderCircuitOpenSeconds != nil {
			cfg.ProviderCircuitOpenSeconds = *req.ProviderCircuitOpenSeconds
		}
		if req.ProviderCircuitMaxOpenSeconds != nil {
			cfg.ProviderCircuitMaxOpenSeconds = *req.ProviderCircuitMaxOpenSeconds
		}
		if req.ProviderCircuitHalfOpenMax != nil {
			cfg.ProviderCircuitHalfOpenMax = *req.ProviderCircuitHalfOpenMax
		}
		cfg.MaxBodyBytes = req.MaxBodyBytes
		cfg.LogLevel = req.LogLevel
		cfg.LogFormat = req.LogFormat
		cfg.LogOutput = req.LogOutput
		cfg.LogFile = req.LogFile
		cfg.LogMaxSizeMB = req.LogMaxSizeMB
		cfg.LogMaxBackups = req.LogMaxBackups
		cfg.LogMaxAgeDays = req.LogMaxAgeDays
		cfg.LogCompress = req.LogCompress
		enabled := req.PersistState
		cfg.PersistState = &enabled
		cfg.StateFile = req.StateFile
		cfg.InvalidTTLHours = req.InvalidTTLHours
		statsEnabled := req.StatsEnabled
		cfg.StatsEnabled = &statsEnabled
		cfg.StatsDir = req.StatsDir
		cfg.StatsRetentionDays = req.StatsRetentionDays
		cfg.StatsMaxRecentRecords = req.StatsMaxRecentRecords
		if req.AdminAPIKey != "" {
			cfg.AdminAPIKey = req.AdminAPIKey
		}
		return nil
	})
	if err != nil {
		h.recordEvent("error", logx.CategoryConfig, "config.save_failed", "settings update failed", map[string]any{
			"error": err.Error(),
		})
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	h.recordEvent("info", logx.CategoryConfig, "config.saved", "settings saved", map[string]any{
		"changed_fields": result.ChangedFields,
	})
	writeJSON(w, http.StatusOK, apiChangeResponse{
		OK:                    true,
		ChangedFields:         result.ChangedFields,
		HotReloadedFields:     result.HotReloadedFields,
		RestartRequiredFields: result.RestartRequiredFields,
	})
}

// eventsIndex 返回最近发生的管理事件。
func (h *Handler) eventsIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": h.listEvents(limit),
	})
}

// apiReload 复用主流程的热重载逻辑，供前端按钮和命令行均能触发。
func (h *Handler) apiReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}
	if err := h.runReload(logx.CategoryConfig, "config.reload_failed", "manual reload failed", "config.reloaded", "manual reload ok"); err != nil {
		h.recordEvent("error", logx.CategoryConfig, "config.reload_failed", "manual reload failed", map[string]any{
			"error": err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// about 输出管理台和运行环境信息，供关于页展示与交付排查使用。
func (h *Handler) about(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, apiAboutResponse{
		AppName:        "ModelMux Control Plane",
		Version:        buildVersion(),
		GoVersion:      runtime.Version(),
		Platform:       runtime.GOOS + "/" + runtime.GOARCH,
		BuildTime:      buildTimeValue(),
		ConfigPath:     h.cfgManager.Path(),
		Listen:         cfg.Listen,
		AdminListen:    cfg.AdminListen,
		StateFile:      cfg.StateFile,
		ActiveProvider: cfg.ActiveProvider,
		ProviderCount:  len(cfg.ProviderConfigs()),
		Features: []string{
			"多 provider 管理",
			"provider 内 key 轮询",
			"热重载",
			"状态持久化",
			"事件缓冲区",
			"调用统计",
			"配置与状态导出",
		},
		ApiEndpoints: []string{
			"GET /admin/api/v1/dashboard",
			"GET /admin/api/v1/providers",
			"GET /admin/api/v1/providers/{id}",
			"GET /admin/api/v1/stats/summary",
			"GET /admin/api/v1/stats/models",
			"GET /admin/api/v1/stats/recent",
			"GET /admin/api/v1/settings",
			"GET /admin/api/v1/events",
		},
		BackupEndpoints: []string{
			"POST /admin/api/v1/config/backup",
			"POST /admin/api/v1/state/backup",
		},
	})
}

// backupConfig 导出当前生效配置，供下载备份或交叉迁移使用。
// 安全说明：admin_api_key 脱敏为占位符，避免备份文件泄露管理密钥。
func (h *Handler) backupConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	// 脱敏 admin_api_key，避免备份文件泄露管理密钥
	if cfg.AdminAPIKey != "" {
		cfg.AdminAPIKey = "<REDACTED>"
	}

	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	downloadAttachment(w, "modelmux-config-backup.json", payload)
}

// backupState 导出当前运行状态，便于快速备份或迁移到其他机器。
func (h *Handler) backupState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if h.pools == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "provider pools are not ready"})
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	file := state.File{
		Version:   state.CurrentVersion,
		SavedAt:   time.Now(),
		Providers: h.pools.Snapshot(),
	}
	if len(file.Providers) == 0 && cfg.ActiveProvider != "" {
		file.Providers = []state.ProviderRecord{{
			ID:   cfg.ActiveProvider,
			Keys: nil,
		}}
	}

	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	downloadAttachment(w, "modelmux-state-backup.json", payload)
}

// buildProviderSummaries 把 provider pools 与配置文件合并成前端可读的汇总列表。
func (h *Handler) buildProviderSummaries() []apiProviderSummary {
	if h.cfgManager == nil {
		return nil
	}

	cfg, err := h.cfgManager.Snapshot()
	if err != nil {
		return nil
	}
	configByID := make(map[string]config.ProviderConfig, len(cfg.ProviderConfigs()))
	for _, provider := range cfg.ProviderConfigs() {
		configByID[provider.ID] = provider
	}

	statuses := h.pools.Status()
	out := make([]apiProviderSummary, 0, len(statuses))
	for _, status := range statuses {
		providerCfg, ok := configByID[status.ID]
		if !ok {
			continue
		}
		out = append(out, buildProviderSummary(providerCfg, status.Active, status.Keys))
	}
	return out
}

// buildProviderSummary 汇总单个 provider 的基础信息和状态统计。
func buildProviderSummary(providerCfg config.ProviderConfig, active bool, statuses []pool.KeyStatus) apiProviderSummary {
	summary := apiProviderSummary{
		ID:           providerCfg.ID,
		Active:       active,
		TargetURL:    providerCfg.TargetURL,
		TotalKeys:    len(statuses),
		DisabledKeys: providerCfg.DisabledKeyCount(),
		Models:       safeModels(providerCfg.Models),
	}
	for _, keyStatus := range statuses {
		switch keyStatus.State {
		case "active":
			summary.ActiveKeys++
		case "cooling":
			summary.CoolingKeys++
		case "invalid":
			summary.InvalidKeys++
			if keyStatus.InvalidReason == pool.InvalidReasonQuotaExhausted {
				summary.QuotaExhaustedKeys++
			}
		}
	}
	return summary
}

// parseProviderAction 解析 provider 路由下的明细、激活、key 批量操作与单 key 重置动作。
func (h *Handler) parseProviderAction(path string) (string, string, string, bool) {
	raw := strings.TrimPrefix(path, "/admin/api/v1/providers/")
	if raw == "" || raw == path {
		return "", "", "", false
	}
	parts := strings.Split(raw, "/")
	switch len(parts) {
	case 1:
		return parts[0], "", "", true
	case 2:
		if parts[1] == "activate" {
			return parts[0], "activate", "", true
		}
		if parts[1] == "keys:append" || parts[1] == "keys:replace" || parts[1] == "keys:delete" ||
			parts[1] == "keys:preview" || parts[1] == "keys:reset-all" {
			return parts[0], parts[1], "", true
		}
		if parts[1] == "models:replace" || parts[1] == "models:fetch" {
			return parts[0], parts[1], "", true
		}
		return "", "", "", false
	case 4:
		if parts[1] == "keys" {
			switch parts[3] {
			case "reset":
				return parts[0], "key:reset", parts[2], true
			case "metadata":
				return parts[0], "key:metadata", parts[2], true
			case "test":
				return parts[0], "key:test", parts[2], true
			}
		}
		return "", "", "", false
	default:
		return "", "", "", false
	}
}

// findProviderConfig 在配置快照中查找指定 provider。
func findProviderConfig(providers []config.ProviderConfig, id string) (config.ProviderConfig, bool) {
	for _, provider := range providers {
		if provider.ID == id {
			return provider, true
		}
	}
	return config.ProviderConfig{}, false
}

// findProviderIndex 返回 provider 在切片中的索引，找不到时返回 -1。
func findProviderIndex(providers []config.ProviderConfig, id string) int {
	for i, provider := range providers {
		if provider.ID == id {
			return i
		}
	}
	return -1
}

// updateProviderKeys 统一处理 key 载荷解析与配置更新，避免 append/replace 重复逻辑。
func (h *Handler) updateProviderKeys(id string, r *http.Request, mutator func(existing []string, incoming []string) ([]string, error)) (*config.UpdateResult, error) {
	if h.cfgManager == nil {
		return nil, fmt.Errorf("config manager is not ready")
	}
	if mutator == nil {
		return nil, fmt.Errorf("key mutator is required")
	}

	var req apiKeysPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid json body")
	}
	incoming := normalizeKeys(req.Keys)

	return h.cfgManager.Update(func(cfg *config.Config) error {
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return fmt.Errorf("provider not found")
		}
		nextKeys, err := mutator(append([]string(nil), cfg.Providers[idx].Keys...), incoming)
		if err != nil {
			return err
		}
		cfg.Providers[idx].Keys = nextKeys
		return nil
	})
}

// normalizeKeys 去掉空白项并保持首个出现顺序，避免配置中积累重复 key。
func normalizeKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// mergeKeys 以保留原有顺序为前提，把新增 key 追加到现有列表尾部并去重。
func mergeKeys(existing []string, incoming []string) []string {
	return normalizeKeys(append(append([]string(nil), existing...), incoming...))
}

// poolKeyID 为 provider 配置中的原始 key 生成与管理接口一致的稳定标识。
func poolKeyID(key string) string {
	return state.KeyID(key)
}

// intPtr 返回 int 指针，便于设置接口在响应中展示可选配置字段。
func intPtr(value int) *int {
	return &value
}

// toChangeResponse 把配置更新结果转换成统一的 API 响应结构。
func (h *Handler) toChangeResponse(result *config.UpdateResult) apiChangeResponse {
	if result == nil {
		return apiChangeResponse{OK: true}
	}
	return apiChangeResponse{
		OK:                    true,
		ChangedFields:         result.ChangedFields,
		HotReloadedFields:     result.HotReloadedFields,
		RestartRequiredFields: result.RestartRequiredFields,
	}
}

// listEvents 返回事件缓冲区中的最近事件。
func (h *Handler) listEvents(limit int) []AdminEvent {
	if h.eventBuffer == nil {
		return nil
	}
	return h.eventBuffer.List(limit)
}

func (h *Handler) providerCircuitSnapshot() *proxy.ProviderCircuitSnapshot {
	if h.healthReader == nil {
		return nil
	}
	snapshot := h.healthReader.ProviderCircuitSnapshot()
	return &snapshot
}

func (h *Handler) statsHealth() apiStatsHealth {
	return apiStatsHealth{
		Enabled:        h.statsStore != nil,
		DroppedRecords: h.droppedStatsRecords(),
		QueueDepth:     h.statsQueueDepth(),
		QueueCapacity:  h.statsQueueCapacity(),
	}
}

func (h *Handler) droppedStatsRecords() uint64 {
	reader, ok := h.statsStore.(statsHealthReader)
	if !ok || reader == nil {
		return 0
	}
	return reader.DroppedRecords()
}

func (h *Handler) statsQueueDepth() int {
	reader, ok := h.statsStore.(statsHealthReader)
	if !ok || reader == nil {
		return 0
	}
	return reader.QueueDepth()
}

func (h *Handler) statsQueueCapacity() int {
	reader, ok := h.statsStore.(statsHealthReader)
	if !ok || reader == nil {
		return 0
	}
	return reader.QueueCapacity()
}

// recordEvent 统一记录结构化事件，供前端页面和日志排查共享。
func (h *Handler) recordEvent(level, category, event, message string, data map[string]any) {
	entry := logx.Event{
		Level:    level,
		Category: category,
		Event:    event,
		Message:  message,
		Data:     data,
	}
	switch level {
	case "warn":
		slog.Warn(message, entry.Attrs()...)
	case "error":
		slog.Error(message, entry.Attrs()...)
	default:
		slog.Info(message, entry.Attrs()...)
	}
	if h.eventBuffer != nil {
		h.eventBuffer.AddEvent(entry)
	}
}

// writeJSON 写出统一 JSON 响应。
// writeAdminError 向客户端返回脱敏错误消息，同时在服务端日志记录完整错误。
func writeAdminError(w http.ResponseWriter, status int, err error, userMsg string) {
	slog.Error(userMsg, "err", err)
	writeJSON(w, status, map[string]any{"error": userMsg})
}

// validateUpstreamURL 校验上游 URL 不指向私有/保留 IP 段，防止 SSRF。
// 返回已解析且通过安全检查的 IP 列表，供调用方构造固定 IP 的 Transport 防 DNS 重绑定。
func validateUpstreamURL(rawURL string) ([]net.IP, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if isBlockedHost(host) {
		return nil, fmt.Errorf("host %q is not allowed", host)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		// DNS 解析失败时不阻塞请求：后续连接会再次解析并自行失败。
		return nil, nil
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, fmt.Errorf("resolved IP %s is in a restricted range", ip)
		}
	}
	return ips, nil
}

// safeUpstreamClient 构造一个绑定到已校验 IP 的 http.Client，避免 DNS 重绑定攻击。
// allowedIPs 为空时退化为普通 client（DNS 解析失败的兜底场景）。
func safeUpstreamClient(allowedIPs []net.IP, timeout time.Duration) *http.Client {
	if len(allowedIPs) == 0 {
		return &http.Client{Timeout: timeout}
	}
	allowed := make(map[string]bool, len(allowedIPs))
	for _, ip := range allowedIPs {
		allowed[ip.String()] = true
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if allowed[host] {
				return dialer.DialContext(ctx, network, addr)
			}
			return nil, fmt.Errorf("dial %s: IP not in allowlist", addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

func isBlockedHost(host string) bool {
	blocked := []string{
		"169.254.169.254",
		"metadata.google.internal",
		"metadata.google",
	}
	for _, b := range blocked {
		if strings.EqualFold(host, b) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// downloadAttachment 输出可下载的 JSON 附件。
func downloadAttachment(w http.ResponseWriter, filename string, payload []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

// buildVersion 返回构建版本，优先读取 Go build info 中的模块版本。
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return "dev"
}

// buildTime 可通过 -ldflags "-X github.com/kelongyan/ModelMux/admin.buildTime=..." 注入。
var buildTime = "dev"

func buildTimeValue() string {
	return buildTime
}
