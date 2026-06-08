package stats

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAppendPersistsDailyJSONLAndKeepsRecent(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
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

	record := CallRecord{
		ProviderID:       "primary",
		Model:            "gpt-4.1-mini",
		Endpoint:         "/v1/chat/completions",
		Method:           "POST",
		Status:           200,
		Success:          true,
		LatencyMs:        1234,
		Attempts:         2,
		KeyID:            "sha256:abc",
		PromptTokens:     int64Ptr(10),
		CompletionTokens: int64Ptr(20),
		TotalTokens:      int64Ptr(30),
		UsageSource:      UsageSourceUpstream,
	}
	if err := store.Append(record); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	recent := store.Recent(10)
	if len(recent) != 1 {
		t.Fatalf("len(Recent) = %d, want 1", len(recent))
	}
	if recent[0].ID == "" {
		t.Fatal("Recent()[0].ID is empty")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	path := filepath.Join(store.dir, "calls-2026-06-01.jsonl")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open stats file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("stats file has no first line")
	}
	var persisted CallRecord
	if err := json.Unmarshal(scanner.Bytes(), &persisted); err != nil {
		t.Fatalf("decode persisted record: %v", err)
	}
	if persisted.ID == "" {
		t.Fatal("persisted ID is empty")
	}
	if !persisted.At.Equal(now) {
		t.Fatalf("At = %s, want %s", persisted.At, now)
	}
	if persisted.Model != "gpt-4.1-mini" || persisted.ProviderID != "primary" {
		t.Fatalf("persisted record = %+v", persisted)
	}
	if persisted.TotalTokens == nil || *persisted.TotalTokens != 30 {
		t.Fatalf("TotalTokens = %v, want 30", persisted.TotalTokens)
	}
	if recent[0].ID != persisted.ID {
		t.Fatalf("Recent()[0].ID = %q, want %q", recent[0].ID, persisted.ID)
	}
}

func TestStoreCloseFlushesQueuedRecords(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
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

	for i := 0; i < 3; i++ {
		if err := store.Append(CallRecord{
			ProviderID:  "primary",
			Model:       "gpt-4.1-mini",
			Endpoint:    "/v1/chat/completions",
			Method:      "POST",
			Status:      200,
			Success:     true,
			LatencyMs:   int64(i + 1),
			UsageSource: UsageSourceUnknown,
		}); err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}
	if got := len(store.Recent(10)); got != 3 {
		t.Fatalf("len(Recent) = %d, want 3 before Close", got)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	path := filepath.Join(store.dir, "calls-2026-06-01.jsonl")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open stats file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan stats file: %v", err)
	}
	if lines != 3 {
		t.Fatalf("persisted lines = %d, want 3", lines)
	}
}

func TestStoreReportsQueueDepthAndCapacity(t *testing.T) {
	store := &Store{commands: make(chan writeCommand, 3)}
	store.commands <- writeCommand{}
	store.commands <- writeCommand{}

	if got := store.QueueDepth(); got != 2 {
		t.Fatalf("QueueDepth() = %d, want 2", got)
	}
	if got := store.QueueCapacity(); got != 3 {
		t.Fatalf("QueueCapacity() = %d, want 3", got)
	}
}

func TestStoreLoadsExistingRecordsAndSkipsInvalidLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "calls-2026-06-01.jsonl")
	if err := os.WriteFile(path, []byte(
		"{\"id\":\"one\",\"at\":\"2026-06-01T10:00:00Z\",\"model\":\"gpt-4.1\"}\n"+
			"not-json\n"+
			"{\"id\":\"two\",\"at\":\"2026-06-01T11:00:00Z\",\"model\":\"gpt-4.1-mini\"}\n",
	), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	store, err := NewStore(Options{
		Dir:              dir,
		RetentionDays:    30,
		MaxRecentRecords: 10,
		Now:              func() time.Time { return time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	recent := store.Recent(10)
	if len(recent) != 2 {
		t.Fatalf("len(Recent) = %d, want 2", len(recent))
	}
	if recent[0].ID != "one" || recent[1].ID != "two" {
		t.Fatalf("Recent IDs = %q, %q; want one, two", recent[0].ID, recent[1].ID)
	}
}

func TestStoreCleansExpiredFilesAndCapsRecentRecords(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "calls-2026-04-01.jsonl")
	keepPath := filepath.Join(dir, "calls-2026-06-01.jsonl")
	if err := os.WriteFile(oldPath, []byte("{\"id\":\"old\",\"at\":\"2026-04-01T00:00:00Z\"}\n"), 0600); err != nil {
		t.Fatalf("write old fixture: %v", err)
	}
	if err := os.WriteFile(keepPath, []byte(
		"{\"id\":\"one\",\"at\":\"2026-06-01T10:00:00Z\"}\n"+
			"{\"id\":\"two\",\"at\":\"2026-06-01T11:00:00Z\"}\n"+
			"{\"id\":\"three\",\"at\":\"2026-06-01T12:00:00Z\"}\n",
	), 0600); err != nil {
		t.Fatalf("write keep fixture: %v", err)
	}

	store, err := NewStore(Options{
		Dir:              dir,
		RetentionDays:    30,
		MaxRecentRecords: 2,
		Now:              func() time.Time { return time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old stats file still exists or stat error = %v", err)
	}
	recent := store.Recent(10)
	if len(recent) != 2 {
		t.Fatalf("len(Recent) = %d, want 2", len(recent))
	}
	if recent[0].ID != "two" || recent[1].ID != "three" {
		t.Fatalf("Recent IDs = %q, %q; want two, three", recent[0].ID, recent[1].ID)
	}
}

func TestStoreLoadsRecentRecordsFromNewestFilesFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "calls-2026-05-30.jsonl"), []byte(
		"{\"id\":\"old-one\",\"at\":\"2026-05-30T10:00:00Z\"}\n"+
			"{\"id\":\"old-two\",\"at\":\"2026-05-30T11:00:00Z\"}\n",
	), 0600); err != nil {
		t.Fatalf("write old fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calls-2026-06-01.jsonl"), []byte(
		"{\"id\":\"new-one\",\"at\":\"2026-06-01T10:00:00Z\"}\n"+
			"{\"id\":\"new-two\",\"at\":\"2026-06-01T11:00:00Z\"}\n",
	), 0600); err != nil {
		t.Fatalf("write new fixture: %v", err)
	}

	store, err := NewStore(Options{
		Dir:              dir,
		RetentionDays:    30,
		MaxRecentRecords: 2,
		Now:              func() time.Time { return time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	recent := store.Recent(10)
	if len(recent) != 2 {
		t.Fatalf("len(Recent) = %d, want 2", len(recent))
	}
	if recent[0].ID != "new-one" || recent[1].ID != "new-two" {
		t.Fatalf("Recent IDs = %q, %q; want new-one, new-two", recent[0].ID, recent[1].ID)
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}
