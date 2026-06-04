package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/stats"
)

func TestStatusIncludesProviderSummary(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	})

	rr := httptest.NewRecorder()
	h.status(rr, httptest.NewRequest(http.MethodGet, "/admin/status", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body["active_provider"] != "p1" {
		t.Fatalf("active_provider = %v, want p1", body["active_provider"])
	}
	if providers, ok := body["providers"].([]any); !ok || len(providers) != 2 {
		t.Fatalf("providers = %#v, want 2 providers", body["providers"])
	}
}

func TestHealthUsesActiveProviderOnly(t *testing.T) {
	h, pools, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	})
	p1, err := pools.Get("p1")
	if err != nil {
		t.Fatalf("Get(p1) error = %v", err)
	}
	p1Key, err := p1.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	p1Key.MarkInvalid()

	rr := httptest.NewRecorder()
	h.health(rr, httptest.NewRequest(http.MethodGet, "/admin/health", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestRegisterServesConsoleIndex(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/console/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if contentType := rr.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
}

func TestProvidersAPIListsProviders(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/api/v1/providers", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body apiProvidersResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.ActiveProvider != "p1" {
		t.Fatalf("ActiveProvider = %q, want p1", body.ActiveProvider)
	}
	if len(body.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(body.Providers))
	}
}

func TestCreateProviderAddsProvider(t *testing.T) {
	h, _, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	body := apiProviderCreatePayload{
		ID:        "p2",
		TargetURL: "https://two.example.com",
		Keys:      []string{"k2", "k3"},
	}
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/providers", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if _, ok := findProviderConfig(loaded.ProviderConfigs(), "p2"); !ok {
		t.Fatal("provider p2 should exist after create")
	}
}

func TestActivateProviderUpdatesConfigAndPools(t *testing.T) {
	h, pools, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/providers/p2/activate", nil)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := pools.ActiveID(); got != "p2" {
		t.Fatalf("ActiveID() = %q, want p2", got)
	}
	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if loaded.ActiveProvider != "p2" {
		t.Fatalf("ActiveProvider = %q, want p2", loaded.ActiveProvider)
	}
	if len(h.eventBuffer.List(10)) == 0 {
		t.Fatal("events buffer should contain activation event")
	}
}

func TestUpdateProviderTargetURL(t *testing.T) {
	h, _, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	buf, err := json.Marshal(apiProviderUpdatePayload{TargetURL: "https://updated.example.com"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/api/v1/providers/p1", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	provider, ok := findProviderConfig(loaded.ProviderConfigs(), "p1")
	if !ok {
		t.Fatal("provider p1 should exist")
	}
	if provider.TargetURL != "https://updated.example.com" {
		t.Fatalf("TargetURL = %q, want updated", provider.TargetURL)
	}
}

func TestDeleteProviderRemovesInactiveProvider(t *testing.T) {
	h, pools, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/v1/providers/p2", nil)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := pools.ProviderCount(); got != 1 {
		t.Fatalf("ProviderCount() = %d, want 1", got)
	}
	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if _, ok := findProviderConfig(loaded.ProviderConfigs(), "p2"); ok {
		t.Fatal("provider p2 should be deleted")
	}
}

func TestSettingsGetAndPut(t *testing.T) {
	h, _, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
		ConnectTimeoutSeconds:        4,
		MaxTransientRetries:          2,
		ResponseHeaderTimeoutSeconds: 9,
		TransientCoolingSeconds:      12,
		WaitForKeyTimeoutMS:          650,
		StatsDir:                     "custom_stats",
		StatsRetentionDays:           14,
		StatsMaxRecentRecords:        1234,
	})
	mux := http.NewServeMux()
	h.Register(mux)

	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, httptest.NewRequest(http.MethodGet, "/admin/api/v1/settings", nil))
	if getRR.Code != http.StatusOK {
		t.Fatalf("GET /settings status = %d, want %d", getRR.Code, http.StatusOK)
	}

	var resp apiSettingsResponse
	if err := json.Unmarshal(getRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GET /settings response invalid JSON: %v", err)
	}
	resp.Settings.CoolingSeconds = 88
	resp.Settings.LogLevel = "debug"
	resp.Settings.StatsEnabled = false
	resp.Settings.StatsDir = "next_stats"
	resp.Settings.StatsRetentionDays = 30
	resp.Settings.StatsMaxRecentRecords = 4321

	buf, err := json.Marshal(resp.Settings)
	if err != nil {
		t.Fatalf("marshal settings request error = %v", err)
	}
	putRR := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/api/v1/settings", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(putRR, req)
	if putRR.Code != http.StatusOK {
		t.Fatalf("PUT /settings status = %d, want %d, body=%s", putRR.Code, http.StatusOK, putRR.Body.String())
	}

	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	if loaded.CoolingSeconds != 88 {
		t.Fatalf("CoolingSeconds = %d, want 88", loaded.CoolingSeconds)
	}
	if loaded.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", loaded.LogLevel)
	}
	if loaded.ConnectTimeoutSeconds != 4 {
		t.Fatalf("ConnectTimeoutSeconds = %d, want 4", loaded.ConnectTimeoutSeconds)
	}
	if loaded.MaxTransientRetries != 2 {
		t.Fatalf("MaxTransientRetries = %d, want 2", loaded.MaxTransientRetries)
	}
	if loaded.ResponseHeaderTimeoutSeconds != 9 {
		t.Fatalf("ResponseHeaderTimeoutSeconds = %d, want 9", loaded.ResponseHeaderTimeoutSeconds)
	}
	if loaded.TransientCoolingSeconds != 12 {
		t.Fatalf("TransientCoolingSeconds = %d, want 12", loaded.TransientCoolingSeconds)
	}
	if loaded.WaitForKeyTimeoutMS != 650 {
		t.Fatalf("WaitForKeyTimeoutMS = %d, want 650", loaded.WaitForKeyTimeoutMS)
	}
	if loaded.StatsCollectionEnabled() {
		t.Fatal("StatsCollectionEnabled() = true, want false")
	}
	if loaded.StatsDir != "next_stats" {
		t.Fatalf("StatsDir = %q, want next_stats", loaded.StatsDir)
	}
	if loaded.StatsRetentionDays != 30 {
		t.Fatalf("StatsRetentionDays = %d, want 30", loaded.StatsRetentionDays)
	}
	if loaded.StatsMaxRecentRecords != 4321 {
		t.Fatalf("StatsMaxRecentRecords = %d, want 4321", loaded.StatsMaxRecentRecords)
	}
}

func TestAppendProviderKeys(t *testing.T) {
	h, _, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	buf, err := json.Marshal(apiKeysPayload{Keys: []string{"k2", "k3", "k2"}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/providers/p1/keys:append", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	provider, _ := findProviderConfig(loaded.ProviderConfigs(), "p1")
	if len(provider.Keys) != 3 {
		t.Fatalf("len(Keys) = %d, want 3", len(provider.Keys))
	}
}

func TestReplaceProviderKeys(t *testing.T) {
	h, _, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1", "k2"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	buf, err := json.Marshal(apiKeysPayload{Keys: []string{"k9"}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/providers/p1/keys:replace", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	provider, _ := findProviderConfig(loaded.ProviderConfigs(), "p1")
	if len(provider.Keys) != 1 || provider.Keys[0] != "k9" {
		t.Fatalf("Keys = %v, want [k9]", provider.Keys)
	}
}

func TestDeleteProviderKeys(t *testing.T) {
	h, _, path := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1", "k2", "k3"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	buf, err := json.Marshal(apiDeleteKeysPayload{KeyIDs: []string{poolKeyID("k2")}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/providers/p1/keys:delete", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	loaded, err := config.Read(path)
	if err != nil {
		t.Fatalf("Read(path) error = %v", err)
	}
	provider, _ := findProviderConfig(loaded.ProviderConfigs(), "p1")
	if len(provider.Keys) != 2 {
		t.Fatalf("len(Keys) = %d, want 2", len(provider.Keys))
	}
}

func TestResetProviderKey(t *testing.T) {
	var stateChangedCalls int
	h, pools, _ := newTestHandlerWithStateChange(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	}, func(bool) {
		stateChangedCalls++
	})
	keyPool, err := pools.Get("p1")
	if err != nil {
		t.Fatalf("Get(p1) error = %v", err)
	}
	key, err := keyPool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	key.MarkInvalid()

	mux := http.NewServeMux()
	h.Register(mux)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/v1/providers/p1/keys/"+poolKeyID("k1")+"/reset", nil)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if stateChangedCalls != 1 {
		t.Fatalf("stateChangedCalls = %d, want 1", stateChangedCalls)
	}
	status := keyPool.Status()
	if len(status) != 1 || status[0].State != "active" {
		t.Fatalf("status = %#v, want active", status)
	}
}

func TestEventsAPIReturnsRecentEvents(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	h.eventBuffer.Add("info", "test", "test.event", "hello", map[string]any{"k": "v"})

	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/api/v1/events?limit=1", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	events, ok := body["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v, want 1 event", body["events"])
	}
}

func TestAboutAPI(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/api/v1/about", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	var body apiAboutResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.AppName == "" || body.ConfigPath == "" {
		t.Fatalf("body = %#v, want app and config path", body)
	}
}

func TestBackupConfigAPI(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/admin/api/v1/config/backup", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if disposition := rr.Header().Get("Content-Disposition"); !strings.Contains(disposition, "modelmux-config-backup.json") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
}

func TestBackupStateAPI(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/admin/api/v1/state/backup", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if disposition := rr.Header().Get("Content-Disposition"); !strings.Contains(disposition, "modelmux-state-backup.json") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
}

func TestStatsAPIsReturnSummaryModelsAndRecentCalls(t *testing.T) {
	base := time.Now().UTC()
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	store, err := stats.NewStore(stats.Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 100,
		Now:              func() time.Time { return base },
	})
	if err != nil {
		t.Fatalf("stats.NewStore() error = %v", err)
	}
	if err := store.Append(stats.CallRecord{
		At:          base.Add(-10 * time.Minute),
		ProviderID:  "p1",
		Model:       "gpt-4.1-mini",
		Status:      http.StatusOK,
		Success:     true,
		LatencyMs:   120,
		TotalTokens: int64PtrAdmin(30),
		UsageSource: stats.UsageSourceUpstream,
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	h.SetStatsStore(store)

	mux := http.NewServeMux()
	h.Register(mux)

	summaryRecorder := httptest.NewRecorder()
	mux.ServeHTTP(summaryRecorder, httptest.NewRequest(http.MethodGet, "/admin/api/v1/stats/summary?window=24h", nil))
	if summaryRecorder.Code != http.StatusOK {
		t.Fatalf("summary status = %d, want %d, body=%s", summaryRecorder.Code, http.StatusOK, summaryRecorder.Body.String())
	}
	var summary struct {
		Window  string        `json:"window"`
		Summary stats.Summary `json:"summary"`
	}
	if err := json.Unmarshal(summaryRecorder.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary response: %v", err)
	}
	if summary.Window != "24h" || summary.Summary.TotalCalls != 1 || summary.Summary.TotalTokens != 30 {
		t.Fatalf("summary response = %+v, want 24h total_calls=1 total_tokens=30", summary)
	}

	modelsRecorder := httptest.NewRecorder()
	mux.ServeHTTP(modelsRecorder, httptest.NewRequest(http.MethodGet, "/admin/api/v1/stats/models?window=24h", nil))
	if modelsRecorder.Code != http.StatusOK {
		t.Fatalf("models status = %d, want %d, body=%s", modelsRecorder.Code, http.StatusOK, modelsRecorder.Body.String())
	}
	var models struct {
		Window string               `json:"window"`
		Models []stats.ModelSummary `json:"models"`
	}
	if err := json.Unmarshal(modelsRecorder.Body.Bytes(), &models); err != nil {
		t.Fatalf("decode models response: %v", err)
	}
	if len(models.Models) != 1 || models.Models[0].Model != "gpt-4.1-mini" {
		t.Fatalf("models response = %+v, want one gpt-4.1-mini row", models)
	}

	recentRecorder := httptest.NewRecorder()
	mux.ServeHTTP(recentRecorder, httptest.NewRequest(http.MethodGet, "/admin/api/v1/stats/recent?limit=10", nil))
	if recentRecorder.Code != http.StatusOK {
		t.Fatalf("recent status = %d, want %d, body=%s", recentRecorder.Code, http.StatusOK, recentRecorder.Body.String())
	}
	var recent struct {
		Records []stats.CallRecord `json:"records"`
	}
	if err := json.Unmarshal(recentRecorder.Body.Bytes(), &recent); err != nil {
		t.Fatalf("decode recent response: %v", err)
	}
	if len(recent.Records) != 1 || recent.Records[0].Model != "gpt-4.1-mini" {
		t.Fatalf("recent response = %+v, want one gpt-4.1-mini record", recent)
	}
}

func TestStatsSummaryRejectsInvalidWindow(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})
	store, err := stats.NewStore(stats.Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 100,
	})
	if err != nil {
		t.Fatalf("stats.NewStore() error = %v", err)
	}
	h.SetStatsStore(store)

	mux := http.NewServeMux()
	h.Register(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/admin/api/v1/stats/summary?window=3h", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestParseProviderActionRecognizesSupportedRoutes(t *testing.T) {
	h, _, _ := newTestHandler(t, &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	})

	cases := []struct {
		path    string
		id      string
		action  string
		keyID   string
		ok      bool
	}{
		{path: "/admin/api/v1/providers/p1", id: "p1", action: "", keyID: "", ok: true},
		{path: "/admin/api/v1/providers/p1/activate", id: "p1", action: "activate", keyID: "", ok: true},
		{path: "/admin/api/v1/providers/p1/keys:append", id: "p1", action: "keys:append", keyID: "", ok: true},
		{path: "/admin/api/v1/providers/p1/keys/abc/reset", id: "p1", action: "key:reset", keyID: "abc", ok: true},
		{path: "/admin/api/v1/providers", ok: false},
	}

	for _, tc := range cases {
		id, action, keyID, ok := h.parseProviderAction(tc.path)
		if id != tc.id || action != tc.action || keyID != tc.keyID || ok != tc.ok {
			t.Fatalf("parseProviderAction(%q) = (%q,%q,%q,%v), want (%q,%q,%q,%v)", tc.path, id, action, keyID, ok, tc.id, tc.action, tc.keyID, tc.ok)
		}
	}
}

func int64PtrAdmin(value int64) *int64 {
	return &value
}

func newTestHandler(t *testing.T, cfg *config.Config) (*Handler, *pool.ProviderPools, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := writeTestConfig(path, cfg); err != nil {
		t.Fatalf("writeTestConfig() error = %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	specs := providerSpecsFromConfigTest(loaded.ProviderConfigs())
	pools, err := pool.NewProviderPools(specs, loaded.ActiveProvider)
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}

	reloadFn := func(path string) error {
		nextCfg, err := config.Read(path)
		if err != nil {
			return err
		}
		if err := pools.Update(providerSpecsFromConfigTest(nextCfg.ProviderConfigs()), nextCfg.ActiveProvider); err != nil {
			return err
		}
		config.SetCurrent(nextCfg)
		return nil
	}

	manager := config.NewManager(path, reloadFn)
	events := NewEventBuffer(50)
	return NewHandler(pools, manager, reloadFn, events, nil), pools, path
}

func newTestHandlerWithStateChange(t *testing.T, cfg *config.Config, stateChanged func(bool)) (*Handler, *pool.ProviderPools, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := writeTestConfig(path, cfg); err != nil {
		t.Fatalf("writeTestConfig() error = %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	specs := providerSpecsFromConfigTest(loaded.ProviderConfigs())
	pools, err := pool.NewProviderPools(specs, loaded.ActiveProvider)
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}
	reloadFn := func(path string) error {
		nextCfg, err := config.Read(path)
		if err != nil {
			return err
		}
		if err := pools.Update(providerSpecsFromConfigTest(nextCfg.ProviderConfigs()), nextCfg.ActiveProvider); err != nil {
			return err
		}
		config.SetCurrent(nextCfg)
		return nil
	}
	manager := config.NewManager(path, reloadFn)
	events := NewEventBuffer(50)
	return NewHandler(pools, manager, reloadFn, events, stateChanged), pools, path
}

func writeTestConfig(path string, cfg *config.Config) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func providerSpecsFromConfigTest(providers []config.ProviderConfig) []pool.ProviderSpec {
	specs := make([]pool.ProviderSpec, 0, len(providers))
	for _, provider := range providers {
		specs = append(specs, pool.ProviderSpec{
			ID:   provider.ID,
			Keys: append([]string(nil), provider.Keys...),
		})
	}
	return specs
}
