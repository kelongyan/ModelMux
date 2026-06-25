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

type apiStatsProvidersResponse struct {
	Window    string                  `json:"window"`
	Since     time.Time               `json:"since"`
	Providers []stats.ProviderSummary `json:"providers"`
}

type apiStatsTimelineResponse struct {
	Window      string                `json:"window"`
	Since       time.Time             `json:"since"`
	Granularity string                `json:"granularity"`
	Timeline    []stats.TimelinePoint `json:"timeline"`
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
	since, custom := parseStatsTimeRange(r, duration)
	if custom {
		window = "custom"
	}
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
	since, custom := parseStatsTimeRange(r, duration)
	if custom {
		window = "custom"
	}
	resp := apiStatsModelsResponse{
		Window: window,
		Since:  since,
	}
	if h.statsStore != nil {
		resp.Models = h.statsStore.ModelsSince(since)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) statsProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	window, duration, ok := parseStatsWindow(r.URL.Query().Get("window"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "window must be one of: 1h, 24h, 7d, 30d"})
		return
	}
	since, custom := parseStatsTimeRange(r, duration)
	if custom {
		window = "custom"
	}
	resp := apiStatsProvidersResponse{
		Window: window,
		Since:  since,
	}
	if h.statsStore != nil {
		resp.Providers = h.statsStore.SummaryByProvider(since)
	}
	if resp.Providers == nil {
		resp.Providers = []stats.ProviderSummary{}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) statsTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	window, duration, ok := parseStatsWindow(r.URL.Query().Get("window"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "window must be one of: 1h, 24h, 7d, 30d"})
		return
	}
	since, custom := parseStatsTimeRange(r, duration)
	if custom {
		window = "custom"
	}

	granularity := stats.TimelineGranularityHour
	if raw := r.URL.Query().Get("granularity"); raw != "" {
		switch raw {
		case "1h":
			granularity = stats.TimelineGranularityHour
		case "1d":
			granularity = stats.TimelineGranularityDay
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "granularity must be one of: 1h, 1d"})
			return
		}
	}

	resp := apiStatsTimelineResponse{
		Window:      window,
		Since:       since,
		Granularity: string(granularity),
	}
	if h.statsStore != nil {
		resp.Timeline = h.statsStore.TimelineSince(since, granularity)
	}
	if resp.Timeline == nil {
		resp.Timeline = []stats.TimelinePoint{}
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

// parseStatsTimeRange 解析自定义时间范围参数 from/to，优先于 window。
// 返回 since 时间和是否使用了自定义范围。
func parseStatsTimeRange(r *http.Request, defaultDuration time.Duration) (time.Time, bool) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" && toStr == "" {
		return time.Now().UTC().Add(-defaultDuration), false
	}

	var from, to time.Time
	var err error

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			// 尝试简化格式 YYYY-MM-DD
			from, err = time.Parse(time.DateOnly, fromStr)
		}
		if err != nil {
			return time.Time{}, false
		}
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			to, err = time.Parse(time.DateOnly, toStr)
		}
		if err != nil {
			return time.Time{}, false
		}
	} else {
		to = time.Now().UTC()
	}

	if from.IsZero() {
		// 只指定了 to，向前取默认窗口
		return to.Add(-defaultDuration), true
	}

	return from, true
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
	since, custom := parseStatsTimeRange(r, duration)
	if custom {
		window = "custom"
	}

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
