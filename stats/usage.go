package stats

import (
	"bytes"
	"encoding/json"
	"strings"
)

type Usage struct {
	PromptTokens     *int64
	CompletionTokens *int64
	TotalTokens      *int64
	Source           string
}

// ExtractUsage 从 OpenAI 兼容响应体中提取上游返回的 token usage。
func ExtractUsage(body []byte) Usage {
	unknown := Usage{Source: UsageSourceUnknown}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return unknown
	}

	if usage := extractUsageFromJSON(body); usage.Source == UsageSourceUpstream {
		return usage
	}
	return extractUsageFromSSE(body)
}

func extractUsageFromJSON(body []byte) Usage {
	unknown := Usage{Source: UsageSourceUnknown}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return unknown
	}
	rawUsage, ok := root["usage"]
	if !ok {
		return unknown
	}

	var usage map[string]json.RawMessage
	if err := json.Unmarshal(rawUsage, &usage); err != nil {
		return unknown
	}

	prompt := firstTokenValue(usage, "prompt_tokens", "input_tokens", "input_token_count")
	completion := firstTokenValue(usage, "completion_tokens", "output_tokens", "output_token_count")
	total := firstTokenValue(usage, "total_tokens", "total_token_count")
	if total == nil && prompt != nil && completion != nil {
		sum := *prompt + *completion
		total = &sum
	}
	if prompt == nil && completion == nil && total == nil {
		return unknown
	}

	return Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		Source:           UsageSourceUpstream,
	}
}

func extractUsageFromSSE(body []byte) Usage {
	var found Usage
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		usage := extractUsageFromJSON([]byte(payload))
		if usage.Source == UsageSourceUpstream {
			found = usage
		}
	}
	if found.Source == UsageSourceUpstream {
		return found
	}
	return Usage{Source: UsageSourceUnknown}
}

func firstTokenValue(values map[string]json.RawMessage, names ...string) *int64 {
	for _, name := range names {
		raw, ok := values[name]
		if !ok {
			continue
		}
		var value int64
		if err := json.Unmarshal(raw, &value); err == nil {
			return &value
		}
	}
	return nil
}
