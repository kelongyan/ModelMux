package stats

import (
	"sort"
	"time"
)

const UnknownModel = "unknown"

type Summary struct {
	TotalCalls       int     `json:"total_calls"`
	SuccessCalls     int     `json:"success_calls"`
	FailedCalls      int     `json:"failed_calls"`
	UsageKnownCalls  int     `json:"usage_known_calls"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

type ModelSummary struct {
	Model            string  `json:"model"`
	Calls            int     `json:"calls"`
	SuccessCalls     int     `json:"success_calls"`
	FailedCalls      int     `json:"failed_calls"`
	UsageKnownCalls  int     `json:"usage_known_calls"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// SummarySince 汇总 since 之后的内存调用记录。
func (s *Store) SummarySince(since time.Time) Summary {
	if s == nil {
		return Summary{}
	}
	records, err := s.recordsSince(since)
	if err != nil {
		return Summary{}
	}

	var summary Summary
	var latencyTotal int64
	for _, record := range records {
		summary.TotalCalls++
		if record.Success {
			summary.SuccessCalls++
		} else {
			summary.FailedCalls++
		}
		if record.PromptTokens != nil || record.CompletionTokens != nil || record.TotalTokens != nil {
			summary.UsageKnownCalls++
		}
		if record.PromptTokens != nil {
			summary.PromptTokens += *record.PromptTokens
		}
		if record.CompletionTokens != nil {
			summary.CompletionTokens += *record.CompletionTokens
		}
		if record.TotalTokens != nil {
			summary.TotalTokens += *record.TotalTokens
		}
		latencyTotal += record.LatencyMs
	}
	if summary.TotalCalls > 0 {
		summary.AvgLatencyMs = float64(latencyTotal) / float64(summary.TotalCalls)
	}
	return summary
}

// ModelsSince 按 model 汇总 since 之后的内存调用记录，调用量高的模型排前面。
func (s *Store) ModelsSince(since time.Time) []ModelSummary {
	if s == nil {
		return nil
	}
	records, err := s.recordsSince(since)
	if err != nil {
		return nil
	}

	type aggregate struct {
		summary      ModelSummary
		latencyTotal int64
	}
	byModel := make(map[string]*aggregate)
	for _, record := range records {
		model := record.Model
		if model == "" {
			model = UnknownModel
		}
		item := byModel[model]
		if item == nil {
			item = &aggregate{summary: ModelSummary{Model: model}}
			byModel[model] = item
		}
		item.summary.Calls++
		if record.Success {
			item.summary.SuccessCalls++
		} else {
			item.summary.FailedCalls++
		}
		if record.PromptTokens != nil || record.CompletionTokens != nil || record.TotalTokens != nil {
			item.summary.UsageKnownCalls++
		}
		if record.PromptTokens != nil {
			item.summary.PromptTokens += *record.PromptTokens
		}
		if record.CompletionTokens != nil {
			item.summary.CompletionTokens += *record.CompletionTokens
		}
		if record.TotalTokens != nil {
			item.summary.TotalTokens += *record.TotalTokens
		}
		item.latencyTotal += record.LatencyMs
	}

	out := make([]ModelSummary, 0, len(byModel))
	for _, item := range byModel {
		if item.summary.Calls > 0 {
			item.summary.AvgLatencyMs = float64(item.latencyTotal) / float64(item.summary.Calls)
		}
		out = append(out, item.summary)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].Model < out[j].Model
	})
	return out
}

func (s *Store) recordsSince(since time.Time) ([]CallRecord, error) {
	return s.recordsSinceFromFiles(since)
}
