package pool

import (
	"sync"
	"sync/atomic"
	"time"
)

type KeyState int32

const (
	StateActive  KeyState = 0
	StateCooling KeyState = 1
	StateInvalid KeyState = 2
)

type Key struct {
	Value          string
	state          atomic.Int32
	coolUntil      atomic.Int64 // unix nano
	ReqCount       atomic.Int64
	ErrCount       atomic.Int64
	totalLatencyMs atomic.Int64
	mu             sync.Mutex
}

func newKey(value string) *Key {
	k := &Key{Value: value}
	k.state.Store(int32(StateActive))
	return k
}

func (k *Key) State() KeyState {
	return KeyState(k.state.Load())
}

func (k *Key) IsAvailable() bool {
	switch k.State() {
	case StateActive:
		return true
	case StateCooling:
		if time.Now().UnixNano() >= k.coolUntil.Load() {
			k.state.CompareAndSwap(int32(StateCooling), int32(StateActive))
			return true
		}
		return false
	default:
		return false
	}
}

func (k *Key) MarkCooling(duration time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.State() == StateInvalid {
		return
	}
	k.coolUntil.Store(time.Now().Add(duration).UnixNano())
	k.state.Store(int32(StateCooling))
	k.ErrCount.Add(1)
}

func (k *Key) MarkInvalid() {
	k.state.Store(int32(StateInvalid))
	k.ErrCount.Add(1)
}

func (k *Key) RecordLatency(d time.Duration) {
	k.totalLatencyMs.Add(d.Milliseconds())
}

func (k *Key) AvgLatencyMs() float64 {
	reqs := k.ReqCount.Load()
	if reqs == 0 {
		return 0
	}
	return float64(k.totalLatencyMs.Load()) / float64(reqs)
}

func (k *Key) CoolUntil() time.Time {
	ns := k.coolUntil.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}
