package admin

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/proxy"
	"github.com/kelongyan/ModelMux/stats"
)

type Handler struct {
	pools        *pool.ProviderPools
	cfgManager   *config.Manager
	reloadFn     func(string) error
	eventBuffer  *EventBuffer
	stateChanged func(bool)
	statsStore   statsReader
	healthReader providerHealthReader
}

type statsReader interface {
	SummarySince(time.Time) stats.Summary
	ModelsSince(time.Time) []stats.ModelSummary
	Recent(limit int) []stats.CallRecord
	QueryLogs(since time.Time, filter stats.CallLogFilter) stats.CallLogResult
}

type statsHealthReader interface {
	DroppedRecords() uint64
	QueueDepth() int
	QueueCapacity() int
}

type providerHealthReader interface {
	ProviderCircuitSnapshot() proxy.ProviderCircuitSnapshot
}

// NewHandler 创建管理端处理器，并挂载配置管理器与事件缓冲区。
func NewHandler(pools *pool.ProviderPools, cfgManager *config.Manager, reloadFn func(string) error, events *EventBuffer, stateChanged func(bool)) *Handler {
	if stateChanged == nil {
		stateChanged = func(bool) {}
	}
	return &Handler{
		pools:        pools,
		cfgManager:   cfgManager,
		reloadFn:     reloadFn,
		eventBuffer:  events,
		stateChanged: stateChanged,
	}
}

// SetStatsStore 挂载调用统计读取器，供管理台统计 API 使用。
func (h *Handler) SetStatsStore(store statsReader) {
	h.statsStore = store
}

// SetProviderHealthReader 挂载代理运行健康读取器，供 dashboard 展示熔断状态。
func (h *Handler) SetProviderHealthReader(reader providerHealthReader) {
	h.healthReader = reader
}

// Register 注册管理端 HTTP 路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/status", h.requireAuth(h.status))
	mux.HandleFunc("/admin/health", h.health)
	mux.HandleFunc("/admin/reload", h.requireAuth(h.reload))
	h.registerAPIRoutes(mux)
	h.registerConsoleRoutes(mux)
}

// requireAuth 返回包装后的 handler，当 admin_api_key 已配置时校验请求认证。
func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.checkAuth(w, r) {
			return
		}
		next(w, r)
	}
}

// checkAuth 校验管理 API 认证；未配置 admin_api_key 时跳过校验。
func (h *Handler) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	cfg := config.Get()
	if cfg == nil || cfg.AdminAPIKey == "" {
		return true
	}
	key := extractAPIKey(r)
	if key == "" || key != cfg.AdminAPIKey {
		slog.Warn("admin auth failed", logx.Fields(logx.CategoryAdmin, logx.EventAdminAuthFailed,
			"remote_addr", r.RemoteAddr,
			"path", r.URL.Path,
		)...)
		h.eventBuffer.Add("warn", logx.CategoryAdmin, logx.EventAdminAuthFailed, "admin authentication failed", map[string]any{
			"remote_addr": r.RemoteAddr,
			"path":        r.URL.Path,
		})
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return false
	}
	return true
}

func extractAPIKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return r.Header.Get("X-Api-Key")
}

// status 输出 key 池状态，供本地排查和监控使用。
func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}

	activeStatus, err := h.pools.ActiveStatus()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
		return
	}

	resp := map[string]any{
		"active_provider": activeStatus.ID,
		"total_keys":      activeStatus.TotalKeys,
		"active_keys":     activeStatus.ActiveKeys,
		"cooling_keys":    activeStatus.CoolingKeys,
		"invalid_keys":    activeStatus.InvalidKeys,
		"keys":            activeStatus.Keys,
		"providers":       h.pools.Status(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// health 输出健康状态；综合检查 key 可用性、熔断状态和 stats 队列健康。
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	activeStatus, err := h.pools.ActiveStatus()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": "no active provider",
		})
		return
	}

	hasActiveKeys := activeStatus.ActiveKeys > 0
	circuitOpen := false
	if h.healthReader != nil {
		snap := h.healthReader.ProviderCircuitSnapshot()
		circuitOpen = snap.State == "open"
	}

	healthy := hasActiveKeys && !circuitOpen
	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	resp := map[string]any{
		"ok":              healthy,
		"active_provider": activeStatus.ID,
		"active_keys":     activeStatus.ActiveKeys,
		"total_keys":      activeStatus.TotalKeys,
		"circuit_open":    circuitOpen,
	}
	writeJSON(w, status, resp)
}

// reload 触发配置热重载，目前由调用方决定实际热更新哪些运行态对象。
func (h *Handler) reload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	if !h.requireConfigManager(w) {
		return
	}
	if err := h.runReload(logx.CategoryAdmin, logx.EventAdminReloadFailed, "legacy reload failed", logx.EventAdminReloadOK, "legacy reload ok"); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) runReload(category, errorEvent, errorMessage, successEvent, successMessage string) error {
	if err := h.reloadFn(h.cfgManager.Path()); err != nil {
		slog.Error(errorMessage, logx.Fields(category, errorEvent, "err", err)...)
		return err
	}
	slog.Info(successMessage, logx.Fields(category, successEvent)...)
	// reloadConfig 回调已成功记录 "config reloaded" 事件，这里不再重复。
	return nil
}
