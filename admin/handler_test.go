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

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/pool"
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
