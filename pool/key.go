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
	last401At      atomic.Int64 // unix nano
	ReqCount       atomic.Int64
	ErrCount       atomic.Int64
	totalLatencyMs atomic.Int64
	mu             sync.Mutex
}

// newKey 创建默认 active 状态的 key。
func newKey(value string) *Key {
	k := &Key{Value: value}
	k.state.Store(int32(StateActive))
	return k
}

// State 返回 key 当前状态。
func (k *Key) State() KeyState {
	return KeyState(k.state.Load())
}

// IsAvailable 判断 key 是否可用，并在冷却到期时自动恢复 active。
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

// MarkCooling 将 key 标记为冷却状态，并记录错误次数。
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

// MarkInvalid 将 key 标记为失效状态，并记录最近一次 401 时间。
func (k *Key) MarkInvalid() {
	k.state.Store(int32(StateInvalid))
	k.last401At.Store(time.Now().UnixNano())
	k.ErrCount.Add(1)
}

// RecordLatency 累加上游请求延迟，用于计算平均延迟。
func (k *Key) RecordLatency(d time.Duration) {
	k.totalLatencyMs.Add(d.Milliseconds())
}

// AvgLatencyMs 返回平均延迟毫秒数。
func (k *Key) AvgLatencyMs() float64 {
	reqs := k.ReqCount.Load()
	if reqs == 0 {
		return 0
	}
	return float64(k.totalLatencyMs.Load()) / float64(reqs)
}

// CoolUntil 返回 cooling 状态的结束时间。
func (k *Key) CoolUntil() time.Time {
	ns := k.coolUntil.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// Last401At 返回最近一次 401 时间。
func (k *Key) Last401At() time.Time {
	ns := k.last401At.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}
