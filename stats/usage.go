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
	Model            string // upstream-returned model ID (may differ from the requested model)
}

func deriveTotalTokens(prompt, completion, total *int64) *int64 {
	if total != nil {
		return total
	}
	if prompt == nil || completion == nil {
		return nil
	}
	sum := *prompt + *completion
	return &sum
}

func normalizeUsageFields(record *CallRecord) {
	if record == nil {
		return
	}
	record.TotalTokens = deriveTotalTokens(record.PromptTokens, record.CompletionTokens, record.TotalTokens)
	if record.UsageSource == "" {
		record.UsageSource = UsageSourceUnknown
	}
}

// ExtractUsage 从 OpenAI 兼容响应体中提取上游返回的 token usage。
// 支持非流式 JSON 与 SSE 流式响应，覆盖多种 provider 的字段命名约定：
//   - OpenAI Chat Completions: 顶层 usage.{prompt_tokens, completion_tokens, total_tokens}
//   - OpenAI Responses API:    SSE 事件 response.completed 中的 response.usage.{input_tokens, output_tokens, total_tokens}
//   - Anthropic Messages:      顶层或 message_delta 事件中的 usage.{input_tokens, output_tokens}
//   - Google Gemini:           顶层 usageMetadata.{promptTokenCount, candidatesTokenCount, totalTokenCount}
func ExtractUsage(body []byte) Usage {
	unknown := Usage{Source: UsageSourceUnknown}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return unknown
	}

	if usage := extractUsageFromJSON(body); usage.Source == UsageSourceUpstream {
		if usage.Model == "" {
			usage.Model = extractTopLevelModel(body)
		}
		return usage
	}
	usage := extractUsageFromSSE(body)
	if usage.Source == UsageSourceUpstream && usage.Model == "" {
		usage.Model = extractModelFromSSE(body)
	}
	return usage
}

// extractTopLevelModel 从 JSON 响应体中提取 model 字段。
// 兼容顶层 model (Chat Completions / Anthropic) 和 response.model (Responses API) 两种路径。
func extractTopLevelModel(body []byte) string {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return ""
	}
	// 顶层 model
	if raw, ok := root["model"]; ok {
		var model string
		if err := json.Unmarshal(raw, &model); err == nil {
			model = strings.TrimSpace(model)
			if model != "" {
				return model
			}
		}
	}
	// response.model (Responses API)
	if rawResponse, ok := root["response"]; ok {
		var responseObj map[string]json.RawMessage
		if err := json.Unmarshal(rawResponse, &responseObj); err == nil {
			if raw, ok := responseObj["model"]; ok {
				var model string
				if err := json.Unmarshal(raw, &model); err == nil {
					return strings.TrimSpace(model)
				}
			}
		}
	}
	return ""
}

// extractModelFromSSE 从 SSE 流中提取第一个包含 model 字段的 data 事件里的 model 值。
// 兼容顶层 model (Chat Completions / Anthropic) 和 response.model (Responses API) 两种路径。
func extractModelFromSSE(body []byte) string {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &root); err != nil {
			continue
		}
		// 顶层 model (Chat Completions / Anthropic message_start)
		if raw, ok := root["model"]; ok {
			var model string
			if err := json.Unmarshal(raw, &model); err == nil {
				model = strings.TrimSpace(model)
				if model != "" {
					return model
				}
			}
		}
		// response.model (Responses API response.completed)
		if rawResponse, ok := root["response"]; ok {
			var responseObj map[string]json.RawMessage
			if err := json.Unmarshal(rawResponse, &responseObj); err == nil {
				if raw, ok := responseObj["model"]; ok {
					var model string
					if err := json.Unmarshal(raw, &model); err == nil {
						model = strings.TrimSpace(model)
						if model != "" {
							return model
						}
					}
				}
			}
		}
	}
	return ""
}

// extractUsageFromJSON 从单个 JSON 对象中按多个常见路径查找 usage。
// 依次尝试: 顶层 usage -> response.usage (Responses API) -> usageMetadata (Gemini)。
func extractUsageFromJSON(body []byte) Usage {
	unknown := Usage{Source: UsageSourceUnknown}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return unknown
	}

	if usage := readUsageObject(root["usage"]); usage.Source == UsageSourceUpstream {
		return usage
	}

	if rawResponse, ok := root["response"]; ok {
		var responseObj map[string]json.RawMessage
		if err := json.Unmarshal(rawResponse, &responseObj); err == nil {
			if usage := readUsageObject(responseObj["usage"]); usage.Source == UsageSourceUpstream {
				return usage
			}
		}
	}

	if usage := readGeminiUsageMetadata(root["usageMetadata"]); usage.Source == UsageSourceUpstream {
		return usage
	}

	return unknown
}

// readUsageObject 从 usage 对象中读取 token 计数。
// 兼容 OpenAI (prompt_tokens/completion_tokens/total_tokens) 与
// Anthropic/Responses API (input_tokens/output_tokens) 两套命名。
func readUsageObject(raw json.RawMessage) Usage {
	unknown := Usage{Source: UsageSourceUnknown}
	if len(raw) == 0 {
		return unknown
	}
	var usage map[string]json.RawMessage
	if err := json.Unmarshal(raw, &usage); err != nil {
		return unknown
	}
	prompt := firstTokenValue(usage, "prompt_tokens", "input_tokens", "input_token_count")
	completion := firstTokenValue(usage, "completion_tokens", "output_tokens", "output_token_count")
	total := deriveTotalTokens(prompt, completion, firstTokenValue(usage, "total_tokens", "total_token_count"))
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

// readGeminiUsageMetadata 从 Google Gemini 的 usageMetadata 对象读取 token 计数。
func readGeminiUsageMetadata(raw json.RawMessage) Usage {
	unknown := Usage{Source: UsageSourceUnknown}
	if len(raw) == 0 {
		return unknown
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(raw, &meta); err != nil {
		return unknown
	}
	prompt := firstTokenValue(meta, "promptTokenCount", "inputTokenCount", "prompt_tokens", "input_tokens")
	completion := firstTokenValue(meta, "candidatesTokenCount", "outputTokenCount", "completion_tokens", "output_tokens")
	total := deriveTotalTokens(prompt, completion, firstTokenValue(meta, "totalTokenCount", "total_tokens"))
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

// extractUsageFromSSE 遍历 SSE 事件，按时间顺序寻找最后一条包含 usage 的 payload。
// 覆盖三种常见形态：
//   - Chat Completions + include_usage: 独立 usage chunk (顶层 usage)
//   - Responses API: response.completed 事件 (response.usage)
//   - Anthropic Messages: message_delta 事件 (顶层 usage)
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
