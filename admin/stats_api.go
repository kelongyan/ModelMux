package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/kelongyan/ModelMux/stats"
)

type apiStatsSummaryResponse struct {
	Window  string        `json:"window"`
	Since   time.Time     `json:"since"`
	Summary stats.Summary `json:"summary"`
}

type apiStatsModelsResponse struct {
	Window string               `json:"window"`
	Since  time.Time            `json:"since"`
	Models []stats.ModelSummary `json:"models"`
}

type apiStatsRecentResponse struct {
	Records []stats.CallRecord `json:"records"`
}

func (h *Handler) statsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	window, duration, ok := parseStatsWindow(r.URL.Query().Get("window"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "window must be one of: 1h, 24h, 7d, 30d"})
		return
	}
	since := time.Now().UTC().Add(-duration)
	resp := apiStatsSummaryResponse{
		Window: window,
		Since:  since,
	}
	if h.statsStore != nil {
		resp.Summary = h.statsStore.SummarySince(since)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) statsModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	window, duration, ok := parseStatsWindow(r.URL.Query().Get("window"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "window must be one of: 1h, 24h, 7d, 30d"})
		return
	}
	since := time.Now().UTC().Add(-duration)
	resp := apiStatsModelsResponse{
		Window: window,
		Since:  since,
	}
	if h.statsStore != nil {
		resp.Models = h.statsStore.ModelsSince(since)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) statsRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := parseStatsLimit(r.URL.Query().Get("limit"))
	resp := apiStatsRecentResponse{}
	if h.statsStore != nil {
		resp.Records = h.statsStore.Recent(limit)
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseStatsWindow(raw string) (string, time.Duration, bool) {
	if raw == "" {
		raw = "24h"
	}
	switch raw {
	case "1h":
		return raw, time.Hour, true
	case "24h":
		return raw, 24 * time.Hour, true
	case "7d":
		return raw, 7 * 24 * time.Hour, true
	case "30d":
		return raw, 30 * 24 * time.Hour, true
	default:
		return "", 0, false
	}
}

func parseStatsLimit(raw string) int {
	if raw == "" {
		return 100
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
