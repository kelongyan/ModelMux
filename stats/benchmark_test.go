package stats

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func BenchmarkStoreAppend(b *testing.B) {
	store, base := newBenchmarkStore(b, b.N+10)
	record := benchmarkCallRecord(base)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		record.At = base.Add(time.Duration(i) * time.Millisecond)
		if err := store.Append(record); err != nil {
			b.Fatalf("Append() error = %v", err)
		}
	}
}

func BenchmarkStoreAppendParallel(b *testing.B) {
	store, base := newBenchmarkStore(b, b.N+10)
	var seq int64
	var mu sync.Mutex

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			at := base.Add(time.Duration(seq) * time.Millisecond)
			seq++
			mu.Unlock()

			record := benchmarkCallRecord(at)
			if err := store.Append(record); err != nil {
				b.Fatalf("Append() error = %v", err)
			}
		}
	})
}

func BenchmarkStoreSummarySince(b *testing.B) {
	store, base := newBenchmarkStore(b, 10_000)
	seedBenchmarkRecords(b, store, base, 10_000)
	since := base.Add(-2 * time.Hour)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		summary := store.SummarySince(since)
		if summary.TotalCalls == 0 {
			b.Fatal("SummarySince() returned no calls")
		}
	}
}

func BenchmarkStoreModelsSince(b *testing.B) {
	store, base := newBenchmarkStore(b, 10_000)
	seedBenchmarkRecords(b, store, base, 10_000)
	since := base.Add(-2 * time.Hour)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		models := store.ModelsSince(since)
		if len(models) == 0 {
			b.Fatal("ModelsSince() returned no models")
		}
	}
}

func BenchmarkStoreQueryLogs(b *testing.B) {
	store, base := newBenchmarkStore(b, 10_000)
	seedBenchmarkRecords(b, store, base, 10_000)
	since := base.Add(-2 * time.Hour)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := store.QueryLogs(since, CallLogFilter{
			Model:    "model-1",
			Status:   "success",
			Page:     1,
			PageSize: 20,
		})
		if result.Page != 1 || result.PageSize != 20 {
			b.Fatalf("QueryLogs() page/page_size = %d/%d, want 1/20", result.Page, result.PageSize)
		}
	}
}

func newBenchmarkStore(tb testing.TB, maxRecentRecords int) (*Store, time.Time) {
	tb.Helper()
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{
		Dir:              tb.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: maxRecentRecords,
		Now:              func() time.Time { return base },
	})
	if err != nil {
		tb.Fatalf("NewStore() error = %v", err)
	}
	tb.Cleanup(func() {
		if err := store.Close(); err != nil {
			tb.Fatalf("Close() error = %v", err)
		}
	})
	return store, base
}

func seedBenchmarkRecords(tb testing.TB, store *Store, base time.Time, count int) {
	tb.Helper()
	for i := 0; i < count; i++ {
		record := benchmarkCallRecord(base.Add(time.Duration(-i) * time.Second))
		record.Model = fmt.Sprintf("model-%d", i%10)
		record.Success = i%5 != 0
		if record.Success {
			record.Status = 200
		} else {
			record.Status = 503
		}
		if err := store.Append(record); err != nil {
			tb.Fatalf("Append(seed %d) error = %v", i, err)
		}
	}
}

func benchmarkCallRecord(at time.Time) CallRecord {
	promptTokens := int64(10)
	completionTokens := int64(20)
	totalTokens := int64(30)
	return CallRecord{
		At:               at,
		ProviderID:       "primary",
		Model:            "model-1",
		Endpoint:         "/v1/chat/completions",
		Method:           "POST",
		Status:           200,
		Success:          true,
		Stream:           false,
		LatencyMs:        123,
		Attempts:         1,
		KeyID:            "sha256:benchmark",
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
		TotalTokens:      &totalTokens,
		UsageSource:      UsageSourceUpstream,
	}
}
