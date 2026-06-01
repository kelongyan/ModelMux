package admin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/stats"
)

type Handler struct {
	pools        *pool.ProviderPools
	cfgManager   *config.Manager
	reloadFn     func(string) error
	eventBuffer  *EventBuffer
	stateChanged func(bool)
	statsStore   statsReader
}

type statsReader interface {
	SummarySince(time.Time) stats.Summary
	ModelsSince(time.Time) []stats.ModelSummary
	Recent(limit int) []stats.CallRecord
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

// Register 注册管理端 HTTP 路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/status", h.status)
	mux.HandleFunc("/admin/health", h.health)
	mux.HandleFunc("/admin/reload", h.reload)
	h.registerAPIRoutes(mux)
	h.registerConsoleRoutes(mux)
}

// status 输出 key 池状态，供本地排查和监控使用。
func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

// health 输出健康状态；只要存在可用 key 就认为服务可用。
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	activeStatus, err := h.pools.ActiveStatus()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	active := activeStatus.ActiveKeys
	status := http.StatusOK
	if active == 0 {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"active_provider": activeStatus.ID,
		"active_keys":     active,
		"total_keys":      activeStatus.TotalKeys,
		"ok":              active > 0,
	})
}

// reload 触发配置热重载，目前由调用方决定实际热更新哪些运行态对象。
func (h *Handler) reload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.cfgManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "config manager is not ready"})
		return
	}
	if err := h.reloadFn(h.cfgManager.Path()); err != nil {
		slog.Error("admin reload failed", logx.Fields(logx.CategoryAdmin, logx.EventAdminReloadFailed,
			"err", err,
		)...)
		h.recordEvent("error", logx.CategoryAdmin, logx.EventAdminReloadFailed, "legacy reload failed", map[string]any{
			"error": err.Error(),
		})
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	slog.Info("admin reload ok", logx.Fields(logx.CategoryAdmin, logx.EventAdminReloadOK)...)
	h.recordEvent("info", logx.CategoryAdmin, logx.EventAdminReloadOK, "legacy reload ok", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
