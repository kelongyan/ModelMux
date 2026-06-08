package stats

import (
	"testing"
	"time"
)

func TestStoreSummarySinceAggregatesRecentCalls(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 10,
		Now:              func() time.Time { return base },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	mustAppendRecord(t, store, CallRecord{
		At:               base.Add(-30 * time.Minute),
		Model:            "gpt-4.1-mini",
		Status:           200,
		Success:          true,
		LatencyMs:        100,
		PromptTokens:     int64Ptr(10),
		CompletionTokens: int64Ptr(20),
		TotalTokens:      int64Ptr(30),
		UsageSource:      UsageSourceUpstream,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-20 * time.Minute),
		Model:       "gpt-4.1-mini",
		Status:      503,
		Success:     false,
		LatencyMs:   300,
		UsageSource: UsageSourceUnknown,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-48 * time.Hour),
		Model:       "old-model",
		Status:      200,
		Success:     true,
		LatencyMs:   999,
		TotalTokens: int64Ptr(999),
		UsageSource: UsageSourceUpstream,
	})

	summary := store.SummarySince(base.Add(-1 * time.Hour))
	if summary.TotalCalls != 2 {
		t.Fatalf("TotalCalls = %d, want 2", summary.TotalCalls)
	}
	if summary.SuccessCalls != 1 || summary.FailedCalls != 1 {
		t.Fatalf("success/failed = %d/%d, want 1/1", summary.SuccessCalls, summary.FailedCalls)
	}
	if summary.UsageKnownCalls != 1 {
		t.Fatalf("UsageKnownCalls = %d, want 1", summary.UsageKnownCalls)
	}
	if summary.TotalTokens != 30 || summary.PromptTokens != 10 || summary.CompletionTokens != 20 {
		t.Fatalf("tokens = prompt %d completion %d total %d, want 10/20/30", summary.PromptTokens, summary.CompletionTokens, summary.TotalTokens)
	}
	if summary.AvgLatencyMs != 200 {
		t.Fatalf("AvgLatencyMs = %.1f, want 200", summary.AvgLatencyMs)
	}
}

func TestStoreModelsSinceAggregatesByModel(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 10,
		Now:              func() time.Time { return base },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-10 * time.Minute),
		Model:       "model-b",
		Success:     true,
		LatencyMs:   100,
		TotalTokens: int64Ptr(5),
		UsageSource: UsageSourceUpstream,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-9 * time.Minute),
		Model:       "model-a",
		Success:     true,
		LatencyMs:   100,
		TotalTokens: int64Ptr(10),
		UsageSource: UsageSourceUpstream,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-8 * time.Minute),
		Model:       "model-a",
		Success:     false,
		LatencyMs:   300,
		UsageSource: UsageSourceUnknown,
	})

	models := store.ModelsSince(base.Add(-1 * time.Hour))
	if len(models) != 2 {
		t.Fatalf("len(ModelsSince) = %d, want 2", len(models))
	}
	if models[0].Model != "model-a" {
		t.Fatalf("models[0].Model = %q, want model-a", models[0].Model)
	}
	if models[0].Calls != 2 || models[0].SuccessCalls != 1 || models[0].FailedCalls != 1 {
		t.Fatalf("model-a counts = %+v, want calls=2 success=1 failed=1", models[0])
	}
	if models[0].TotalTokens != 10 || models[0].AvgLatencyMs != 200 {
		t.Fatalf("model-a totals = %+v, want tokens=10 avg=200", models[0])
	}
	if models[1].Model != "model-b" || models[1].Calls != 1 {
		t.Fatalf("models[1] = %+v, want model-b calls=1", models[1])
	}
}

func TestStoreSummarySinceReadsBeyondRecentCache(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 2,
		Now:              func() time.Time { return base },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	for i := 0; i < 4; i++ {
		mustAppendRecord(t, store, CallRecord{
			At:          base.Add(time.Duration(-i) * 10 * time.Minute),
			Model:       "model-a",
			Success:     true,
			LatencyMs:   100,
			TotalTokens: int64Ptr(10),
			UsageSource: UsageSourceUpstream,
		})
	}

	if got := len(store.Recent(10)); got != 2 {
		t.Fatalf("len(Recent) = %d, want 2 due to recent cap", got)
	}

	summary := store.SummarySince(base.Add(-1 * time.Hour))
	if summary.TotalCalls != 4 {
		t.Fatalf("TotalCalls = %d, want 4 from files not recent cache", summary.TotalCalls)
	}
	if summary.TotalTokens != 40 {
		t.Fatalf("TotalTokens = %d, want 40", summary.TotalTokens)
	}
}

func TestStoreModelsSinceReadsBeyondRecentCache(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 2,
		Now:              func() time.Time { return base },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-50 * time.Minute),
		Model:       "model-a",
		Success:     true,
		LatencyMs:   100,
		TotalTokens: int64Ptr(10),
		UsageSource: UsageSourceUpstream,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-40 * time.Minute),
		Model:       "model-b",
		Success:     true,
		LatencyMs:   100,
		TotalTokens: int64Ptr(20),
		UsageSource: UsageSourceUpstream,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-30 * time.Minute),
		Model:       "model-a",
		Success:     false,
		LatencyMs:   200,
		UsageSource: UsageSourceUnknown,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-20 * time.Minute),
		Model:       "model-c",
		Success:     true,
		LatencyMs:   150,
		TotalTokens: int64Ptr(5),
		UsageSource: UsageSourceUpstream,
	})

	if got := len(store.Recent(10)); got != 2 {
		t.Fatalf("len(Recent) = %d, want 2 due to recent cap", got)
	}

	models := store.ModelsSince(base.Add(-1 * time.Hour))
	if len(models) != 3 {
		t.Fatalf("len(ModelsSince) = %d, want 3 from files not recent cache", len(models))
	}
	if models[0].Model != "model-a" || models[0].Calls != 2 {
		t.Fatalf("models[0] = %+v, want model-a with 2 calls", models[0])
	}
}

func TestStoreSummarySinceUsesShortTTLCache(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	now := base
	store, err := NewStore(Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 10,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	since := base.Add(-1 * time.Hour)
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-30 * time.Minute),
		Model:       "model-a",
		Success:     true,
		LatencyMs:   100,
		UsageSource: UsageSourceUnknown,
	})

	if got := store.SummarySince(since).TotalCalls; got != 1 {
		t.Fatalf("initial TotalCalls = %d, want 1", got)
	}

	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-10 * time.Minute),
		Model:       "model-b",
		Success:     true,
		LatencyMs:   100,
		UsageSource: UsageSourceUnknown,
	})

	if got := store.SummarySince(since).TotalCalls; got != 1 {
		t.Fatalf("cached TotalCalls = %d, want 1 before TTL refresh", got)
	}

	now = now.Add(3 * time.Second)
	if got := store.SummarySince(since).TotalCalls; got != 2 {
		t.Fatalf("refreshed TotalCalls = %d, want 2 after TTL refresh", got)
	}
}

func TestStoreQueryLogsUsesShortTTLCacheAndKeepsFilters(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	now := base
	store, err := NewStore(Options{
		Dir:              t.TempDir(),
		RetentionDays:    30,
		MaxRecentRecords: 10,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	since := base.Add(-1 * time.Hour)
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-30 * time.Minute),
		Model:       "model-a",
		Success:     false,
		Status:      503,
		LatencyMs:   100,
		UsageSource: UsageSourceUnknown,
	})
	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-20 * time.Minute),
		Model:       "model-b",
		Success:     true,
		Status:      200,
		LatencyMs:   100,
		UsageSource: UsageSourceUnknown,
	})

	filter := CallLogFilter{Model: "model-b", Status: "success", Page: 1, PageSize: 10}
	if got := store.QueryLogs(since, filter).Total; got != 1 {
		t.Fatalf("initial logs total = %d, want 1", got)
	}

	mustAppendRecord(t, store, CallRecord{
		At:          base.Add(-10 * time.Minute),
		Model:       "model-b",
		Success:     true,
		Status:      200,
		LatencyMs:   100,
		UsageSource: UsageSourceUnknown,
	})

	if got := store.QueryLogs(since, filter).Total; got != 1 {
		t.Fatalf("cached logs total = %d, want 1 before TTL refresh", got)
	}

	now = now.Add(3 * time.Second)
	if got := store.QueryLogs(since, filter).Total; got != 2 {
		t.Fatalf("refreshed logs total = %d, want 2 after TTL refresh", got)
	}
}

func mustAppendRecord(t *testing.T, store *Store, record CallRecord) {
	t.Helper()
	if err := store.Append(record); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
}
