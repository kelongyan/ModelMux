package stats

import (
	"sort"
	"time"
)

const UnknownModel = "unknown"

type Summary struct {
	TotalCalls       int            `json:"total_calls"`
	SuccessCalls     int            `json:"success_calls"`
	FailedCalls      int            `json:"failed_calls"`
	UsageKnownCalls  int            `json:"usage_known_calls"`
	PromptTokens     int64          `json:"prompt_tokens"`
	CompletionTokens int64          `json:"completion_tokens"`
	TotalTokens      int64          `json:"total_tokens"`
	AvgLatencyMs     float64        `json:"avg_latency_ms"`
	P50LatencyMs     int64          `json:"p50_latency_ms"`
	P95LatencyMs     int64          `json:"p95_latency_ms"`
	P99LatencyMs     int64          `json:"p99_latency_ms"`
	ErrorByStatus    map[int]int64  `json:"error_by_status,omitempty"`
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
	latencies := make([]int64, 0, len(records))
	for _, record := range records {
		summary.TotalCalls++
		if record.Success {
			summary.SuccessCalls++
		} else {
			summary.FailedCalls++
			// 按 HTTP 状态码分类错误
			if record.Status > 0 {
				if summary.ErrorByStatus == nil {
					summary.ErrorByStatus = make(map[int]int64)
				}
				summary.ErrorByStatus[record.Status]++
			}
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
		latencies = append(latencies, record.LatencyMs)
	}
	if summary.TotalCalls > 0 {
		summary.AvgLatencyMs = float64(latencyTotal) / float64(summary.TotalCalls)
		summary.P50LatencyMs = percentile(latencies, 50)
		summary.P95LatencyMs = percentile(latencies, 95)
		summary.P99LatencyMs = percentile(latencies, 99)
	}
	return summary
}

// percentile 计算给定百分位的延迟值（P50/P95/P99）。
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	// 复制后排序，避免影响原切片
	sortedCopy := make([]int64, len(sorted))
	copy(sortedCopy, sorted)
	sort.Slice(sortedCopy, func(i, j int) bool { return sortedCopy[i] < sortedCopy[j] })

	idx := (p * len(sortedCopy)) / 100
	if idx >= len(sortedCopy) {
		idx = len(sortedCopy) - 1
	}
	return sortedCopy[idx]
}

// TimelinePoint 表示时间线上的一个数据点。
type TimelinePoint struct {
	Time           time.Time `json:"time"`
	TotalCalls     int       `json:"total_calls"`
	SuccessCalls   int       `json:"success_calls"`
	FailedCalls    int       `json:"failed_calls"`
	AvgLatencyMs   float64   `json:"avg_latency_ms"`
	TotalTokens    int64     `json:"total_tokens"`
}

// TimelineGranularity 时间线粒度枚举。
type TimelineGranularity string

const (
	TimelineGranularityHour TimelineGranularity = "1h"
	TimelineGranularityDay  TimelineGranularity = "1d"
)

// TimelineSince 按时间粒度聚合 since 之后的调用记录，返回时间线数据点。
func (s *Store) TimelineSince(since time.Time, granularity TimelineGranularity) []TimelinePoint {
	if s == nil {
		return nil
	}
	records, err := s.recordsSince(since)
	if err != nil {
		return nil
	}

	// 确定时间桶的大小
	var bucketDuration time.Duration
	switch granularity {
	case TimelineGranularityDay:
		bucketDuration = 24 * time.Hour
	default:
		bucketDuration = time.Hour
	}

	// 按时间桶聚合
	type bucketAggregate struct {
		totalCalls   int
		successCalls int
		failedCalls  int
		latencyTotal int64
		totalTokens  int64
	}
	buckets := make(map[int64]*bucketAggregate)

	for _, record := range records {
		// 计算桶的起始时间戳
		bucketKey := record.At.Truncate(bucketDuration).UnixNano()
		bucket := buckets[bucketKey]
		if bucket == nil {
			bucket = &bucketAggregate{}
			buckets[bucketKey] = bucket
		}
		bucket.totalCalls++
		if record.Success {
			bucket.successCalls++
		} else {
			bucket.failedCalls++
		}
		bucket.latencyTotal += record.LatencyMs
		if record.TotalTokens != nil {
			bucket.totalTokens += *record.TotalTokens
		}
	}

	// 转换为有序的时间线数据点
	out := make([]TimelinePoint, 0, len(buckets))
	for ts, bucket := range buckets {
		point := TimelinePoint{
			Time:         time.Unix(0, ts).UTC(),
			TotalCalls:   bucket.totalCalls,
			SuccessCalls: bucket.successCalls,
			FailedCalls:  bucket.failedCalls,
			TotalTokens:  bucket.totalTokens,
		}
		if bucket.totalCalls > 0 {
			point.AvgLatencyMs = float64(bucket.latencyTotal) / float64(bucket.totalCalls)
		}
		out = append(out, point)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}

// ProviderSummary 表示单个 provider 的统计汇总。
type ProviderSummary struct {
	ProviderID       string  `json:"provider_id"`
	Calls            int     `json:"calls"`
	SuccessCalls     int     `json:"success_calls"`
	FailedCalls      int     `json:"failed_calls"`
	UsageKnownCalls  int     `json:"usage_known_calls"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// SummaryByProvider 按 provider 汇总 since 之后的内存调用记录，调用量高的排前面。
func (s *Store) SummaryByProvider(since time.Time) []ProviderSummary {
	if s == nil {
		return nil
	}
	records, err := s.recordsSince(since)
	if err != nil {
		return nil
	}

	type aggregate struct {
		summary      ProviderSummary
		latencyTotal int64
	}
	byProvider := make(map[string]*aggregate)
	for _, record := range records {
		providerID := record.ProviderID
		if providerID == "" {
			providerID = "unknown"
		}
		item := byProvider[providerID]
		if item == nil {
			item = &aggregate{summary: ProviderSummary{ProviderID: providerID}}
			byProvider[providerID] = item
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

	out := make([]ProviderSummary, 0, len(byProvider))
	for _, item := range byProvider {
		if item.summary.Calls > 0 {
			item.summary.AvgLatencyMs = float64(item.latencyTotal) / float64(item.summary.Calls)
		}
		out = append(out, item.summary)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].ProviderID < out[j].ProviderID
	})
	return out
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
	if s == nil {
		return nil, nil
	}

	requestSince := since.UTC()

	// 快速路径：如果内存中的记录覆盖了查询窗口，直接从内存过滤，避免 Flush + 全量文件扫描。
	if s.recordsCoverSince(requestSince) {
		return s.filterMemoryRecordsSince(requestSince), nil
	}

	// 慢速路径：查询窗口超出内存范围，回退到文件扫描（含 Flush）。
	scanSince := requestSince.Truncate(defaultQueryCacheTTL)
	now := s.now().UTC()
	key := recordsCacheKey{sinceUnixNano: scanSince.UnixNano()}

	s.queryCacheMu.Lock()
	if entry, ok := s.recordsCache[key]; ok && now.Before(entry.expiresAt) {
		records := filterRecordsSince(entry.records, requestSince)
		s.queryCacheMu.Unlock()
		return records, nil
	}
	s.queryCacheMu.Unlock()

	records, err := s.recordsSinceFromFiles(scanSince)
	if err != nil {
		return nil, err
	}

	now = s.now().UTC()
	s.queryCacheMu.Lock()
	if s.recordsCache == nil {
		s.recordsCache = make(map[recordsCacheKey]recordsCacheEntry)
	}
	s.pruneRecordsCacheLocked(now)
	s.recordsCache[key] = recordsCacheEntry{
		expiresAt: now.Add(defaultQueryCacheTTL),
		records:   append([]CallRecord(nil), records...),
	}
	s.queryCacheMu.Unlock()

	return filterRecordsSince(records, requestSince), nil
}

// recordsCoverSince 判断内存中的记录是否完全覆盖查询窗口。
// 如果内存中最早的记录时间不晚于 since，则覆盖。
func (s *Store) recordsCoverSince(since time.Time) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.records) == 0 {
		return false
	}
	oldest := s.records[0].At
	return !oldest.After(since)
}

// filterMemoryRecordsSince 从内存记录中过滤 since 之后的记录。
func (s *Store) filterMemoryRecordsSince(since time.Time) []CallRecord {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return filterRecordsSince(s.records, since)
}

func filterRecordsSince(records []CallRecord, since time.Time) []CallRecord {
	filtered := make([]CallRecord, 0, len(records))
	for _, record := range records {
		if record.At.IsZero() || record.At.Before(since) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

func (s *Store) pruneRecordsCacheLocked(now time.Time) {
	for key, entry := range s.recordsCache {
		if !now.Before(entry.expiresAt) {
			delete(s.recordsCache, key)
		}
	}
}
