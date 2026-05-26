package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/state"
)

type apiSettingsPayload struct {
	Listen                       string `json:"listen"`
	AdminListen                  string `json:"admin_listen"`
	ActiveProvider               string `json:"active_provider"`
	CoolingSeconds               int    `json:"cooling_seconds"`
	MaxRetries                   int    `json:"max_retries"`
	MaxTransientRetries          *int   `json:"max_transient_retries,omitempty"`
	RequestTimeoutSeconds        int    `json:"request_timeout_seconds"`
	ConnectTimeoutSeconds        *int   `json:"connect_timeout_seconds,omitempty"`
	ResponseHeaderTimeoutSeconds *int   `json:"response_header_timeout_seconds,omitempty"`
	TransientCoolingSeconds      *int   `json:"transient_cooling_seconds,omitempty"`
	WaitForKeyTimeoutMS          *int   `json:"wait_for_key_timeout_ms,omitempty"`
	MaxBodyBytes                 int64  `json:"max_body_bytes"`
	LogLevel                     string `json:"log_level"`
	LogFormat                    string `json:"log_format"`
	LogOutput                    string `json:"log_output"`
	LogFile                      string `json:"log_file"`
	LogMaxSizeMB                 int    `json:"log_max_size_mb"`
	LogMaxBackups                int    `json:"log_max_backups"`
	LogMaxAgeDays                int    `json:"log_max_age_days"`
	LogCompress                  bool   `json:"log_compress"`
	PersistState                 bool   `json:"persist_state"`
	StateFile                    string `json:"state_file"`
	InvalidTTLHours              int    `json:"invalid_ttl_hours"`
}

type apiProviderSummary struct {
	ID          string `json:"id"`
	Active      bool   `json:"active"`
	TargetURL   string `json:"target_url"`
	TotalKeys   int    `json:"total_keys"`
	ActiveKeys  int    `json:"active_keys"`
	CoolingKeys int    `json:"cooling_keys"`
	InvalidKeys int    `json:"invalid_keys"`
}

type apiProviderDetail struct {
	ID          string           `json:"id"`
	Active      bool             `json:"active"`
	TargetURL   string           `json:"target_url"`
	TotalKeys   int              `json:"total_keys"`
	ActiveKeys  int              `json:"active_keys"`
	CoolingKeys int              `json:"cooling_keys"`
	InvalidKeys int              `json:"invalid_keys"`
	Keys        []pool.KeyStatus `json:"keys"`
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

type apiDeleteKeysPayload struct {
	KeyIDs []string `json:"key_ids"`
}

type apiDashboardResponse struct {
	ActiveProvider string               `json:"active_provider"`
	ProviderCount  int                  `json:"provider_count"`
	ActiveKeys     int                  `json:"active_keys"`
	CoolingKeys    int                  `json:"cooling_keys"`
	InvalidKeys    int                  `json:"invalid_keys"`
	Providers      []apiProviderSummary `json:"providers"`
	Events         []AdminEvent         `json:"events"`
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

// registerAPIRoutes 注册管理台 API 路由。
func (h *Handler) registerAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/api/v1/dashboard", h.dashboard)
	mux.HandleFunc("/admin/api/v1/providers", h.providersIndex)
	mux.HandleFunc("/admin/api/v1/providers/", h.providersDetail)
	mux.HandleFunc("/admin/api/v1/settings", h.settings)
	mux.HandleFunc("/admin/api/v1/events", h.eventsIndex)
	mux.HandleFunc("/admin/api/v1/about", h.about)
	mux.HandleFunc("/admin/api/v1/reload", h.apiReload)
	mux.HandleFunc("/admin/api/v1/config/backup", h.backupConfig)
	mux.HandleFunc("/admin/api/v1/state/backup", h.backupState)
}

// dashboard 输出面向前端首页的聚合视图。
func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	activeStatus, err := h.pools.ActiveStatus()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	providers := h.buildProviderSummaries()
	resp := apiDashboardResponse{
		ActiveProvider: activeStatus.ID,
		ProviderCount:  len(providers),
		ActiveKeys:     activeStatus.ActiveKeys,
		CoolingKeys:    activeStatus.CoolingKeys,
		InvalidKeys:    activeStatus.InvalidKeys,
		Providers:      providers,
		Events:         h.listEvents(10),
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	case "activate":
		h.activateProvider(w, r, id)
	case "keys:append":
		h.appendProviderKeys(w, r, id)
	case "keys:replace":
		h.replaceProviderKeys(w, r, id)
	case "keys:delete":
		h.deleteProviderKeys(w, r, id)
	case "key:reset":
		h.resetProviderKey(w, r, id, keyID)
	default:
		http.NotFound(w, r)
	}
}

// providerDetail 返回单个 provider 的详细状态。
func (h *Handler) providerDetail(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

	keys := poolStatus.Status()
	summary := buildProviderSummary(providerCfg, id == h.pools.ActiveID(), keys)
	writeJSON(w, http.StatusOK, apiProviderDetail{
		ID:          summary.ID,
		Active:      summary.Active,
		TargetURL:   summary.TargetURL,
		TotalKeys:   summary.TotalKeys,
		ActiveKeys:  summary.ActiveKeys,
		CoolingKeys: summary.CoolingKeys,
		InvalidKeys: summary.InvalidKeys,
		Keys:        keys,
	})
}

// activateProvider 切换 active provider，并通过配置管理器提交变更。
func (h *Handler) activateProvider(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	var req apiProviderCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		keys := normalizeKeys(req.Keys)
		if req.ID == "" {
			return errorf("provider id is required")
		}
		if req.TargetURL == "" {
			return errorf("target_url is required")
		}
		if len(keys) == 0 {
			return errorf("at least one key is required")
		}
		if _, exists := findProviderConfig(cfg.Providers, req.ID); exists {
			return errorf("provider %q already exists", req.ID)
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	var req apiProviderUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		if req.TargetURL == "" {
			return errorf("target_url is required")
		}
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return errorf("provider not found")
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	result, err := h.cfgManager.Update(func(cfg *config.Config) error {
		if cfg.ActiveProvider == id {
			return errorf("cannot delete active provider")
		}
		if len(cfg.Providers) <= 1 {
			return errorf("cannot delete the last provider")
		}
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return errorf("provider not found")
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := h.updateProviderKeys(id, r, func(existing []string, incoming []string) ([]string, error) {
		if len(incoming) == 0 {
			return nil, errorf("at least one key is required")
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result, err := h.updateProviderKeys(id, r, func(_ []string, incoming []string) ([]string, error) {
		if len(incoming) == 0 {
			return nil, errorf("at least one key is required")
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}

	var req apiDeleteKeysPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
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
			return errorf("provider not found")
		}

		nextKeys := make([]string, 0, len(cfg.Providers[idx].Keys))
		for _, key := range cfg.Providers[idx].Keys {
			if _, shouldDelete := deleteSet[poolKeyID(key)]; shouldDelete {
				continue
			}
			nextKeys = append(nextKeys, key)
		}
		if len(nextKeys) == len(cfg.Providers[idx].Keys) {
			return errorf("no matching keys were found")
		}
		if len(nextKeys) == 0 {
			return errorf("provider must keep at least one key")
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

// settings 输出当前配置的管理台可编辑视图。
func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSettings(w, r)
	case http.MethodPut:
		h.updateSettings(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
			Listen:                       cfg.Listen,
			AdminListen:                  cfg.AdminListen,
			ActiveProvider:               cfg.ActiveProvider,
			CoolingSeconds:               cfg.CoolingSeconds,
			MaxRetries:                   cfg.MaxRetries,
			MaxTransientRetries:          intPtr(cfg.MaxTransientRetries),
			RequestTimeoutSeconds:        cfg.RequestTimeoutSeconds,
			ConnectTimeoutSeconds:        intPtr(cfg.ConnectTimeoutSeconds),
			ResponseHeaderTimeoutSeconds: intPtr(cfg.ResponseHeaderTimeoutSeconds),
			TransientCoolingSeconds:      intPtr(cfg.TransientCoolingSeconds),
			WaitForKeyTimeoutMS:          intPtr(cfg.WaitForKeyTimeoutMS),
			MaxBodyBytes:                 cfg.MaxBodyBytes,
			LogLevel:                     cfg.LogLevel,
			LogFormat:                    cfg.LogFormat,
			LogOutput:                    cfg.LogOutput,
			LogFile:                      cfg.LogFile,
			LogMaxSizeMB:                 cfg.LogMaxSizeMB,
			LogMaxBackups:                cfg.LogMaxBackups,
			LogMaxAgeDays:                cfg.LogMaxAgeDays,
			LogCompress:                  cfg.LogCompress,
			PersistState:                 cfg.StatePersistenceEnabled(),
			StateFile:                    cfg.StateFile,
			InvalidTTLHours:              cfg.InvalidTTLHours,
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}
	if err := h.reloadFn(h.cfgManager.Path()); err != nil {
		h.recordEvent("error", logx.CategoryConfig, "config.reload_failed", "manual reload failed", map[string]any{
			"error": err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	h.recordEvent("info", logx.CategoryConfig, "config.reloaded", "manual reload ok", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// about 输出管理台和运行环境信息，供关于页展示与交付排查使用。
func (h *Handler) about(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
		BuildTime:      buildTime(),
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
			"配置与状态导出",
		},
		ApiEndpoints: []string{
			"GET /admin/api/v1/dashboard",
			"GET /admin/api/v1/providers",
			"GET /admin/api/v1/providers/{id}",
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
func (h *Handler) backupConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
		ID:        providerCfg.ID,
		Active:    active,
		TargetURL: providerCfg.TargetURL,
	}
	for _, keyStatus := range statuses {
		summary.TotalKeys++
		switch keyStatus.State {
		case "active":
			summary.ActiveKeys++
		case "cooling":
			summary.CoolingKeys++
		case "invalid":
			summary.InvalidKeys++
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
		if parts[1] == "keys:append" || parts[1] == "keys:replace" || parts[1] == "keys:delete" {
			return parts[0], parts[1], "", true
		}
		return "", "", "", false
	case 4:
		if parts[1] == "keys" && parts[3] == "reset" {
			return parts[0], "key:reset", parts[2], true
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
		return nil, errorf("config manager is not ready")
	}
	if mutator == nil {
		return nil, errorf("key mutator is required")
	}

	var req apiKeysPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, errorf("invalid json body")
	}
	incoming := normalizeKeys(req.Keys)

	return h.cfgManager.Update(func(cfg *config.Config) error {
		idx := findProviderIndex(cfg.Providers, id)
		if idx < 0 {
			return errorf("provider not found")
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

// errorf 统一创建管理台面向用户的格式化错误。
func errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// listEvents 返回事件缓冲区中的最近事件。
func (h *Handler) listEvents(limit int) []AdminEvent {
	if h.eventBuffer == nil {
		return nil
	}
	return h.eventBuffer.List(limit)
}

// recordEvent 统一记录结构化事件，供前端页面和日志排查共享。
func (h *Handler) recordEvent(level, category, event, message string, data map[string]any) {
	if h.eventBuffer != nil {
		h.eventBuffer.Add(level, category, event, message, data)
	}
}

// writeJSON 写出统一 JSON 响应。
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

// buildVersion 返回构建版本；本地构建默认视为 dev。
func buildVersion() string {
	if info, ok := debugBuildInfo(); ok {
		return info
	}
	return "dev"
}

// buildTime 预留构建时间展示，当前本地构建场景下返回 dev。
func buildTime() string {
	return "dev"
}

// debugBuildInfo 读取 Go build info，避免关于页显示空版本。
func debugBuildInfo() (string, bool) {
	return "dev", true
}
