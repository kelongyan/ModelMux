package pool

import (
	"sync"
	"testing"
	"time"

	"github.com/kelongyan/ModelMux/state"
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

func TestNextPrefersLowerInFlightKey(t *testing.T) {
	p := New([]string{"k1", "k2", "k3"})
	p.keys[0].BeginRequest()
	p.keys[0].BeginRequest()
	p.keys[1].BeginRequest()
	p.cursor.Store(0)

	k, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if k.Value != "k3" {
		t.Fatalf("Next() key = %q, want k3 with lower in-flight", k.Value)
	}
	if k.InFlight() != 1 {
		t.Fatalf("k3 InFlight = %d, want 1 after selection", k.InFlight())
	}
}

func TestFinishRequestDoesNotGoNegative(t *testing.T) {
	k := newKey("k1")
	k.BeginRequest()
	k.BeginRequest()

	k.FinishRequest()
	if got := k.InFlight(); got != 1 {
		t.Fatalf("InFlight = %d, want 1", got)
	}
	k.FinishRequest()
	k.FinishRequest()
	if got := k.InFlight(); got != 0 {
		t.Fatalf("InFlight = %d, want 0 after extra finish", got)
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

func TestNextAvailableInUsesSoonestCoolingKey(t *testing.T) {
	p := New([]string{"k1", "k2", "k3"})
	p.keys[0].MarkCooling(200 * time.Millisecond)
	p.keys[1].MarkCooling(50 * time.Millisecond)

	wait, ok := p.NextAvailableIn(time.Now())
	if !ok {
		t.Fatal("NextAvailableIn() ok = false, want true")
	}
	if wait <= 0 || wait > 150*time.Millisecond {
		t.Fatalf("wait = %v, want soonest cooling window", wait)
	}
}

func TestNextAvailableInReturnsZeroForExpiredCooling(t *testing.T) {
	p := New([]string{"k1"})
	p.keys[0].coolUntil.Store(time.Now().Add(-time.Millisecond).UnixNano())
	p.keys[0].state.Store(int32(StateCooling))

	wait, ok := p.NextAvailableIn(time.Now())
	if !ok {
		t.Fatal("NextAvailableIn() ok = false, want true")
	}
	if wait != 0 {
		t.Fatalf("wait = %v, want 0", wait)
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
	inFlight := int64(0)
	for _, s := range p.Status() {
		inFlight += s.InFlight
	}
	if inFlight != goroutines*callsEach {
		t.Errorf("expected %d in-flight requests, got %d", goroutines*callsEach, inFlight)
	}
}

func TestSnapshotDoesNotExposeRawKeys(t *testing.T) {
	p := New([]string{"sk-secret"})
	k, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	k.RecordLatency(10 * time.Millisecond)

	snapshots := p.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("len(Snapshot()) = %d, want 1", len(snapshots))
	}
	if snapshots[0].KeyID == "sk-secret" {
		t.Fatal("Snapshot() exposed raw key")
	}
	if snapshots[0].KeyID != state.KeyID("sk-secret") {
		t.Fatalf("KeyID = %q, want %q", snapshots[0].KeyID, state.KeyID("sk-secret"))
	}
	if snapshots[0].ReqCount != 1 {
		t.Fatalf("ReqCount = %d, want 1", snapshots[0].ReqCount)
	}
	if snapshots[0].TotalLatencyMs != 10 {
		t.Fatalf("TotalLatencyMs = %d, want 10", snapshots[0].TotalLatencyMs)
	}
}

func TestRestoreCoolingState(t *testing.T) {
	p := New([]string{"k1"})
	coolUntil := time.Now().Add(time.Hour)

	p.Restore([]state.KeyRecord{{
		KeyID:     state.KeyID("k1"),
		State:     "cooling",
		CoolUntil: coolUntil,
		ReqCount:  5,
		ErrCount:  1,
	}}, 24*time.Hour)

	if p.keys[0].State() != StateCooling {
		t.Fatalf("State = %v, want StateCooling", p.keys[0].State())
	}
	if p.keys[0].ReqCount.Load() != 5 {
		t.Fatalf("ReqCount = %d, want 5", p.keys[0].ReqCount.Load())
	}
}

func TestRestoreExpiredCoolingAsActive(t *testing.T) {
	p := New([]string{"k1"})

	p.Restore([]state.KeyRecord{{
		KeyID:     state.KeyID("k1"),
		State:     "cooling",
		CoolUntil: time.Now().Add(-time.Hour),
	}}, 24*time.Hour)

	if p.keys[0].State() != StateActive {
		t.Fatalf("State = %v, want StateActive", p.keys[0].State())
	}
}

func TestRestoreInvalidOnlyWithinTTL(t *testing.T) {
	p := New([]string{"k1", "k2"})

	p.Restore([]state.KeyRecord{
		{
			KeyID:     state.KeyID("k1"),
			State:     "invalid",
			Last401At: time.Now().Add(-time.Hour),
		},
		{
			KeyID:     state.KeyID("k2"),
			State:     "invalid",
			Last401At: time.Now().Add(-48 * time.Hour),
		},
	}, 24*time.Hour)

	if p.keys[0].State() != StateInvalid {
		t.Fatalf("k1 State = %v, want StateInvalid", p.keys[0].State())
	}
	if p.keys[1].State() != StateActive {
		t.Fatalf("k2 State = %v, want StateActive", p.keys[1].State())
	}
}

// TestIsAvailableRespectsConcurrentInvalidation 复现 cooling 到期与 MarkInvalid 并发时的 race：
// 旧实现里只要冷却到期 IsAvailable 必返回 true，但 invalid 状态此时已生效，
// 调用方就会拿一个 invalid key 去打上游、浪费一次重试预算。
func TestIsAvailableRespectsConcurrentInvalidation(t *testing.T) {
	k := newKey("k1")
	// 让 cooling 已到期，CAS 路径可达。
	k.state.Store(int32(StateCooling))
	k.coolUntil.Store(time.Now().Add(-time.Hour).UnixNano())
	// 模拟另一条路径同时把 key 标成 invalid。
	k.MarkInvalid()

	if k.IsAvailable() {
		t.Fatalf("IsAvailable() = true, want false after MarkInvalid")
	}
	if k.State() != StateInvalid {
		t.Fatalf("State = %v, want StateInvalid (must not be overwritten by CAS)", k.State())
	}
}

// TestIsAvailableConcurrentCoolingExpiryAndInvalidate 在 race 检测器下验证：
// 多 goroutine 同时调用 IsAvailable / MarkInvalid 时，IsAvailable 不会反悔已生效的 invalid 状态。
// 用 go test -race 运行可以捕捉到状态被错误覆盖的情况。
func TestIsAvailableConcurrentCoolingExpiryAndInvalidate(t *testing.T) {
	const rounds = 200

	for i := 0; i < rounds; i++ {
		k := newKey("k1")
		k.state.Store(int32(StateCooling))
		k.coolUntil.Store(time.Now().Add(-time.Millisecond).UnixNano())

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			k.IsAvailable()
		}()
		go func() {
			defer wg.Done()
			k.MarkInvalid()
		}()
		wg.Wait()

		// MarkInvalid 是单向状态迁移，最终状态必须是 invalid，
		// IsAvailable 的 CAS 不能把它改回 active。
		if k.State() != StateInvalid {
			t.Fatalf("round %d: State = %v, want StateInvalid", i, k.State())
		}
	}
}
