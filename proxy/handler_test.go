package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/state"
	"github.com/kelongyan/ModelMux/stats"
)

func mustHandler(t testing.TB, cfg *config.Config) (*Handler, *pool.ProviderPools) {
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
	req.Header.Set("Accept-Encoding", "br")
	req.Header.Set("Content-Type", "application/json")

	key, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	outReq, err := buildRequest(h.snapshot(), req, key, []byte(`{"x":1}`), requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}

	if got := outReq.Header.Get("Authorization"); got != "Bearer rotated-key" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer rotated-key")
	}
	if got := outReq.Header.Get("X-Api-Key"); got != "rotated-key" {
		t.Fatalf("X-Api-Key = %q, want %q", got, "rotated-key")
	}
	if got := outReq.Header.Get("Accept-Encoding"); got != "" {
		t.Fatalf("Accept-Encoding = %q, want empty so transport can decode upstream errors", got)
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
	outReq, err := buildRequest(h.snapshot(), req, key, []byte(`{"x":1}`), requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	if got := outReq.URL.String(); got != "https://example.com/v1/messages?foo=bar" {
		t.Fatalf("URL = %q", got)
	}
}

func TestBuildRequestStripsToolFieldsWhenProviderRequiresIt(t *testing.T) {
	cfg := &config.Config{
		ActiveProvider: "thank",
		Providers: []config.ProviderConfig{
			{
				ID:         "thank",
				TargetURL:  "https://example.com/v1",
				Keys:       []string{"rotated-key"},
				StripTools: true,
			},
		},
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

	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"ping"}],"stream":true,"tools":[{"type":"function"}],"tool_choice":"auto","parallel_tool_calls":true}`)
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
	outReq, err := buildRequest(h.snapshot(), req, key, body, requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	outBody, err := io.ReadAll(outReq.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(outBody, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	for _, field := range []string{"tools", "tool_choice", "parallel_tool_calls"} {
		if _, ok := payload[field]; ok {
			t.Fatalf("payload still contains %q: %s", field, string(outBody))
		}
	}
	if payload["model"] != "claude-sonnet-4-6" {
		t.Fatalf("model = %v, want claude-sonnet-4-6", payload["model"])
	}
	if _, ok := payload["messages"]; !ok {
		t.Fatalf("payload missing messages: %s", string(outBody))
	}
}

func TestBuildRequestKeepsToolFieldsByDefault(t *testing.T) {
	cfg := &config.Config{
		ActiveProvider: "default",
		Providers: []config.ProviderConfig{
			{
				ID:        "default",
				TargetURL: "https://example.com/v1",
				Keys:      []string{"rotated-key"},
			},
		},
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

	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"ping"}],"tools":[{"type":"function"}],"tool_choice":"auto","parallel_tool_calls":true}`)
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
	outReq, err := buildRequest(h.snapshot(), req, key, body, requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	outBody, err := io.ReadAll(outReq.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(outBody) != string(body) {
		t.Fatalf("body changed by default:\ngot  %s\nwant %s", string(outBody), string(body))
	}
}

func TestBuildRequestAddsStreamUsageOptionForStreamingRequests(t *testing.T) {
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
	key, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	body := []byte(`{"model":"gpt-4.1-mini","stream":true,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
	outReq, err := buildRequest(h.snapshot(), req, key, body, requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	outBody, err := io.ReadAll(outReq.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var payload struct {
		Stream        bool `json:"stream"`
		StreamOptions struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
	}
	if err := json.Unmarshal(outBody, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !payload.Stream {
		t.Fatal("stream = false, want true")
	}
	if !payload.StreamOptions.IncludeUsage {
		t.Fatalf("stream_options.include_usage = false, want true; body=%s", string(outBody))
	}
}

func TestBuildRequestKeepsExplicitStreamUsageOption(t *testing.T) {
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
	key, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	body := []byte(`{"model":"gpt-4.1-mini","stream":true,"stream_options":{"include_usage":false},"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
	outReq, err := buildRequest(h.snapshot(), req, key, body, requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	outBody, err := io.ReadAll(outReq.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var payload struct {
		StreamOptions struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
	}
	if err := json.Unmarshal(outBody, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.StreamOptions.IncludeUsage {
		t.Fatalf("stream_options.include_usage = true, want explicit false preserved; body=%s", string(outBody))
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

func TestServeHTTPRecordsSuccessfulCallStats(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"usage": {
				"prompt_tokens": 11,
				"completion_tokens": 22,
				"total_tokens": 33
			}
		}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{
			{ID: "primary", TargetURL: upstream.URL, Keys: []string{"k1"}},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	recorder := &recordingStatsSink{}
	h.SetStatsRecorder(recorder)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"gpt-4.1-mini","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	record := records[0]
	if record.ProviderID != "primary" {
		t.Fatalf("ProviderID = %q, want primary", record.ProviderID)
	}
	if record.Model != "gpt-4.1-mini" {
		t.Fatalf("Model = %q, want gpt-4.1-mini", record.Model)
	}
	if record.Endpoint != "/v1/chat/completions" || record.Method != http.MethodPost {
		t.Fatalf("endpoint/method = %s %s, want POST /v1/chat/completions", record.Method, record.Endpoint)
	}
	if record.Status != http.StatusOK || !record.Success {
		t.Fatalf("status/success = %d/%v, want 200/true", record.Status, record.Success)
	}
	if record.Attempts != 1 {
		t.Fatalf("Attempts = %d, want 1", record.Attempts)
	}
	if record.KeyID != state.KeyID("k1") || record.KeyID == "k1" {
		t.Fatalf("KeyID = %q, want hashed key id", record.KeyID)
	}
	if record.UsageSource != stats.UsageSourceUpstream {
		t.Fatalf("UsageSource = %q, want %q", record.UsageSource, stats.UsageSourceUpstream)
	}
	if record.PromptTokens == nil || *record.PromptTokens != 11 {
		t.Fatalf("PromptTokens = %v, want 11", record.PromptTokens)
	}
	if record.CompletionTokens == nil || *record.CompletionTokens != 22 {
		t.Fatalf("CompletionTokens = %v, want 22", record.CompletionTokens)
	}
	if record.TotalTokens == nil || *record.TotalTokens != 33 {
		t.Fatalf("TotalTokens = %v, want 33", record.TotalTokens)
	}
}

func TestServeHTTPRecordsStreamingUsageStats(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":8,\"total_tokens\":15}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{
			{ID: "primary", TargetURL: upstream.URL, Keys: []string{"k1"}},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	recorder := &recordingStatsSink{}
	h.SetStatsRecorder(recorder)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"stream-model","stream":true,"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	record := records[0]
	if !record.Stream {
		t.Fatal("Stream = false, want true")
	}
	if record.UsageSource != stats.UsageSourceUpstream {
		t.Fatalf("UsageSource = %q, want %q", record.UsageSource, stats.UsageSourceUpstream)
	}
	if record.PromptTokens == nil || *record.PromptTokens != 7 {
		t.Fatalf("PromptTokens = %v, want 7", record.PromptTokens)
	}
	if record.CompletionTokens == nil || *record.CompletionTokens != 8 {
		t.Fatalf("CompletionTokens = %v, want 8", record.CompletionTokens)
	}
	if record.TotalTokens == nil || *record.TotalTokens != 15 {
		t.Fatalf("TotalTokens = %v, want 15", record.TotalTokens)
	}
}

func TestServeHTTPRecordsStreamingUsageStatsAfterLargeStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		chunk := strings.Repeat("x", 4096)
		for i := 0; i < 80; i++ {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"" + chunk + "\"}}]}\n\n"))
		}
		_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":70,\"completion_tokens\":80,\"total_tokens\":150}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{
			{ID: "primary", TargetURL: upstream.URL, Keys: []string{"k1"}},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	recorder := &recordingStatsSink{}
	h.SetStatsRecorder(recorder)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"large-stream-model","stream":true,"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	record := records[0]
	if record.UsageSource != stats.UsageSourceUpstream {
		t.Fatalf("UsageSource = %q, want %q", record.UsageSource, stats.UsageSourceUpstream)
	}
	if record.PromptTokens == nil || *record.PromptTokens != 70 {
		t.Fatalf("PromptTokens = %v, want 70", record.PromptTokens)
	}
	if record.CompletionTokens == nil || *record.CompletionTokens != 80 {
		t.Fatalf("CompletionTokens = %v, want 80", record.CompletionTokens)
	}
	if record.TotalTokens == nil || *record.TotalTokens != 150 {
		t.Fatalf("TotalTokens = %v, want 150", record.TotalTokens)
	}
}

func TestServeHTTPRecordsUpstreamStreamReadFailureDetails(t *testing.T) {
	readErr := errors.New("upstream read failed with api_key=secret-token")
	upstreamBody := &failingReadCloser{
		chunks: [][]byte{[]byte("data: partial\n\n")},
		err:    readErr,
	}
	transport := &scriptedTransport{
		responses: []scriptedResponse{{
			status: http.StatusOK,
			body:   upstreamBody,
		}},
	}
	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{{
			ID:        "primary",
			TargetURL: "https://upstream.test/v1",
			Keys:      []string{"key-a"},
		}},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	rt := h.snapshot()
	rt.client.Transport = transport
	rt.transport = nil

	recorder := &recordingStatsSink{}
	events := &recordingEventSink{}
	h.SetStatsRecorder(recorder)
	h.SetEventRecorder(events)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/responses", strings.NewReader(`{"model":"gpt-5.5","stream":true}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 because stream had already started", rr.Code)
	}
	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	record := records[0]
	if record.Success {
		t.Fatal("record.Success = true, want false")
	}
	if record.Error != "stream upstream read failed: upstream read failed with api_key=[REDACTED]" {
		t.Fatalf("record.Error = %q", record.Error)
	}

	event := events.Last()
	if event.Event != logx.EventProxyRequestFailed {
		t.Fatalf("event = %q, want %q", event.Event, logx.EventProxyRequestFailed)
	}
	if event.Data["stream_failure_side"] != "upstream_read" {
		t.Fatalf("stream_failure_side = %#v, want upstream_read", event.Data["stream_failure_side"])
	}
	if event.Data["stream_error"] != "upstream read failed with api_key=[REDACTED]" {
		t.Fatalf("stream_error = %#v", event.Data["stream_error"])
	}
	if strings.Contains(fmt.Sprint(event.Data["stream_error"]), "secret-token") {
		t.Fatalf("stream_error leaked secret: %#v", event.Data["stream_error"])
	}
}

func TestServeHTTPRecordsClientStreamWriteFailureDetails(t *testing.T) {
	writeErr := errors.New("client write failed")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: chunk\n\n"))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{{
			ID:        "primary",
			TargetURL: upstream.URL,
			Keys:      []string{"key-a"},
		}},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	recorder := &recordingStatsSink{}
	events := &recordingEventSink{}
	h.SetStatsRecorder(recorder)
	h.SetEventRecorder(events)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/responses", strings.NewReader(`{"model":"gpt-5.5","stream":true}`))
	h.ServeHTTP(failingResponseWriter{err: writeErr}, req)

	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	record := records[0]
	if record.Success {
		t.Fatal("record.Success = true, want false")
	}
	if record.Error != "stream client write failed: client write failed" {
		t.Fatalf("record.Error = %q", record.Error)
	}

	event := events.Last()
	if event.Event != logx.EventProxyRequestFailed {
		t.Fatalf("event = %q, want %q", event.Event, logx.EventProxyRequestFailed)
	}
	if event.Data["stream_failure_side"] != "client_write" {
		t.Fatalf("stream_failure_side = %#v, want client_write", event.Data["stream_failure_side"])
	}
	if event.Data["stream_error"] != "client write failed" {
		t.Fatalf("stream_error = %#v", event.Data["stream_error"])
	}
}

func TestServeHTTPRecordsClientCanceledStreamAsCancellation(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: chunk\n\n"))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{{
			ID:        "primary",
			TargetURL: upstream.URL,
			Keys:      []string{"key-a"},
		}},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	recorder := &recordingStatsSink{}
	events := &recordingEventSink{}
	h.SetStatsRecorder(recorder)
	h.SetEventRecorder(events)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/responses", strings.NewReader(`{"model":"gpt-5.5","stream":true}`))
	h.ServeHTTP(failingResponseWriter{err: context.Canceled}, req)

	if records := recorder.Records(); len(records) != 0 {
		t.Fatalf("records = %d, want 0 because client cancellation is not a proxy failure", len(records))
	}

	event := events.Last()
	if event.Event != logx.EventClientCanceled {
		t.Fatalf("event = %q, want %q", event.Event, logx.EventClientCanceled)
	}
	if event.Level != "info" {
		t.Fatalf("level = %q, want info", event.Level)
	}
	if event.Data["stream_failure_side"] != "client_canceled" {
		t.Fatalf("stream_failure_side = %#v, want client_canceled", event.Data["stream_failure_side"])
	}
	if event.Data["stream_error"] != "context canceled" {
		t.Fatalf("stream_error = %#v", event.Data["stream_error"])
	}
}

func TestServeHTTPRecordsOneCallAcrossRetries(t *testing.T) {
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch attempts.Add(1) {
		case 1:
			w.WriteHeader(http.StatusUnauthorized)
		case 2:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{
			{ID: "primary", TargetURL: upstream.URL, Keys: []string{"k1", "k2"}},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	recorder := &recordingStatsSink{}
	h.SetStatsRecorder(recorder)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"retry-model"}`)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	records := recorder.Records()
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	record := records[0]
	if record.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2", record.Attempts)
	}
	if record.KeyID != state.KeyID("k2") {
		t.Fatalf("KeyID = %q, want %q", record.KeyID, state.KeyID("k2"))
	}
	if record.Model != "retry-model" {
		t.Fatalf("Model = %q, want retry-model", record.Model)
	}
	if attempts.Load() != 2 {
		t.Fatalf("upstream attempts = %d, want 2", attempts.Load())
	}
}

func TestServeHTTPRecordsDiagnosticEventForStrippedTools(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll() error = %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.Contains(string(body), `"tools"`) {
			t.Errorf("upstream body still contains tools: %s", string(body))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "thank",
		Providers: []config.ProviderConfig{
			{ID: "thank", TargetURL: upstream.URL, Keys: []string{"k1"}, StripTools: true},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	events := &recordingEventSink{}
	h.SetEventRecorder(events)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"claude-sonnet-4-6","messages":[],"stream":true,"tools":[{"type":"function"}],"tool_choice":"auto"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	event := events.Last()
	if event.Event != logx.EventProxyRequestCompleted {
		t.Fatalf("event = %q, want %q", event.Event, logx.EventProxyRequestCompleted)
	}
	if event.RequestID == "" {
		t.Fatal("RequestID is empty")
	}
	if event.ProviderID != "thank" {
		t.Fatalf("ProviderID = %q, want thank", event.ProviderID)
	}
	if event.KeyID != state.KeyID("k1") {
		t.Fatalf("KeyID = %q, want %q", event.KeyID, state.KeyID("k1"))
	}
	if event.KeyHint != logx.MaskSecret("k1") {
		t.Fatalf("KeyHint = %q, want %q", event.KeyHint, logx.MaskSecret("k1"))
	}
	if event.Model != "claude-sonnet-4-6" || !event.Stream {
		t.Fatalf("model/stream = %q/%v, want claude-sonnet-4-6/true", event.Model, event.Stream)
	}
	if event.Status != http.StatusOK || event.Attempts != 1 {
		t.Fatalf("status/attempts = %d/%d, want 200/1", event.Status, event.Attempts)
	}
	if got, ok := event.Data["tools_present"].(bool); !ok || !got {
		t.Fatalf("tools_present = %v, want true", event.Data["tools_present"])
	}
	if got, ok := event.Data["tools_stripped"].(bool); !ok || !got {
		t.Fatalf("tools_stripped = %v, want true", event.Data["tools_stripped"])
	}
}

func TestServeHTTPRecordsDiagnosticEventForUpstreamBadRequest(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"tools are not supported by this provider"}}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{
			{ID: "primary", TargetURL: upstream.URL, Keys: []string{"k1"}},
		},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	events := &recordingEventSink{}
	h.SetEventRecorder(events)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"bad-model","messages":[],"tools":[{"type":"function"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	event := events.Last()
	if event.Level != "warn" {
		t.Fatalf("Level = %q, want warn", event.Level)
	}
	if event.Event != logx.EventProxyRequestFailed {
		t.Fatalf("event = %q, want %q", event.Event, logx.EventProxyRequestFailed)
	}
	if event.Status != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", event.Status)
	}
	if event.RequestID == "" {
		t.Fatal("RequestID is empty")
	}
	if event.RetryScope != retryScopeNone.String() {
		t.Fatalf("RetryScope = %q, want %q", event.RetryScope, retryScopeNone.String())
	}
	excerpt, _ := event.Data["upstream_error_excerpt"].(string)
	if !strings.Contains(excerpt, "tools are not supported") {
		t.Fatalf("upstream_error_excerpt = %q, want provider error text", excerpt)
	}
	if got, ok := event.Data["tools_present"].(bool); !ok || !got {
		t.Fatalf("tools_present = %v, want true", event.Data["tools_present"])
	}
	if got, ok := event.Data["tools_stripped"].(bool); !ok || got {
		t.Fatalf("tools_stripped = %v, want false", event.Data["tools_stripped"])
	}
}

type recordingStatsSink struct {
	mu      sync.Mutex
	records []stats.CallRecord
}

func (s *recordingStatsSink) Append(record stats.CallRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *recordingStatsSink) Records() []stats.CallRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stats.CallRecord(nil), s.records...)
}

type droppingStatsSink struct {
	dropped       atomic.Uint64
	queueDepth    int
	queueCapacity int
}

func (s *droppingStatsSink) Append(stats.CallRecord) error {
	s.dropped.Add(1)
	return nil
}

func (s *droppingStatsSink) DroppedRecords() uint64 {
	return s.dropped.Load()
}

func (s *droppingStatsSink) QueueDepth() int {
	return s.queueDepth
}

func (s *droppingStatsSink) QueueCapacity() int {
	return s.queueCapacity
}

type recordingEventSink struct {
	mu     sync.Mutex
	events []logx.Event
}

func (s *recordingEventSink) AddEvent(event logx.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *recordingEventSink) Last() logx.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) == 0 {
		return logx.Event{}
	}
	return s.events[len(s.events)-1]
}

func (s *recordingEventSink) Events() []logx.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]logx.Event(nil), s.events...)
}

func TestRecordCallStatsEmitsThrottledDroppedStatsEvent(t *testing.T) {
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	h := &Handler{}
	statsSink := &droppingStatsSink{queueDepth: 4096, queueCapacity: 4096}
	eventSink := &recordingEventSink{}
	h.SetStatsRecorder(statsSink)
	h.SetEventRecorder(eventSink)

	record := stats.CallRecord{ProviderID: "p1", Model: "gpt-4.1-mini"}
	h.recordCallStats(context.Background(), record)
	h.recordCallStats(context.Background(), record)

	events := filterEventsByName(eventSink.Events(), logx.EventStatsQueueDropped)
	if len(events) != 1 {
		t.Fatalf("dropped events = %d, want 1 inside throttle window", len(events))
	}
	assertStatsDroppedEvent(t, events[0], 1, 1, 4096, 4096)

	h.lastStatsDroppedEventUnixNano.Store(time.Now().Add(-statsDroppedEventInterval - time.Second).UnixNano())
	h.recordCallStats(context.Background(), record)

	events = filterEventsByName(eventSink.Events(), logx.EventStatsQueueDropped)
	if len(events) != 2 {
		t.Fatalf("dropped events = %d, want 2 after throttle window", len(events))
	}
	assertStatsDroppedEvent(t, events[1], 3, 2, 4096, 4096)
}

func filterEventsByName(events []logx.Event, name string) []logx.Event {
	out := make([]logx.Event, 0, len(events))
	for _, event := range events {
		if event.Event == name {
			out = append(out, event)
		}
	}
	return out
}

func assertStatsDroppedEvent(t *testing.T, event logx.Event, droppedRecords, droppedDelta uint64, queueDepth, queueCapacity int) {
	t.Helper()
	if event.Level != "warn" || event.Category != logx.CategoryStats {
		t.Fatalf("event level/category = %s/%s, want warn/%s", event.Level, event.Category, logx.CategoryStats)
	}
	if event.ProviderID != "p1" || event.Model != "gpt-4.1-mini" {
		t.Fatalf("event provider/model = %q/%q, want p1/gpt-4.1-mini", event.ProviderID, event.Model)
	}
	if got := event.Data["dropped_records"]; got != droppedRecords {
		t.Fatalf("dropped_records = %#v, want %d", got, droppedRecords)
	}
	if got := event.Data["dropped_delta"]; got != droppedDelta {
		t.Fatalf("dropped_delta = %#v, want %d", got, droppedDelta)
	}
	if got := event.Data["queue_depth"]; got != queueDepth {
		t.Fatalf("queue_depth = %#v, want %d", got, queueDepth)
	}
	if got := event.Data["queue_capacity"]; got != queueCapacity {
		t.Fatalf("queue_capacity = %#v, want %d", got, queueCapacity)
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
	remainingCooling := time.Until(status.Keys[0].CoolUntil)
	if remainingCooling <= 0 {
		t.Fatalf("first key remaining cooling = %v, want positive duration", remainingCooling)
	}
	if remainingCooling > 5*time.Second {
		t.Fatalf("first key remaining cooling = %v, want short connection-failure cooling", remainingCooling)
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

func TestServeHTTPDrainsRetryableStatusBodyBeforeRetry(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "unauthorized", status: http.StatusUnauthorized},
		{name: "rate limit", status: http.StatusTooManyRequests},
		{name: "bad gateway", status: http.StatusBadGateway},
		{name: "provider unavailable", status: http.StatusServiceUnavailable},
		{name: "gateway timeout", status: http.StatusGatewayTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			firstBody := newTrackingReadCloser(strings.Repeat("retry-body", 8))
			transport := &scriptedTransport{responses: []scriptedResponse{
				{status: tc.status, body: firstBody},
				{status: http.StatusOK, body: newTrackingReadCloser(`{"ok":true}`)},
			}}

			cfg := &config.Config{
				TargetURL:               "https://upstream.example.com",
				Keys:                    []string{"k1", "k2"},
				RequestTimeoutSeconds:   10,
				MaxRetries:              1,
				MaxTransientRetries:     1,
				CoolingSeconds:          1,
				TransientCoolingSeconds: 1,
			}
			h, _ := mustHandler(t, cfg)
			h.snapshot().client = &http.Client{Transport: transport}

			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
			}
			if got := transport.calls.Load(); got != 2 {
				t.Fatalf("upstream calls = %d, want 2", got)
			}
			if !firstBody.closed.Load() {
				t.Fatal("first retryable response body was not closed")
			}
			if got, want := firstBody.bytesRead.Load(), int64(len(firstBody.data)); got != want {
				t.Fatalf("retryable response bytes read = %d, want %d", got, want)
			}
		})
	}
}

func TestServeHTTPDrainsQuotaForbiddenBodyBeforeRetry(t *testing.T) {
	quotaMessage := `{"error":{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}}`
	body := quotaMessage + strings.Repeat("x", int(quotaErrorInspectBytes)-len(quotaMessage)+32)
	firstBody := newTrackingReadCloser(body)
	transport := &scriptedTransport{responses: []scriptedResponse{
		{status: http.StatusForbidden, body: firstBody},
		{status: http.StatusOK, body: newTrackingReadCloser(`{"ok":true}`)},
	}}

	cfg := &config.Config{
		TargetURL:             "https://upstream.example.com",
		Keys:                  []string{"k1", "k2"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)
	h.snapshot().client = &http.Client{Transport: transport}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages", strings.NewReader(`{"prompt":"hi"}`)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := transport.calls.Load(); got != 2 {
		t.Fatalf("upstream calls = %d, want 2", got)
	}
	if !firstBody.closed.Load() {
		t.Fatal("quota response body was not closed")
	}
	if got, want := firstBody.bytesRead.Load(), int64(len(firstBody.data)); got != want {
		t.Fatalf("quota response bytes read = %d, want %d", got, want)
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
	if rt.requestTimeout != 11*time.Second {
		t.Fatalf("requestTimeout = %v, want 11s", rt.requestTimeout)
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

func TestRuntimeConfigUsesProviderCircuitOptions(t *testing.T) {
	cfg := &config.Config{
		TargetURL:                       "https://example.com",
		Keys:                            []string{"k1"},
		RequestTimeoutSeconds:           10,
		MaxRetries:                      1,
		CoolingSeconds:                  1,
		ProviderCircuitFailureThreshold: 5,
		ProviderCircuitOpenSeconds:      7,
		ProviderCircuitMaxOpenSeconds:   42,
		ProviderCircuitHalfOpenMax:      2,
	}
	h, _ := mustHandler(t, cfg)

	circuit := h.snapshot().circuit
	if circuit.failureThreshold != 5 {
		t.Fatalf("failureThreshold = %d, want 5", circuit.failureThreshold)
	}
	if circuit.openCooling != 7*time.Second {
		t.Fatalf("openCooling = %v, want 7s", circuit.openCooling)
	}
	if circuit.maxOpenCooling != 42*time.Second {
		t.Fatalf("maxOpenCooling = %v, want 42s", circuit.maxOpenCooling)
	}
	if circuit.halfOpenMax != 2 {
		t.Fatalf("halfOpenMax = %d, want 2", circuit.halfOpenMax)
	}
}

func TestRuntimeConfigUsesStreamOptions(t *testing.T) {
	cfg := &config.Config{
		TargetURL:                "https://example.com",
		Keys:                     []string{"k1"},
		RequestTimeoutSeconds:    10,
		MaxRetries:               1,
		CoolingSeconds:           1,
		StreamKeepAliveSeconds:   2,
		StreamIdleTimeoutSeconds: 7,
		StreamMaxDurationSeconds: 42,
	}
	h, _ := mustHandler(t, cfg)

	rt := h.snapshot()
	if rt.streamKeepAlive != 2*time.Second {
		t.Fatalf("streamKeepAlive = %v, want 2s", rt.streamKeepAlive)
	}
	if rt.streamIdleTimeout != 7*time.Second {
		t.Fatalf("streamIdleTimeout = %v, want 7s", rt.streamIdleTimeout)
	}
	if rt.streamMaxDuration != 42*time.Second {
		t.Fatalf("streamMaxDuration = %v, want 42s", rt.streamMaxDuration)
	}
}

func TestRuntimeConfigUsesOptimizedUpstreamConnectionPool(t *testing.T) {
	cfg := &config.Config{
		TargetURL:             "https://example.com",
		Keys:                  []string{"k1"},
		RequestTimeoutSeconds: 10,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)

	rt := h.snapshot()
	if rt.transport.MaxIdleConns != 256 {
		t.Fatalf("MaxIdleConns = %d, want 256", rt.transport.MaxIdleConns)
	}
	if rt.transport.MaxIdleConnsPerHost != 64 {
		t.Fatalf("MaxIdleConnsPerHost = %d, want 64", rt.transport.MaxIdleConnsPerHost)
	}
	if !rt.transport.ForceAttemptHTTP2 {
		t.Fatal("ForceAttemptHTTP2 = false, want true")
	}
	if rt.transport.MaxConnsPerHost != 0 {
		t.Fatalf("MaxConnsPerHost = %d, want 0 for no artificial upstream concurrency cap", rt.transport.MaxConnsPerHost)
	}
}

func TestServeHTTPStreamingRequestIgnoresFixedClientTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"slow\"}}]}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(1200 * time.Millisecond)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{
			{ID: "primary", TargetURL: upstream.URL, Keys: []string{"k1"}},
		},
		RequestTimeoutSeconds: 1,
		MaxRetries:            1,
		CoolingSeconds:        1,
	}
	h, _ := mustHandler(t, cfg)

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"stream-model","stream":true,"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "data: [DONE]") {
		t.Fatalf("body = %q, want completed streaming response", rr.Body.String())
	}
}

func TestServeHTTPStreamingRequestWritesKeepAliveDuringUpstreamIdle(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(35 * time.Millisecond)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		ActiveProvider: "primary",
		Providers: []config.ProviderConfig{
			{ID: "primary", TargetURL: upstream.URL, Keys: []string{"k1"}},
		},
		RequestTimeoutSeconds:    10,
		MaxRetries:               1,
		CoolingSeconds:           1,
		StreamKeepAliveSeconds:   1,
		StreamIdleTimeoutSeconds: 2,
		StreamMaxDurationSeconds: 3,
	}
	h, _ := mustHandler(t, cfg)
	rt := h.snapshot()
	rt.streamKeepAlive = 10 * time.Millisecond
	rt.streamIdleTimeout = time.Second
	rt.streamMaxDuration = time.Second

	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(`{"model":"stream-model","stream":true,"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ": modelmux keepalive\n\n") {
		t.Fatalf("body = %q, want SSE keepalive comment", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "data: [DONE]") {
		t.Fatalf("body = %q, want completed stream", rr.Body.String())
	}
}

func TestRequestDiagnosticEventRedactsUpstreamErrorExcerpt(t *testing.T) {
	event := requestDiagnosticEvent(requestDiagnosticInput{
		level:                "warn",
		event:                logx.EventProxyRequestFailed,
		message:              "upstream rejected request",
		retryScope:           retryScopeNone,
		upstreamErrorExcerpt: `Authorization: Bearer sk-secret-123456` + "\n" + `api_key=abc123`,
	})

	excerpt, _ := event.Data["upstream_error_excerpt"].(string)
	if strings.Contains(excerpt, "sk-secret-123456") || strings.Contains(excerpt, "abc123") {
		t.Fatalf("upstream_error_excerpt leaked secret: %q", excerpt)
	}
	if !strings.Contains(excerpt, "[REDACTED]") {
		t.Fatalf("upstream_error_excerpt = %q, want redaction marker", excerpt)
	}
}

func TestExtractRequestMetaParsesModelStreamAndToolsPresence(t *testing.T) {
	meta := extractRequestMeta([]byte(`{"model":"gpt-4.1-mini","stream":true,"tools":[{"type":"function"}]}`))

	if meta.model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want gpt-4.1-mini", meta.model)
	}
	if !meta.stream {
		t.Fatal("stream = false, want true")
	}
	if !meta.toolsPresent {
		t.Fatal("toolsPresent = false, want true")
	}
}

func TestExtractRequestMetaIgnoresInvalidJSON(t *testing.T) {
	meta := extractRequestMeta([]byte(`{`))

	if meta.model != "" {
		t.Fatalf("model = %q, want empty", meta.model)
	}
	if meta.stream {
		t.Fatal("stream = true, want false")
	}
	if meta.toolsPresent {
		t.Fatal("toolsPresent = true, want false")
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

func TestServeHTTPProviderTransportFailureDoesNotPoisonKeys(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	targetURL := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close() error = %v", err)
	}

	cfg := &config.Config{
		TargetURL:                    targetURL,
		Keys:                         []string{"k1", "k2", "k3"},
		RequestTimeoutSeconds:        10,
		ConnectTimeoutSeconds:        1,
		ResponseHeaderTimeoutSeconds: 1,
		MaxRetries:                   5,
		MaxTransientRetries:          1,
		CoolingSeconds:               1,
		TransientCoolingSeconds:      1,
	}
	h, pools := mustHandler(t, cfg)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/models", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
	status, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	for _, key := range status.Keys {
		if key.State != "active" {
			t.Fatalf("key %s state = %q, want active for provider transport failure", key.MaskedKey, key.State)
		}
	}
}

func TestServeHTTPProviderCircuitOpensAndRejectsRequests(t *testing.T) {
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"provider down"}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:               upstream.URL,
		Keys:                    []string{"k1", "k2", "k3"},
		RequestTimeoutSeconds:   10,
		MaxRetries:              5,
		MaxTransientRetries:     2,
		CoolingSeconds:          1,
		TransientCoolingSeconds: 1,
	}
	h, pools := mustHandler(t, cfg)
	events := &recordingEventSink{}
	h.SetEventRecorder(events)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d, want %d, body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("upstream calls after first request = %d, want 3", got)
	}
	if got := h.snapshot().circuit.snapshot().state; got != providerCircuitStateOpen.String() {
		t.Fatalf("circuit state = %q, want open", got)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("second status = %d, want %d, body=%s", second.Code, http.StatusServiceUnavailable, second.Body.String())
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("upstream calls after rejected request = %d, want 3", got)
	}
	if !strings.Contains(second.Body.String(), "active provider is temporarily unavailable") {
		t.Fatalf("second body = %s, want circuit rejection message", second.Body.String())
	}
	if !hasEvent(events.Events(), logx.EventProviderCircuitOpened) {
		t.Fatalf("events did not include %q: %+v", logx.EventProviderCircuitOpened, events.Events())
	}
	if !hasEvent(events.Events(), logx.EventProviderCircuitRejected) {
		t.Fatalf("events did not include %q: %+v", logx.EventProviderCircuitRejected, events.Events())
	}

	status, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	for _, key := range status.Keys {
		if key.State != "active" {
			t.Fatalf("key %s state = %q, want active for provider circuit failure", key.MaskedKey, key.State)
		}
	}
}

func TestServeHTTPProviderCircuitUsesConfiguredFailureThreshold(t *testing.T) {
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"provider down"}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{
		TargetURL:                       upstream.URL,
		Keys:                            []string{"k1", "k2", "k3"},
		RequestTimeoutSeconds:           10,
		MaxRetries:                      5,
		MaxTransientRetries:             2,
		CoolingSeconds:                  1,
		TransientCoolingSeconds:         1,
		ProviderCircuitFailureThreshold: 4,
	}
	h, _ := mustHandler(t, cfg)

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if first.Code != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d, want %d, body=%s", first.Code, http.StatusServiceUnavailable, first.Body.String())
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("upstream calls after first request = %d, want 3", got)
	}
	if got := h.snapshot().circuit.snapshot().state; got != providerCircuitStateClosed.String() {
		t.Fatalf("circuit state after 3 failures = %q, want closed", got)
	}

	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("second status = %d, want %d, body=%s", second.Code, http.StatusServiceUnavailable, second.Body.String())
	}
	if got := attempts.Load(); got != 4 {
		t.Fatalf("upstream calls after threshold failure = %d, want 4", got)
	}
	if got := h.snapshot().circuit.snapshot().state; got != providerCircuitStateOpen.String() {
		t.Fatalf("circuit state after 4 failures = %q, want open", got)
	}

	rejected := httptest.NewRecorder()
	h.ServeHTTP(rejected, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if got := attempts.Load(); got != 4 {
		t.Fatalf("upstream calls after rejected request = %d, want 4", got)
	}
	if !strings.Contains(rejected.Body.String(), "active provider is temporarily unavailable") {
		t.Fatalf("rejected body = %s, want circuit rejection message", rejected.Body.String())
	}
}

func TestServeHTTPKeyScopeErrorsDoNotOpenProviderCircuit(t *testing.T) {
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch attempts.Add(1) {
		case 1:
			w.WriteHeader(http.StatusUnauthorized)
		case 2:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Errorf("unexpected attempt %d", attempts.Load())
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

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	snapshot := h.snapshot().circuit.snapshot()
	if snapshot.state != providerCircuitStateClosed.String() {
		t.Fatalf("circuit state = %q, want closed", snapshot.state)
	}
	if snapshot.consecutiveFailures != 0 {
		t.Fatalf("consecutiveFailures = %d, want 0", snapshot.consecutiveFailures)
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

func TestConnectionCoolingDurationAdaptsAndCaps(t *testing.T) {
	keyPool := pool.New([]string{"k1"})
	key, err := keyPool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	key.FinishRequest()

	tests := []struct {
		name        string
		cap         time.Duration
		priorErrors int64
		want        time.Duration
	}{
		{name: "fresh key uses short cooling", cap: 15 * time.Second, priorErrors: 0, want: 2 * time.Second},
		{name: "second failure backs off", cap: 15 * time.Second, priorErrors: 1, want: 4 * time.Second},
		{name: "third failure backs off again", cap: 15 * time.Second, priorErrors: 2, want: 8 * time.Second},
		{name: "repeated failures cap at configured transient cooling", cap: 15 * time.Second, priorErrors: 3, want: 15 * time.Second},
		{name: "configured shorter cap is respected", cap: time.Second, priorErrors: 0, want: time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key.ResetConnectionFailures()
			for range tc.priorErrors {
				key.MarkConnectionCooling(time.Nanosecond)
			}

			if got := connectionCoolingDuration(tc.cap, key.ConnectionFailureCount()); got != tc.want {
				t.Fatalf("connectionCoolingDuration() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestConnectionCoolingDurationIgnoresNonConnectionErrors(t *testing.T) {
	keyPool := pool.New([]string{"k1"})
	key, err := keyPool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	key.FinishRequest()
	key.MarkCooling(time.Nanosecond)
	key.MarkCooling(time.Nanosecond)
	key.MarkCooling(time.Nanosecond)

	if got := connectionCoolingDuration(15*time.Second, key.ConnectionFailureCount()); got != 2*time.Second {
		t.Fatalf("connectionCoolingDuration() = %v, want fresh connection cooling despite non-connection errors", got)
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
			_, _ = w.Write([]byte(`{"error":{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}}`))
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
			name: "structured insufficient balance code",
			body: `{"error":{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}}`,
			want: true,
		},
		{
			name: "insufficient account balance message",
			body: `{"error":{"message":"Insufficient account balance"}}`,
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

func TestDrainAndCloseReadsSmallBodyAndCloses(t *testing.T) {
	body := newTrackingReadCloser("abcdef")

	if err := drainAndClose(body, 64); err != nil {
		t.Fatalf("drainAndClose() error = %v", err)
	}
	if !body.closed.Load() {
		t.Fatal("body was not closed")
	}
	if got, want := body.bytesRead.Load(), int64(len(body.data)); got != want {
		t.Fatalf("bytes read = %d, want %d", got, want)
	}
}

func TestDrainAndCloseLimitsLargeBodyAndCloses(t *testing.T) {
	body := newTrackingReadCloser(strings.Repeat("x", 128))

	if err := drainAndClose(body, 64); err != nil {
		t.Fatalf("drainAndClose() error = %v", err)
	}
	if !body.closed.Load() {
		t.Fatal("body was not closed")
	}
	if got, want := body.bytesRead.Load(), int64(64); got != want {
		t.Fatalf("bytes read = %d, want %d", got, want)
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

func TestUpdateConfigUsesFreshProviderCircuit(t *testing.T) {
	var firstCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
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
		TargetURL:               first.URL,
		Keys:                    []string{"k1"},
		RequestTimeoutSeconds:   10,
		MaxRetries:              5,
		MaxTransientRetries:     2,
		CoolingSeconds:          1,
		TransientCoolingSeconds: 1,
	}
	h, pools := mustHandler(t, cfg)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
	oldCircuit := h.snapshot().circuit
	if got := oldCircuit.snapshot().state; got != providerCircuitStateOpen.String() {
		t.Fatalf("old circuit state = %q, want open", got)
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

	newCircuit := h.snapshot().circuit
	if newCircuit == oldCircuit {
		t.Fatal("new runtime reused old provider circuit")
	}
	if got := newCircuit.snapshot().state; got != providerCircuitStateClosed.String() {
		t.Fatalf("new circuit state = %q, want closed", got)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://proxy.test/v1/messages", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if firstCalls.Load() != 3 {
		t.Fatalf("first calls = %d, want 3", firstCalls.Load())
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

	err := streamBody(writer, strings.NewReader("chunk"), nil)

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestStreamBodyClassifiesCanceledClientWrite(t *testing.T) {
	err := streamBody(failingResponseWriter{err: context.Canceled}, strings.NewReader("chunk"), nil)

	side, streamErr := streamFailureDetails(err)
	if side != streamFailureSideClientCanceled {
		t.Fatalf("stream failure side = %q, want %q", side, streamFailureSideClientCanceled)
	}
	if !errors.Is(streamErr, context.Canceled) {
		t.Fatalf("stream error = %v, want context.Canceled", streamErr)
	}
}

func TestStreamBodyClassifiesCanceledUpstreamReadAsClientCanceled(t *testing.T) {
	err := streamBody(httptest.NewRecorder(), &failingReadCloser{err: context.Canceled}, nil)

	side, streamErr := streamFailureDetails(err)
	if side != streamFailureSideClientCanceled {
		t.Fatalf("stream failure side = %q, want %q", side, streamFailureSideClientCanceled)
	}
	if !errors.Is(streamErr, context.Canceled) {
		t.Fatalf("stream error = %v, want context.Canceled", streamErr)
	}
}

func TestStreamBodyWritesSSEKeepAliveWhileWaitingForUpstream(t *testing.T) {
	reader := newBlockingReadCloser()
	defer reader.Close()
	writer := &flushRecorder{}

	errCh := make(chan error, 1)
	go func() {
		errCh <- streamBody(writer, reader, func() { _ = reader.Close() }, streamOptions{
			keepAlive:        10 * time.Millisecond,
			idleTimeout:      time.Second,
			contentType:      "text/event-stream",
			keepAliveComment: ": modelmux keepalive\n\n",
		})
	}()

	waitForCondition(t, time.Second, func() bool {
		return strings.Contains(writer.String(), ": modelmux keepalive\n\n")
	})
	reader.Send("data: done\n\n")
	reader.Close()

	if err := <-errCh; err != nil {
		t.Fatalf("streamBody() error = %v", err)
	}
	if !strings.Contains(writer.String(), "data: done\n\n") {
		t.Fatalf("body = %q, want upstream chunk", writer.String())
	}
	if writer.Flushes() == 0 {
		t.Fatal("flushes = 0, want keepalive/upstream flushes")
	}
}

func TestStreamBodyDoesNotInjectKeepAliveForNonSSE(t *testing.T) {
	reader := newBlockingReadCloser()
	defer reader.Close()
	writer := &flushRecorder{}

	errCh := make(chan error, 1)
	go func() {
		errCh <- streamBody(writer, reader, func() { _ = reader.Close() }, streamOptions{
			keepAlive:   10 * time.Millisecond,
			idleTimeout: time.Second,
			contentType: "application/json",
		})
	}()

	time.Sleep(30 * time.Millisecond)
	reader.Send(`{"ok":true}`)
	reader.Close()

	if err := <-errCh; err != nil {
		t.Fatalf("streamBody() error = %v", err)
	}
	if strings.Contains(writer.String(), "keepalive") {
		t.Fatalf("body = %q, want no injected keepalive for non-SSE", writer.String())
	}
	if !strings.Contains(writer.String(), `{"ok":true}`) {
		t.Fatalf("body = %q, want upstream JSON", writer.String())
	}
}

func TestStreamBodyFailsWhenUpstreamIdleTimeoutExpires(t *testing.T) {
	reader := newBlockingReadCloser()
	defer reader.Close()

	err := streamBody(httptest.NewRecorder(), reader, func() { _ = reader.Close() }, streamOptions{
		idleTimeout: 10 * time.Millisecond,
		contentType: "text/event-stream",
	})

	side, streamErr := streamFailureDetails(err)
	if side != streamFailureSideUpstreamRead {
		t.Fatalf("stream failure side = %q, want %q", side, streamFailureSideUpstreamRead)
	}
	if !errors.Is(streamErr, errStreamIdleTimeout) {
		t.Fatalf("stream error = %v, want errStreamIdleTimeout", streamErr)
	}
}

func TestStreamBodyFailsWhenMaxDurationExpires(t *testing.T) {
	reader := newBlockingReadCloser()
	defer reader.Close()
	writer := &flushRecorder{}

	errCh := make(chan error, 1)
	go func() {
		errCh <- streamBody(writer, reader, func() { _ = reader.Close() }, streamOptions{
			keepAlive:   5 * time.Millisecond,
			maxDuration: 20 * time.Millisecond,
			contentType: "text/event-stream",
		})
	}()

	err := <-errCh
	side, streamErr := streamFailureDetails(err)
	if side != streamFailureSideUpstreamRead {
		t.Fatalf("stream failure side = %q, want %q", side, streamFailureSideUpstreamRead)
	}
	if !errors.Is(streamErr, errStreamMaxDurationExceeded) {
		t.Fatalf("stream error = %v, want errStreamMaxDurationExceeded", streamErr)
	}
}

func TestStreamFailureLogLevelUsesInfoForClientCancellation(t *testing.T) {
	if got := streamFailureLogLevel(streamFailureSideClientCanceled); got != slog.LevelInfo {
		t.Fatalf("client canceled log level = %v, want info", got)
	}
	if got := streamFailureLogLevel(streamFailureSideUpstreamRead); got != slog.LevelWarn {
		t.Fatalf("upstream read log level = %v, want warn", got)
	}
	if got := streamFailureLogLevel(streamFailureSideClientWrite); got != slog.LevelWarn {
		t.Fatalf("client write log level = %v, want warn", got)
	}
}

func hasEvent(events []logx.Event, name string) bool {
	for _, event := range events {
		if event.Event == name {
			return true
		}
	}
	return false
}

type scriptedResponse struct {
	status  int
	body    io.ReadCloser
	headers http.Header
}

type scriptedTransport struct {
	responses []scriptedResponse
	calls     atomic.Int32
}

func (t *scriptedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}

	idx := int(t.calls.Add(1)) - 1
	if idx >= len(t.responses) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    req,
		}, nil
	}

	next := t.responses[idx]
	body := next.body
	if body == nil {
		body = http.NoBody
	}
	headers := make(http.Header)
	for name, values := range next.headers {
		headers[name] = append([]string(nil), values...)
	}
	return &http.Response{
		StatusCode: next.status,
		Status:     fmt.Sprintf("%d %s", next.status, http.StatusText(next.status)),
		Header:     headers,
		Body:       body,
		Request:    req,
	}, nil
}

type trackingReadCloser struct {
	data      []byte
	offset    int
	bytesRead atomic.Int64
	closed    atomic.Bool
}

func newTrackingReadCloser(value string) *trackingReadCloser {
	return &trackingReadCloser{data: []byte(value)}
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	r.bytesRead.Add(int64(n))
	return n, nil
}

func (r *trackingReadCloser) Close() error {
	r.closed.Store(true)
	return nil
}

type failingReadCloser struct {
	chunks [][]byte
	err    error
	closed atomic.Bool
}

func (r *failingReadCloser) Read(p []byte) (int, error) {
	if len(r.chunks) == 0 {
		return 0, r.err
	}
	chunk := r.chunks[0]
	n := copy(p, chunk)
	if n == len(chunk) {
		r.chunks = r.chunks[1:]
	} else {
		r.chunks[0] = chunk[n:]
	}
	return n, nil
}

func (r *failingReadCloser) Close() error {
	r.closed.Store(true)
	return nil
}

type blockingReadCloser struct {
	ch     chan []byte
	closed atomic.Bool
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{ch: make(chan []byte, 8)}
}

func (r *blockingReadCloser) Read(p []byte) (int, error) {
	chunk, ok := <-r.ch
	if !ok {
		return 0, io.EOF
	}
	n := copy(p, chunk)
	if n < len(chunk) {
		remaining := append([]byte(nil), chunk[n:]...)
		go func() {
			r.ch <- remaining
		}()
	}
	return n, nil
}

func (r *blockingReadCloser) Send(value string) {
	r.ch <- []byte(value)
}

func (r *blockingReadCloser) Close() error {
	if r.closed.CompareAndSwap(false, true) {
		close(r.ch)
	}
	return nil
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

type flushRecorder struct {
	header  http.Header
	body    strings.Builder
	mu      sync.Mutex
	flushes atomic.Int32
}

func (w *flushRecorder) Header() http.Header {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *flushRecorder) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.Write(p)
}

func (w *flushRecorder) WriteHeader(int) {}

func (w *flushRecorder) Flush() {
	w.flushes.Add(1)
}

func (w *flushRecorder) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

func (w *flushRecorder) Flushes() int {
	return int(w.flushes.Load())
}

func waitForCondition(t testing.TB, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}
