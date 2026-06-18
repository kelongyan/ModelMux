package pool

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	InvalidReasonUnauthorized   = "unauthorized"
	InvalidReasonQuotaExhausted = "quota_exhausted"
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
	invalidReason  atomic.Value // string
	ReqCount       atomic.Int64
	ErrCount       atomic.Int64
	connectionErrs atomic.Int64
	inFlight       atomic.Int64
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
// CAS 失败说明状态已被并发改写（例如同一时刻 MarkInvalid），此时不能虚报为可用。
func (k *Key) IsAvailable() bool {
	switch k.State() {
	case StateActive:
		return true
	case StateCooling:
		if time.Now().UnixNano() >= k.coolUntil.Load() {
			return k.state.CompareAndSwap(int32(StateCooling), int32(StateActive))
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

// MarkConnectionCooling 将 key 标记为连接类短冷却，并记录连续连接失败次数。
func (k *Key) MarkConnectionCooling(duration time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.State() == StateInvalid {
		return
	}
	k.coolUntil.Store(time.Now().Add(duration).UnixNano())
	k.state.Store(int32(StateCooling))
	k.ErrCount.Add(1)
	k.connectionErrs.Add(1)
}

// MarkInvalid 将 key 标记为失效状态，并记录最近一次 401 时间。
func (k *Key) MarkInvalid() {
	k.MarkInvalidWithReason(InvalidReasonUnauthorized)
}

// MarkInvalidWithReason 将 key 标记为失效状态，并记录失效原因。
func (k *Key) MarkInvalidWithReason(reason string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.state.Store(int32(StateInvalid))
	k.last401At.Store(time.Now().UnixNano())
	k.invalidReason.Store(reason)
	k.ErrCount.Add(1)
}

// ResetActive 手动把 key 恢复为 active，并清理冷却截止时间。
func (k *Key) ResetActive() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.coolUntil.Store(0)
	k.state.Store(int32(StateActive))
	k.invalidReason.Store("")
	k.connectionErrs.Store(0)
}

// BeginRequest 记录一次进入上游的请求，并增加当前 key 的并发占用数。
func (k *Key) BeginRequest() {
	k.ReqCount.Add(1)
	k.inFlight.Add(1)
}

// FinishRequest 释放当前 key 的并发占用数，防止异常路径导致计数为负。
func (k *Key) FinishRequest() {
	for {
		current := k.inFlight.Load()
		if current <= 0 {
			return
		}
		if k.inFlight.CompareAndSwap(current, current-1) {
			return
		}
	}
}

// InFlight 返回当前正在使用该 key 的请求数量。
func (k *Key) InFlight() int64 {
	return k.inFlight.Load()
}

// ConnectionFailureCount 返回当前连续连接类失败次数。
func (k *Key) ConnectionFailureCount() int64 {
	return k.connectionErrs.Load()
}

// ResetConnectionFailures 清理连续连接类失败次数。
func (k *Key) ResetConnectionFailures() {
	k.connectionErrs.Store(0)
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

// InvalidReason 返回 key 最近一次被标记 invalid 的原因。
func (k *Key) InvalidReason() string {
	reason, _ := k.invalidReason.Load().(string)
	return reason
}

func (k *Key) SetInvalidReason(reason string) {
	k.invalidReason.Store(reason)
}
