package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/claude-key-proxy/pool"
)

func TestStatusIncludesProviderSummary(t *testing.T) {
	pools, err := pool.NewProviderPools([]pool.ProviderSpec{
		{ID: "p1", Keys: []string{"k1"}},
		{ID: "p2", Keys: []string{"k2"}},
	}, "p1")
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}
	h := NewHandler(pools, "config.json", func(string) error { return nil })

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
	pools, err := pool.NewProviderPools([]pool.ProviderSpec{
		{ID: "p1", Keys: []string{"k1"}},
		{ID: "p2", Keys: []string{"k2"}},
	}, "p1")
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}
	p1, err := pools.Get("p1")
	if err != nil {
		t.Fatalf("Get(p1) error = %v", err)
	}
	p1Status := p1.Status()
	if len(p1Status) != 1 {
		t.Fatalf("len(Status()) = %d, want 1", len(p1Status))
	}
	p1Key, err := p1.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	p1Key.MarkInvalid()

	h := NewHandler(pools, "config.json", func(string) error { return nil })
	rr := httptest.NewRecorder()
	h.health(rr, httptest.NewRequest(http.MethodGet, "/admin/health", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}
