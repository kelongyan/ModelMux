package proxy

import (
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

func TestBuildRequestRewritesAnthropicAuthHeaders(t *testing.T) {
	p := pool.New([]string{"rotated-key"})
	cfg := &config.Config{
		TargetURL:             "https://example.com",
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}

	h, err := NewHandler(p, cfg)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
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
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, err := NewHandler(pool.New([]string{"k1", "k2"}), cfg)
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", "caller-key")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	mu.Lock()
	err = serverErr
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
