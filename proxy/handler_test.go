package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/claude-key-proxy/config"
	"github.com/claude-key-proxy/pool"
)

func mustHandler(t *testing.T, cfg *config.Config) (*Handler, *pool.ProviderPools) {
	t.Helper()
	providers := cfg.ProviderConfigs()
	if cfg.ActiveProvider == "" && len(providers) > 0 {
		cfg.ActiveProvider = providers[0].ID
	}
	specs := make([]pool.ProviderSpec, 0, len(providers))
	for _, provider := range providers {
		specs = append(specs, pool.ProviderSpec{ID: provider.ID, Keys: provider.Keys})
	}
	pools, err := pool.NewProviderPools(specs, cfg.ActiveProvider)
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}
	h, err := NewHandler(pools, cfg)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	return h, pools
}

func TestBuildRequestRewritesAnthropicAuthHeaders(t *testing.T) {
	cfg := &config.Config{
		TargetURL:             "https://example.com",
		Keys:                  []string{"rotated-key"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}

	h, pools := mustHandler(t, cfg)
	p, err := pools.Get(cfg.ActiveProvider)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", cfg.ActiveProvider, err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages?foo=bar", strings.NewReader(`{"x":1}`))
	req.Header.Set("Authorization", "Bearer original")
	req.Header.Set("X-Api-Key", "original-key")
	req.Header.Set("Content-Type", "application/json")

	key, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	outReq, err := h.buildRequest(req, key, []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}

	if got := outReq.Header.Get("Authorization"); got != "Bearer rotated-key" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer rotated-key")
	}
	if got := outReq.Header.Get("X-Api-Key"); got != "rotated-key" {
		t.Fatalf("X-Api-Key = %q, want %q", got, "rotated-key")
	}
	if got := outReq.URL.String(); got != "https://example.com/v1/messages?foo=bar" {
		t.Fatalf("URL = %q", got)
	}
}

func TestServeHTTPRetriesUnauthorizedWithOriginalBody(t *testing.T) {
	var attempts atomic.Int32
	var firstBody string
	var secondBody string
	var mu sync.Mutex
	var serverErr error

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			mu.Lock()
			serverErr = fmt.Errorf("ReadAll() error: %w", err)
			mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch attempts.Add(1) {
		case 1:
			firstBody = string(body)
			if got := r.Header.Get("X-Api-Key"); got != "k1" {
				mu.Lock()
				serverErr = fmt.Errorf("first X-Api-Key = %q, want %q", got, "k1")
				mu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		case 2:
			secondBody = string(body)
			if got := r.Header.Get("X-Api-Key"); got != "k2" {
				mu.Lock()
				serverErr = fmt.Errorf("second X-Api-Key = %q, want %q", got, "k2")
				mu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			mu.Lock()
			serverErr = fmt.Errorf("unexpected attempt %d", attempts.Load())
			mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:             upstream.URL,
		Keys:                  []string{"k1", "k2"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", "caller-key")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	mu.Lock()
	err := serverErr
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if firstBody != `{"prompt":"hi"}` {
		t.Fatalf("first body = %q", firstBody)
	}
	if secondBody != `{"prompt":"hi"}` {
		t.Fatalf("second body = %q", secondBody)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestUpdateConfigChangesTargetForNewRequests(t *testing.T) {
	var firstCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"server":"first"}`))
	}))
	defer first.Close()

	var secondCalls atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"server":"second"}`))
	}))
	defer second.Close()

	cfg := &config.Config{
		TargetURL:             first.URL,
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, pools := mustHandler(t, cfg)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", rr.Code, http.StatusOK)
	}

	nextCfg := &config.Config{
		TargetURL:             second.URL,
		Keys:                  []string{"k1"},
		ActiveProvider:        config.DefaultProviderID,
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	if err := pools.Update([]pool.ProviderSpec{{ID: nextCfg.ActiveProvider, Keys: nextCfg.Keys}}, nextCfg.ActiveProvider); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := h.UpdateConfig(nextCfg); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", rr.Code, http.StatusOK)
	}
	if firstCalls.Load() != 1 {
		t.Fatalf("first calls = %d, want 1", firstCalls.Load())
	}
	if secondCalls.Load() != 1 {
		t.Fatalf("second calls = %d, want 1", secondCalls.Load())
	}
}

func TestServeHTTPUsesOnlyActiveProvider(t *testing.T) {
	var firstCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		if got := r.Header.Get("X-Api-Key"); got != "p1-key" {
			t.Errorf("first X-Api-Key = %q, want p1-key", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer first.Close()

	var secondCalls atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls.Add(1)
		if got := r.Header.Get("X-Api-Key"); got != "p2-key" {
			t.Errorf("second X-Api-Key = %q, want p2-key", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()

	cfg := &config.Config{
		ActiveProvider: "p1",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: first.URL, Keys: []string{"p1-key"}},
			{ID: "p2", TargetURL: second.URL, Keys: []string{"p2-key"}},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, pools := mustHandler(t, cfg)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", rr.Code, http.StatusOK)
	}
	if firstCalls.Load() != 1 || secondCalls.Load() != 0 {
		t.Fatalf("calls before switch: first=%d second=%d, want 1/0", firstCalls.Load(), secondCalls.Load())
	}

	nextCfg := &config.Config{
		ActiveProvider: "p2",
		Providers: []config.ProviderConfig{
			{ID: "p1", TargetURL: first.URL, Keys: []string{"p1-key"}},
			{ID: "p2", TargetURL: second.URL, Keys: []string{"p2-key"}},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	if err := pools.Update([]pool.ProviderSpec{
		{ID: "p1", Keys: []string{"p1-key"}},
		{ID: "p2", Keys: []string{"p2-key"}},
	}, "p2"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := h.UpdateConfig(nextCfg); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", rr.Code, http.StatusOK)
	}
	if firstCalls.Load() != 1 || secondCalls.Load() != 1 {
		t.Fatalf("calls after switch: first=%d second=%d, want 1/1", firstCalls.Load(), secondCalls.Load())
	}
}

func TestUpdateConfigRejectsInvalidTargetAndKeepsOldRuntime(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:             upstream.URL,
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)

	nextCfg := &config.Config{
		TargetURL:             "%",
		Keys:                  []string{"k1"},
		ActiveProvider:        config.DefaultProviderID,
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	if err := h.UpdateConfig(nextCfg); err == nil {
		t.Fatal("UpdateConfig() error = nil, want invalid target error")
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
}

func TestServeHTTPSuccessTriggersDebouncedStateHook(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:             upstream.URL,
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)

	var immediate atomic.Bool
	var calls atomic.Int32
	h.SetStateChangeHook(func(now bool) {
		immediate.Store(now)
		calls.Add(1)
	})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if calls.Load() != 1 {
		t.Fatalf("state hook calls = %d, want 1", calls.Load())
	}
	if immediate.Load() {
		t.Fatal("state hook immediate = true, want false for successful request stats")
	}
}

func TestServeHTTPRetryableErrorTriggersImmediateStateHook(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:             upstream.URL,
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            0,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)

	var immediate atomic.Bool
	var calls atomic.Int32
	h.SetStateChangeHook(func(now bool) {
		immediate.Store(now)
		calls.Add(1)
	})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
	if calls.Load() == 0 {
		t.Fatal("state hook was not called")
	}
	if !immediate.Load() {
		t.Fatal("state hook immediate = false, want true for invalid key")
	}
}

func TestServeHTTPRejectsOversizedBodyBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:             upstream.URL,
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
		MaxBodyBytes:          4,
	}
	h, _ := mustHandler(t, cfg)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages", strings.NewReader("12345"))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
	if upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls.Load())
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v; body=%s", err, rr.Body.String())
	}
	if got := body["error"]["type"]; got != "proxy_error" {
		t.Fatalf("error.type = %q, want proxy_error", got)
	}
}

func TestWriteProxyErrorEscapesMessage(t *testing.T) {
	rr := httptest.NewRecorder()
	msg := "bad \"quote\"\nline"

	writeProxyError(rr, http.StatusBadGateway, msg)

	var body map[string]map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v; body=%s", err, rr.Body.String())
	}
	if got := body["error"]["message"]; got != "proxy: "+msg {
		t.Fatalf("message = %q, want %q", got, "proxy: "+msg)
	}
}

func TestReadRequestBodyLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test", strings.NewReader("1234"))
	body, err := readRequestBody(req, 4)
	if err != nil {
		t.Fatalf("readRequestBody() error = %v", err)
	}
	if string(body) != "1234" {
		t.Fatalf("body = %q, want 1234", string(body))
	}

	req = httptest.NewRequest(http.MethodPost, "http://proxy.test", strings.NewReader("12345"))
	_, err = readRequestBody(req, 4)
	if !errors.Is(err, errRequestBodyTooLarge) {
		t.Fatalf("error = %v, want errRequestBodyTooLarge", err)
	}
}

func TestStreamBodyReturnsWriteError(t *testing.T) {
	wantErr := errors.New("write failed")
	writer := failingResponseWriter{err: wantErr}

	err := streamBody(writer, strings.NewReader("chunk"))

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

type failingResponseWriter struct {
	err error
}

func (w failingResponseWriter) Header() http.Header {
	return http.Header{}
}

func (w failingResponseWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func (w failingResponseWriter) WriteHeader(int) {}
