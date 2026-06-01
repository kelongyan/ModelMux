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

func assertInt64Ptr(t *testing.T, got *int64, want int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("token value = nil, want %d", want)
	}
	if *got != want {
		t.Fatalf("token value = %d, want %d", *got, want)
	}
}
