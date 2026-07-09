package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kelongyan/ModelMux/config"
)

// TestBuildRequestSkipsStreamUsageForNonOpenAIProtocol 验证非 openai 协议的 provider
// 不会被注入 OpenAI 专有的 stream_options.include_usage 字段。
func TestBuildRequestSkipsStreamUsageForNonOpenAIProtocol(t *testing.T) {
	cfg := &config.Config{
		ActiveProvider: "anthropic",
		Providers: []config.ProviderConfig{
			{
				ID:        "anthropic",
				TargetURL: "https://example.com/v1",
				Keys:      []string{"rotated-key"},
				Protocol:  config.ProtocolAnthropic,
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

	body := []byte(`{"model":"claude-sonnet-4-6","stream":true,"messages":[]}`)
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/messages", strings.NewReader(string(body)))
	outReq, err := buildRequest(h.snapshot(), req, key, body, requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	outBody, err := io.ReadAll(outReq.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(outBody) != string(body) {
		t.Fatalf("body changed for anthropic protocol:\ngot  %s\nwant %s", string(outBody), string(body))
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(outBody, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := payload["stream_options"]; ok {
		t.Fatalf("stream_options was injected for anthropic protocol: %s", string(outBody))
	}
}

// TestBuildRequestInjectsStreamUsageForOpenAIProtocol 验证显式声明 openai 协议时
// 仍会注入 include_usage，确保默认行为不变。
func TestBuildRequestInjectsStreamUsageForOpenAIProtocol(t *testing.T) {
	cfg := &config.Config{
		ActiveProvider: "oai",
		Providers: []config.ProviderConfig{
			{
				ID:        "oai",
				TargetURL: "https://example.com/v1",
				Keys:      []string{"rotated-key"},
				Protocol:  config.ProtocolOpenAI,
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
		StreamOptions struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
	}
	if err := json.Unmarshal(outBody, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !payload.StreamOptions.IncludeUsage {
		t.Fatalf("stream_options.include_usage = false, want true; body=%s", string(outBody))
	}
}

// TestBuildRequestSkipsStripToolsWhenMessagesReferenceToolCalls 验证当历史消息中
// 已包含 assistant 的 tool_calls 时，即使开启 strip_tools 也保留顶层 tools 定义，
// 避免破坏进行中的工具调用对话导致上游 400。
func TestBuildRequestSkipsStripToolsWhenMessagesReferenceToolCalls(t *testing.T) {
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

	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":"sunny"}],"tools":[{"type":"function"}],"tool_choice":"auto"}`)
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
	outReq, err := buildRequest(h.snapshot(), req, key, body, requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	outBody, err := io.ReadAll(outReq.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(outBody, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := payload["tools"]; !ok {
		t.Fatalf("tools was stripped despite tool_calls in messages: %s", string(outBody))
	}
	if _, ok := payload["tool_choice"]; !ok {
		t.Fatalf("tool_choice was stripped despite tool_calls in messages: %s", string(outBody))
	}
}

// TestBuildRequestStripsToolsForPlainConversation 验证纯对话（无 tool_calls）
// 场景下 strip_tools 仍按预期剥离工具定义。
func TestBuildRequestStripsToolsForPlainConversation(t *testing.T) {
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

	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"ping"}],"tools":[{"type":"function"}],"tool_choice":"auto"}`)
	req := httptest.NewRequest(http.MethodPost, "http://proxy.test/v1/chat/completions", strings.NewReader(string(body)))
	outReq, err := buildRequest(h.snapshot(), req, key, body, requestMeta{})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	outBody, err := io.ReadAll(outReq.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(outBody, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := payload["tools"]; ok {
		t.Fatalf("tools was not stripped for plain conversation: %s", string(outBody))
	}
	if _, ok := payload["tool_choice"]; ok {
		t.Fatalf("tool_choice was not stripped for plain conversation: %s", string(outBody))
	}
}
