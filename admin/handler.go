package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/claude-key-proxy/logx"
	"github.com/claude-key-proxy/pool"
)

type Handler struct {
	pools      *pool.ProviderPools
	configPath string
	reloadFn   func(string) error
}

func NewHandler(pools *pool.ProviderPools, configPath string, reloadFn func(string) error) *Handler {
	return &Handler{pools: pools, configPath: configPath, reloadFn: reloadFn}
}

// Register 注册管理端 HTTP 路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/status", h.status)
	mux.HandleFunc("/admin/health", h.health)
	mux.HandleFunc("/admin/reload", h.reload)
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
	if err := h.reloadFn(h.configPath); err != nil {
		slog.Error("admin reload failed", logx.Fields(logx.CategoryAdmin, logx.EventAdminReloadFailed,
			"err", err,
		)...)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	slog.Info("admin reload ok", logx.Fields(logx.CategoryAdmin, logx.EventAdminReloadOK)...)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// writeJSON 写出统一 JSON 响应。
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
