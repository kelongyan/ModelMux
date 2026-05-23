package pool

import (
	"testing"
	"time"

	"github.com/claude-key-proxy/state"
)

func TestProviderPoolsActiveProviderOnly(t *testing.T) {
	pools, err := NewProviderPools([]ProviderSpec{
		{ID: "p1", Keys: []string{"k1", "k2"}},
		{ID: "p2", Keys: []string{"k3"}},
	}, "p1")
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}

	activeID, keyPool, err := pools.Active()
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if activeID != "p1" {
		t.Fatalf("activeID = %q, want p1", activeID)
	}

	k, err := keyPool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if k.Value != "k1" {
		t.Fatalf("Next() key = %q, want k1", k.Value)
	}

	if p2, err := pools.Get("p2"); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if p2.TotalCount() != 1 {
		t.Fatalf("p2.TotalCount() = %d, want 1", p2.TotalCount())
	}
}

func TestProviderPoolsUpdatePreservesProviderState(t *testing.T) {
	pools, err := NewProviderPools([]ProviderSpec{
		{ID: "p1", Keys: []string{"k1"}},
		{ID: "p2", Keys: []string{"k2"}},
	}, "p1")
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}

	p1, err := pools.Get("p1")
	if err != nil {
		t.Fatalf("Get(p1) error = %v", err)
	}
	p1.keys[0].ReqCount.Add(9)

	if err := pools.Update([]ProviderSpec{
		{ID: "p1", Keys: []string{"k1", "k3"}},
		{ID: "p2", Keys: []string{"k2"}},
	}, "p1"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	status, err := pools.ActiveStatus()
	if err != nil {
		t.Fatalf("ActiveStatus() error = %v", err)
	}
	if status.TotalKeys != 2 {
		t.Fatalf("TotalKeys = %d, want 2", status.TotalKeys)
	}
	if status.Keys[0].ReqCount != 9 {
		t.Fatalf("ReqCount = %d, want 9", status.Keys[0].ReqCount)
	}
}

func TestProviderPoolsSnapshotAndRestore(t *testing.T) {
	pools, err := NewProviderPools([]ProviderSpec{
		{ID: "p1", Keys: []string{"k1"}},
		{ID: "p2", Keys: []string{"k2"}},
	}, "p1")
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}

	snapshots := pools.Snapshot()
	if len(snapshots) != 2 {
		t.Fatalf("len(Snapshot()) = %d, want 2", len(snapshots))
	}
	if snapshots[0].ID != "p1" {
		t.Fatalf("snapshot[0].ID = %q, want p1", snapshots[0].ID)
	}

	restored, err := NewProviderPools([]ProviderSpec{
		{ID: "p1", Keys: []string{"k1"}},
		{ID: "p2", Keys: []string{"k2"}},
	}, "p1")
	if err != nil {
		t.Fatalf("NewProviderPools() error = %v", err)
	}
	restored.Restore([]state.ProviderRecord{{
		ID: "p1",
		Keys: []state.KeyRecord{{
			KeyID:     state.KeyID("k1"),
			State:     "cooling",
			CoolUntil: time.Now().Add(time.Hour),
			ReqCount:  7,
		}},
	}}, 24*time.Hour)

	statuses := restored.Status()
	if len(statuses) != 2 {
		t.Fatalf("len(Status()) = %d, want 2", len(statuses))
	}
	if statuses[0].ID != "p1" || !statuses[0].Active {
		t.Fatalf("status[0] = %+v, want active p1", statuses[0])
	}
	if statuses[0].CoolingKeys != 1 {
		t.Fatalf("CoolingKeys = %d, want 1", statuses[0].CoolingKeys)
	}
}
