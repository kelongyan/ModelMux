package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKeyIDIsStableAndDoesNotExposeKey(t *testing.T) {
	key := "sk-secret"
	first := KeyID(key)
	second := KeyID(key)

	if first != second {
		t.Fatalf("KeyID() is not stable: %q != %q", first, second)
	}
	if first == key {
		t.Fatal("KeyID() exposed the original key")
	}
}

func TestStoreLoadMissingReturnsEmptyState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "state.json"))

	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if file.Version != CurrentVersion {
		t.Fatalf("Version = %d, want %d", file.Version, CurrentVersion)
	}
	if len(file.Keys) != 0 {
		t.Fatalf("len(Keys) = %d, want 0", len(file.Keys))
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	store := NewStore(path)
	now := time.Date(2026, 5, 23, 20, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	records := []ProviderRecord{{
		ID: "default",
		Keys: []KeyRecord{{
			KeyID:          KeyID("sk-test"),
			State:          "cooling",
			CoolUntil:      now.Add(time.Minute),
			ReqCount:       7,
			ErrCount:       2,
			TotalLatencyMs: 1234,
			Last401At:      now.Add(-time.Hour),
		}},
	}}
	if err := store.Save(records); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.SavedAt != now {
		t.Fatalf("SavedAt = %v, want %v", loaded.SavedAt, now)
	}
	if len(loaded.Providers) != 1 {
		t.Fatalf("len(Providers) = %d, want 1", len(loaded.Providers))
	}
	if loaded.Providers[0].Keys[0].ReqCount != 7 {
		t.Fatalf("ReqCount = %d, want 7", loaded.Providers[0].Keys[0].ReqCount)
	}
}

func TestVersionedProviderRecordsMapsLegacyKeysToActiveProvider(t *testing.T) {
	file := &File{
		Version: 1,
		Keys: []KeyRecord{{
			KeyID: KeyID("sk-test"),
			State: "active",
		}},
	}

	records := file.VersionedProviderRecords("zhuji")
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0].ID != "zhuji" {
		t.Fatalf("provider ID = %q, want zhuji", records[0].ID)
	}
	if len(records[0].Keys) != 1 {
		t.Fatalf("len(Keys) = %d, want 1", len(records[0].Keys))
	}
}
