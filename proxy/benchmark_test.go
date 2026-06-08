package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/stats"
)

func BenchmarkServeHTTPSuccess(b *testing.B) {
	suppressBenchmarkLogs(b)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-bench","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`))
	}))
	defer upstream.Close()

	cfg := benchmarkConfig(upstream.URL, []string{"k1"})
	h, _ := mustHandler(b, cfg)

	body := []byte(`{"model":"bench-model","messages":[{"role":"user","content":"ping"}]}`)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	}
}

func BenchmarkServeHTTPSuccessWithStatsStore(b *testing.B) {
	suppressBenchmarkLogs(b)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-bench","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`))
	}))
	defer upstream.Close()

	cfg := benchmarkConfig(upstream.URL, []string{"k1"})
	h, _ := mustHandler(b, cfg)
	store, err := stats.NewStore(stats.Options{
		Dir:              b.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: b.N + 10,
	})
	if err != nil {
		b.Fatalf("NewStore() error = %v", err)
	}
	b.Cleanup(func() {
		if err := store.Close(); err != nil {
			b.Fatalf("Close() error = %v", err)
		}
	})
	h.SetStatsRecorder(store)

	body := []byte(`{"model":"bench-model","messages":[{"role":"user","content":"ping"}]}`)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	}
}

func BenchmarkServeHTTPUnauthorizedRetry(b *testing.B) {
	suppressBenchmarkLogs(b)

	var attempts atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1)%2 == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := benchmarkConfig(upstream.URL, []string{"k1", "k2"})
	cfg.MaxRetries = 1
	h, pools := mustHandler(b, cfg)

	body := []byte(`{"model":"bench-model","messages":[{"role":"user","content":"ping"}]}`)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		resetBenchmarkKeyStates(b, pools)
		b.StartTimer()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	}
}

func BenchmarkServeHTTPRateLimitRetry(b *testing.B) {
	suppressBenchmarkLogs(b)

	var attempts atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1)%2 == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := benchmarkConfig(upstream.URL, []string{"k1", "k2"})
	cfg.MaxRetries = 1
	cfg.CoolingSeconds = 1
	h, pools := mustHandler(b, cfg)

	body := []byte(`{"model":"bench-model","messages":[{"role":"user","content":"ping"}]}`)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		resetBenchmarkKeyStates(b, pools)
		b.StartTimer()
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	}
}

func BenchmarkServeHTTPProviderUnavailable(b *testing.B) {
	suppressBenchmarkLogs(b)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	cfg := benchmarkConfig(upstream.URL, []string{"k1", "k2", "k3"})
	cfg.MaxRetries = 5
	cfg.MaxTransientRetries = 1
	h, _ := mustHandler(b, cfg)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/models", nil)
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			b.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
		}
	}
}

func BenchmarkServeHTTPStreamingResponse(b *testing.B) {
	suppressBenchmarkLogs(b)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 3; i++ {
			_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"%d\"}}]}\n\n", i)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	cfg := benchmarkConfig(upstream.URL, []string{"k1"})
	h, _ := mustHandler(b, cfg)

	body := []byte(`{"model":"bench-model","stream":true,"messages":[{"role":"user","content":"ping"}]}`)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
		}
	}
}

func suppressBenchmarkLogs(tb testing.TB) {
	tb.Helper()
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	tb.Cleanup(func() {
		slog.SetDefault(previous)
	})
}

func benchmarkConfig(targetURL string, keys []string) *config.Config {
	return &config.Config{
		TargetURL:                    targetURL,
		Keys:                         keys,
		RequestTimeoutSeconds:        10,
		ConnectTimeoutSeconds:        2,
		ResponseHeaderTimeoutSeconds: 2,
		MaxRetries:                   1,
		MaxTransientRetries:          1,
		CoolingSeconds:               1,
		TransientCoolingSeconds:      1,
		WaitForKeyTimeoutMS:          1,
		MaxBodyBytes:                 config.DefaultMaxBodyBytes,
	}
}

func resetBenchmarkKeyStates(tb testing.TB, pools *pool.ProviderPools) {
	tb.Helper()
	activeID, keyPool, err := pools.Active()
	if err != nil {
		tb.Fatalf("Active() error = %v", err)
	}
	for _, key := range keyPool.Status() {
		if err := keyPool.ResetKeyByID(key.KeyID); err != nil {
			tb.Fatalf("ResetKeyByID(%s/%s) error = %v", activeID, key.KeyID, err)
		}
	}
}
