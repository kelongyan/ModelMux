package pool

import (
	"sync"
	"testing"
	"time"
)

func TestRoundRobin(t *testing.T) {
	p := New([]string{"k1", "k2", "k3"})

	seen := make([]string, 6)
	for i := range seen {
		k, err := p.Next()
		if err != nil {
			t.Fatalf("unexpected error at i=%d: %v", i, err)
		}
		seen[i] = k.Value
	}

	// Should cycle: k1 k2 k3 k1 k2 k3
	want := []string{"k1", "k2", "k3", "k1", "k2", "k3"}
	for i, v := range want {
		if seen[i] != v {
			t.Errorf("position %d: got %q, want %q", i, seen[i], v)
		}
	}
}

func TestSkipCoolingKey(t *testing.T) {
	p := New([]string{"k1", "k2", "k3"})

	// Mark k2 (index 1) as cooling for a long time.
	p.keys[1].MarkCooling(10 * time.Minute)

	for i := 0; i < 6; i++ {
		k, err := p.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if k.Value == "k2" {
			t.Errorf("got cooling key k2 at iteration %d", i)
		}
	}
}

func TestSkipInvalidKey(t *testing.T) {
	p := New([]string{"k1", "k2", "k3"})
	p.keys[0].MarkInvalid()

	for i := 0; i < 6; i++ {
		k, err := p.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if k.Value == "k1" {
			t.Errorf("got invalid key k1 at iteration %d", i)
		}
	}
}

func TestAllKeysUnavailable(t *testing.T) {
	p := New([]string{"k1", "k2"})
	p.keys[0].MarkInvalid()
	p.keys[1].MarkInvalid()

	_, err := p.Next()
	if err != ErrNoAvailableKey {
		t.Errorf("expected ErrNoAvailableKey, got %v", err)
	}
}

func TestCoolingKeyAutoRecovery(t *testing.T) {
	p := New([]string{"k1"})
	// Cool for 1 nanosecond — effectively already expired.
	p.keys[0].MarkCooling(time.Nanosecond)
	time.Sleep(2 * time.Millisecond)

	k, err := p.Next()
	if err != nil {
		t.Fatalf("expected key to recover, got error: %v", err)
	}
	if k.Value != "k1" {
		t.Errorf("expected k1, got %q", k.Value)
	}
	if k.State() != StateActive {
		t.Errorf("expected StateActive after recovery, got %v", k.State())
	}
}

func TestUpdate_AddNewKey(t *testing.T) {
	p := New([]string{"k1", "k2"})
	p.Update([]string{"k1", "k2", "k3"})

	if p.TotalCount() != 3 {
		t.Errorf("expected 3 keys after update, got %d", p.TotalCount())
	}
}

func TestUpdate_RemoveKey(t *testing.T) {
	p := New([]string{"k1", "k2", "k3"})
	p.Update([]string{"k1", "k3"})

	if p.TotalCount() != 2 {
		t.Errorf("expected 2 keys after update, got %d", p.TotalCount())
	}
	for i := 0; i < 4; i++ {
		k, err := p.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if k.Value == "k2" {
			t.Errorf("removed key k2 still returned")
		}
	}
}

func TestUpdate_PreservesStats(t *testing.T) {
	p := New([]string{"k1", "k2"})
	// Simulate some usage on k1.
	p.keys[0].ReqCount.Add(42)
	p.keys[0].ErrCount.Add(3)

	p.Update([]string{"k1", "k2", "k3"})

	// k1 should still have its stats.
	var k1 *Key
	for _, k := range p.keys {
		if k.Value == "k1" {
			k1 = k
			break
		}
	}
	if k1 == nil {
		t.Fatal("k1 not found after update")
	}
	if k1.ReqCount.Load() != 42 {
		t.Errorf("expected req_count=42, got %d", k1.ReqCount.Load())
	}
}

func TestConcurrentNext(t *testing.T) {
	p := New([]string{"k1", "k2", "k3"})
	const goroutines = 50
	const callsEach = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsEach; j++ {
				k, err := p.Next()
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if k == nil {
					t.Error("got nil key")
					return
				}
			}
		}()
	}
	wg.Wait()

	total := int64(0)
	for _, s := range p.Status() {
		total += s.ReqCount
	}
	if total != goroutines*callsEach {
		t.Errorf("expected %d total requests, got %d", goroutines*callsEach, total)
	}
}
