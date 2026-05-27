package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/pool"
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

func TestBuildRequestRewritesUpstreamAuthHeaders(t *testing.T) {
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

func TestBuildRequestDoesNotDuplicateTargetPathPrefix(t *testing.T) {
	cfg := &config.Config{
		TargetURL:             "https://example.com/v1",
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
	key, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages?foo=bar", strings.NewReader(`{"x":1}`))
	outReq, err := h.buildRequest(req, key, []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
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

func TestServeHTTPRetriesTransportErrorWithNextKey(t *testing.T) {
	var attempts atomic.Int32
	var serverErr error
	var mu sync.Mutex

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch attempts.Add(1) {
		case 1:
			if got := r.Header.Get("X-Api-Key"); got != "k1" {
				mu.Lock()
				serverErr = fmt.Errorf("first X-Api-Key = %q, want %q", got, "k1")
				mu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				mu.Lock()
				serverErr = fmt.Errorf("response writer does not support hijacking")
				mu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				mu.Lock()
				serverErr = fmt.Errorf("Hijack() error: %w", err)
				mu.Unlock()
				return
			}
			_ = conn.Close()
		case 2:
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
		TargetURL:                    upstream.URL,
		Keys:                         []string{"k1", "k2"},
		RequestTimeoutSeconds:        10,
		ResponseHeaderTimeoutSeconds: 2,
		MaxRetries:                   1,
		CoolingSeconds:               1,
		TransientCoolingSeconds:      60,
	}
	h, pools := mustHandler(t, cfg)

	var immediateCalls atomic.Int32
	h.SetStateChangeHook(func(now bool) {
		if now {
			immediateCalls.Add(1)
		}
	})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	mu.Lock()
	err := serverErr
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	if immediateCalls.Load() == 0 {
		t.Fatal("state hook immediate = false, want true for transient key cooling")
	}
	status, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	if got := status.Keys[0].State; got != "cooling" {
		t.Fatalf("first key state = %q, want cooling", got)
	}
	if got := status.Keys[0].InFlight; got != 0 {
		t.Fatalf("first key in_flight = %d, want 0 after retry", got)
	}
	if got := status.Keys[1].State; got != "active" {
		t.Fatalf("second key state = %q, want active", got)
	}
	if got := status.Keys[1].InFlight; got != 0 {
		t.Fatalf("second key in_flight = %d, want 0 after success", got)
	}
}

func TestServeHTTPRetriesRetryableGatewayStatusWithNextKey(t *testing.T) {
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch attempts.Add(1) {
		case 1:
			if got := r.Header.Get("X-Api-Key"); got != "k1" {
				t.Errorf("first X-Api-Key = %q, want k1", got)
			}
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusServiceUnavailable)
		case 2:
			if got := r.Header.Get("X-Api-Key"); got != "k2" {
				t.Errorf("second X-Api-Key = %q, want k2", got)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected attempt %d", attempts.Load())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:               upstream.URL,
		Keys:                    []string{"k1", "k2"},
		RequestTimeoutSeconds:   10,
		MaxTransientRetries:     1,
		MaxRetries:              1,
		CoolingSeconds:          1,
		TransientCoolingSeconds: 1,
	}
	h, pools := mustHandler(t, cfg)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
	status, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	if got := status.Keys[0].State; got != "active" {
		t.Fatalf("first key state = %q, want active for provider-level transient", got)
	}
	if got := status.Keys[1].State; got != "active" {
		t.Fatalf("second key state = %q, want active", got)
	}
}

func TestRuntimeConfigUsesSplitTimeouts(t *testing.T) {
	cfg := &config.Config{
		TargetURL:                    "https://example.com",
		Keys:                         []string{"k1"},
		RequestTimeoutSeconds:        11,
		ConnectTimeoutSeconds:        3,
		ResponseHeaderTimeoutSeconds: 7,
		MaxTransientRetries:          2,
		TransientCoolingSeconds:      5,
		WaitForKeyTimeoutMS:          900,
		MaxRetries:                   1,
		CoolingSeconds:               1,
	}
	h, _ := mustHandler(t, cfg)

	rt := h.snapshot()
	if rt.client.Timeout != 11*time.Second {
		t.Fatalf("client timeout = %v, want 11s", rt.client.Timeout)
	}
	if rt.transport.ResponseHeaderTimeout != 7*time.Second {
		t.Fatalf("response header timeout = %v, want 7s", rt.transport.ResponseHeaderTimeout)
	}
	if rt.maxTransientRetries != 2 {
		t.Fatalf("maxTransientRetries = %d, want 2", rt.maxTransientRetries)
	}
	if rt.transientCoolingSeconds != 5 {
		t.Fatalf("transientCoolingSeconds = %d, want 5", rt.transientCoolingSeconds)
	}
	if rt.waitForKeyTimeout != 900*time.Millisecond {
		t.Fatalf("waitForKeyTimeout = %v, want 900ms", rt.waitForKeyTimeout)
	}
}

func TestServeHTTPLimitsProviderTransientRetries(t *testing.T) {
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:               upstream.URL,
		Keys:                    []string{"k1", "k2", "k3"},
		RequestTimeoutSeconds:   10,
		MaxRetries:              5,
		MaxTransientRetries:     1,
		CoolingSeconds:          1,
		TransientCoolingSeconds: 1,
	}
	h, pools := mustHandler(t, cfg)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2 because max_transient_retries=1", attempts.Load())
	}
	status, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	for _, key := range status.Keys {
		if key.State != "active" {
			t.Fatalf("key %s state = %q, want active for provider-level transient", key.MaskedKey, key.State)
		}
	}
}

func TestClassifyTransportRetryScope(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want retryScope
	}{
		{name: "eof is connection", err: io.EOF, want: retryScopeConnection},
		{name: "response header timeout is connection", err: errors.New("net/http: timeout awaiting response headers"), want: retryScopeConnection},
		{name: "connection refused is provider", err: errors.New("dial tcp 127.0.0.1:65535: connectex: No connection could be made because the target machine actively refused it"), want: retryScopeProvider},
		{name: "dns is provider", err: &net.DNSError{Err: "no such host", Name: "missing.example"}, want: retryScopeProvider},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyTransportRetryScope(tc.err); got != tc.want {
				t.Fatalf("classifyTransportRetryScope() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestServeHTTPWaitsForCoolingKeyRecovery(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:             upstream.URL,
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            0,
		CoolingSeconds:        1,
		WaitForKeyTimeoutMS:   80,
	}
	h, pools := mustHandler(t, cfg)

	keyPool, err := pools.Get(cfg.ActiveProvider)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", cfg.ActiveProvider, err)
	}
	keyPool.Status()
	keyPoolSnapshot, err := keyPool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	keyPoolSnapshot.FinishRequest()
	keyPoolSnapshot.MarkCooling(25 * time.Millisecond)

	start := time.Now()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("elapsed = %v, want proxy to wait for cooling key recovery", elapsed)
	}
}

func TestServeHTTPDoesNotWaitPastBudget(t *testing.T) {
	cfg := &config.Config{
		TargetURL:             "https://example.com",
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            0,
		CoolingSeconds:        1,
		WaitForKeyTimeoutMS:   10,
	}
	h, pools := mustHandler(t, cfg)

	keyPool, err := pools.Get(cfg.ActiveProvider)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", cfg.ActiveProvider, err)
	}
	key, err := keyPool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	key.FinishRequest()
	key.MarkCooling(60 * time.Millisecond)

	start := time.Now()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
	if elapsed := time.Since(start); elapsed > 40*time.Millisecond {
		t.Fatalf("elapsed = %v, want failure without long wait", elapsed)
	}
}

func TestServeHTTPRetriesQuotaExhaustedForbiddenWithOriginalBody(t *testing.T) {
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
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"预扣费额度失败, 用户剩余额度: 灵石0.288604, 需要预扣费额度: 灵石0.315356"}}`))
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
	h, pools := mustHandler(t, cfg)

	var sawImmediate atomic.Bool
	var hookCalls atomic.Int32
	h.SetStateChangeHook(func(now bool) {
		if now {
			sawImmediate.Store(true)
		}
		hookCalls.Add(1)
	})

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
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
	if hookCalls.Load() == 0 {
		t.Fatal("state hook was not called")
	}
	if !sawImmediate.Load() {
		t.Fatal("state hook never received immediate = true for quota exhausted key")
	}

	activeStatus, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	if got := activeStatus.InvalidKeys; got != 1 {
		t.Fatalf("invalid keys = %d, want 1", got)
	}
	if got := activeStatus.ActiveKeys; got != 1 {
		t.Fatalf("active keys = %d, want 1", got)
	}
}

func TestServeHTTPPassesThroughNonQuotaForbidden(t *testing.T) {
	var calls atomic.Int32
	const upstreamBody = `{"error":{"message":"model access denied"}}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("X-Upstream-Error", "permission")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(upstreamBody))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:             upstream.URL,
		Keys:                  []string{"k1", "k2"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            3,
		CoolingSeconds:        1,
	}
	h, pools := mustHandler(t, cfg)

	var hookCalls atomic.Int32
	h.SetStateChangeHook(func(bool) {
		hookCalls.Add(1)
	})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if got := rr.Body.String(); got != upstreamBody {
		t.Fatalf("body = %q, want %q", got, upstreamBody)
	}
	if got := rr.Header().Get("X-Upstream-Error"); got != "permission" {
		t.Fatalf("X-Upstream-Error = %q, want permission", got)
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}
	if hookCalls.Load() != 1 {
		t.Fatalf("state hook calls = %d, want 1 for forwarded response stats", hookCalls.Load())
	}

	activeStatus, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	if got := activeStatus.InvalidKeys; got != 0 {
		t.Fatalf("invalid keys = %d, want 0", got)
	}
	if got := activeStatus.ActiveKeys; got != 2 {
		t.Fatalf("active keys = %d, want 2", got)
	}
}

func TestIsQuotaExhaustedBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "chinese prepaid quota",
			body: "预扣费额度失败, 用户剩余额度: 灵石0.288604, 需要预扣费额度: 灵石0.315356",
			want: true,
		},
		{
			name: "english quota code",
			body: `{"type":"insufficient_quota","message":"quota exceeded"}`,
			want: true,
		},
		{
			name: "ordinary permission denied",
			body: `{"message":"model access denied"}`,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isQuotaExhaustedBody([]byte(tc.body)); got != tc.want {
				t.Fatalf("isQuotaExhaustedBody() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReadResponsePrefixReplaysFullBody(t *testing.T) {
	prefix, replayBody, err := readResponsePrefix(strings.NewReader("abcdef"), 3)
	if err != nil {
		t.Fatalf("readResponsePrefix() error = %v", err)
	}
	if string(prefix) != "abc" {
		t.Fatalf("prefix = %q, want abc", string(prefix))
	}
	replayed, err := io.ReadAll(replayBody)
	if err != nil {
		t.Fatalf("ReadAll(replayBody) error = %v", err)
	}
	if string(replayed) != "abcdef" {
		t.Fatalf("replayed body = %q, want abcdef", string(replayed))
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
