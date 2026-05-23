package admin

import (
	"encoding/json"
	"net/http"

	"github.com/claude-key-proxy/pool"
)

type Handler struct {
	pool       *pool.Pool
	configPath string
	reloadFn   func(string) error
}

func NewHandler(p *pool.Pool, configPath string, reloadFn func(string) error) *Handler {
	return &Handler{pool: p, configPath: configPath, reloadFn: reloadFn}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/admin/status", h.status)
	mux.HandleFunc("/admin/health", h.health)
	mux.HandleFunc("/admin/reload", h.reload)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	statuses := h.pool.Status()
	active, cooling, invalid := 0, 0, 0
	for _, s := range statuses {
		switch s.State {
		case "active":
			active++
		case "cooling":
			cooling++
		case "invalid":
			invalid++
		}
	}

	resp := map[string]any{
		"total_keys":   h.pool.TotalCount(),
		"active_keys":  active,
		"cooling_keys": cooling,
		"invalid_keys": invalid,
		"keys":         statuses,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	active := h.pool.ActiveCount()
	status := http.StatusOK
	if active == 0 {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"active_keys": active,
		"total_keys":  h.pool.TotalCount(),
		"ok":          active > 0,
	})
}

func (h *Handler) reload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.reloadFn(h.configPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
