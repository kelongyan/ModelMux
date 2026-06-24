package stats

import "testing"

func TestExtractUsageReadsOpenAICompatibleUsage(t *testing.T) {
	usage := ExtractUsage([]byte(`{
		"id": "chatcmpl-test",
		"usage": {
			"prompt_tokens": 12,
			"completion_tokens": 34,
			"total_tokens": 46
		}
	}`))

	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 12)
	assertInt64Ptr(t, usage.CompletionTokens, 34)
	assertInt64Ptr(t, usage.TotalTokens, 46)
}

func TestExtractUsageReadsCommonInputOutputAliases(t *testing.T) {
	usage := ExtractUsage([]byte(`{
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50
		}
	}`))

	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 100)
	assertInt64Ptr(t, usage.CompletionTokens, 50)
	assertInt64Ptr(t, usage.TotalTokens, 150)
}

func TestExtractUsageReadsSSEUsageChunk(t *testing.T) {
	usage := ExtractUsage([]byte("event: message\ndata: {\"id\":\"chatcmpl-test\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"id\":\"chatcmpl-test\",\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":34,\"total_tokens\":46}}\n\ndata: [DONE]\n"))

	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 12)
	assertInt64Ptr(t, usage.CompletionTokens, 34)
	assertInt64Ptr(t, usage.TotalTokens, 46)
}

func TestExtractUsageReturnsUnknownWhenUsageMissing(t *testing.T) {
	usage := ExtractUsage([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))

	if usage.Source != UsageSourceUnknown {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUnknown)
	}
	if usage.PromptTokens != nil || usage.CompletionTokens != nil || usage.TotalTokens != nil {
		t.Fatalf("usage tokens = %+v, want nil token fields", usage)
	}
}

func TestExtractUsageReadsOpenAIResponsesAPICompletedEvent(t *testing.T) {
	body := `event: response.completed
data: {"type":"response.completed","response":{"id":"resp_abc","status":"completed","usage":{"input_tokens":120,"output_tokens":80,"total_tokens":200}}}` + "\n\n" + `data: [DONE]` + "\n"

	usage := ExtractUsage([]byte(body))
	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 120)
	assertInt64Ptr(t, usage.CompletionTokens, 80)
	assertInt64Ptr(t, usage.TotalTokens, 200)
}

func TestExtractUsageReadsOpenAIResponsesAPINonStreaming(t *testing.T) {
	body := `{
		"id": "resp_abc",
		"object": "response",
		"status": "completed",
		"output": [{"type":"message","content":[{"type":"output_text","text":"hi"}]}],
		"usage": {"input_tokens": 30, "output_tokens": 10, "total_tokens": 40}
	}`
	usage := ExtractUsage([]byte(body))
	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 30)
	assertInt64Ptr(t, usage.CompletionTokens, 10)
	assertInt64Ptr(t, usage.TotalTokens, 40)
}

func TestExtractUsageReadsAnthropicMessageDeltaUsage(t *testing.T) {
	body := `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}

event: message_stop
data: {"type":"message_stop"}
`
	usage := ExtractUsage([]byte(body))
	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.CompletionTokens, 15)
}

func TestExtractUsageReadsAnthropicNonStreamingUsage(t *testing.T) {
	body := `{
		"id": "msg_abc",
		"type": "message",
		"role": "assistant",
		"content": [{"type":"text","text":"hello"}],
		"usage": {"input_tokens": 25, "output_tokens": 12}
	}`
	usage := ExtractUsage([]byte(body))
	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 25)
	assertInt64Ptr(t, usage.CompletionTokens, 12)
	assertInt64Ptr(t, usage.TotalTokens, 37)
}

func TestExtractUsageReadsGeminiUsageMetadata(t *testing.T) {
	body := `{
		"candidates": [{"content":{"parts":[{"text":"hi"}]}}],
		"usageMetadata": {
			"promptTokenCount": 14,
			"candidatesTokenCount": 7,
			"totalTokenCount": 21
		}
	}`
	usage := ExtractUsage([]byte(body))
	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 14)
	assertInt64Ptr(t, usage.CompletionTokens, 7)
	assertInt64Ptr(t, usage.TotalTokens, 21)
}

func TestExtractUsagePrefersTopLevelUsageOverResponseUsage(t *testing.T) {
	body := `{
		"usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		"response": {"usage": {"input_tokens": 999, "output_tokens": 999, "total_tokens": 1998}}
	}`
	usage := ExtractUsage([]byte(body))
	if usage.Source != UsageSourceUpstream {
		t.Fatalf("Source = %q, want %q", usage.Source, UsageSourceUpstream)
	}
	assertInt64Ptr(t, usage.PromptTokens, 5)
	assertInt64Ptr(t, usage.CompletionTokens, 3)
	assertInt64Ptr(t, usage.TotalTokens, 8)
}

func TestExtractUsageReturnsUnknownForEmptyOrInvalid(t *testing.T) {
	for _, tc := range []string{"", "   ", "not-json", "null", "[]"} {
		usage := ExtractUsage([]byte(tc))
		if usage.Source != UsageSourceUnknown {
			t.Fatalf("Source(%q) = %q, want %q", tc, usage.Source, UsageSourceUnknown)
		}
	}
}

func assertInt64Ptr(t *testing.T, got *int64, want int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("token value = nil, want %d", want)
	}
	if *got != want {
		t.Fatalf("token value = %d, want %d", *got, want)
	}
}
