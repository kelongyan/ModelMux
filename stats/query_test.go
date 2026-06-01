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

func mustAppendRecord(t *testing.T, store *Store, record CallRecord) {
	t.Helper()
	if err := store.Append(record); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
}
