package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/kelongyan/ModelMux/stats"
)

type apiStatsSummaryResponse struct {
	Window         string        `json:"window"`
	Since          time.Time     `json:"since"`
	Summary        stats.Summary `json:"summary"`
	DroppedRecords uint64        `json:"dropped_records"`
	QueueDepth     int           `json:"queue_depth"`
	QueueCapacity  int           `json:"queue_capacity"`
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
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	window, duration, ok := parseStatsWindow(r.URL.Query().Get("window"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "window must be one of: 1h, 24h, 7d, 30d"})
		return
	}
	since := time.Now().UTC().Add(-duration)
	resp := apiStatsSummaryResponse{
		Window:         window,
		Since:          since,
		DroppedRecords: h.droppedStatsRecords(),
		QueueDepth:     h.statsQueueDepth(),
		QueueCapacity:  h.statsQueueCapacity(),
	}
	if h.statsStore != nil {
		resp.Summary = h.statsStore.SummarySince(since)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) statsModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
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
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
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

type apiStatsLogsResponse struct {
	Window   string             `json:"window"`
	Since    time.Time          `json:"since"`
	Records  []stats.CallRecord `json:"records"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

func (h *Handler) statsLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	window, duration, ok := parseStatsWindow(r.URL.Query().Get("window"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "window must be one of: 1h, 24h, 7d, 30d"})
		return
	}
	since := time.Now().UTC().Add(-duration)

	model := r.URL.Query().Get("model")
	status := r.URL.Query().Get("status")
	page := parseStatsPage(r.URL.Query().Get("page"))
	pageSize := parseStatsPageSize(r.URL.Query().Get("page_size"))

	resp := apiStatsLogsResponse{
		Window:   window,
		Since:    since,
		Page:     page,
		PageSize: pageSize,
	}
	if h.statsStore != nil {
		result := h.statsStore.QueryLogs(since, stats.CallLogFilter{
			Model:    model,
			Status:   status,
			Page:     page,
			PageSize: pageSize,
		})
		resp.Records = result.Records
		resp.Total = result.Total
		resp.Page = result.Page
		resp.PageSize = result.PageSize
	}
	if resp.Records == nil {
		resp.Records = []stats.CallRecord{}
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseStatsPage(raw string) int {
	if raw == "" {
		return 1
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page <= 0 {
		return 1
	}
	return page
}

func parseStatsPageSize(raw string) int {
	if raw == "" {
		return 20
	}
	size, err := strconv.Atoi(raw)
	if err != nil || size <= 0 {
		return 20
	}
	if size > 200 {
		return 200
	}
	return size
}
