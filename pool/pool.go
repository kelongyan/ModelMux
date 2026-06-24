package pool

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/state"
)

var ErrNoAvailableKey = errors.New("no available keys in pool")
var ErrKeyNotFound = errors.New("key not found in pool")

type Pool struct {
	keys   []*Key
	cursor atomic.Int64
	mu     sync.RWMutex
}

func New(keyValues []string) *Pool {
	p := &Pool{}
	p.keys = makeKeys(keyValues)
	return p
}

func makeKeys(values []string) []*Key {
	keys := make([]*Key, len(values))
	for i, v := range values {
		keys[i] = newKey(v)
	}
	return keys
}

// Next 使用带 in-flight 感知的 round-robin 返回下一个可用 key，并跳过 cooling/invalid key。
func (p *Pool) Next() (*Key, error) {
	p.mu.RLock()
	keys := p.keys
	p.mu.RUnlock()

	n := int64(len(keys))
	if n == 0 {
		return nil, ErrNoAvailableKey
	}

	start := p.cursor.Add(1) - 1
	var selected *Key
	var selectedInFlight int64
	for i := int64(0); i < n; i++ {
		idx := (start + i) % n
		k := keys[idx]
		if !k.IsAvailable() {
			continue
		}
		inFlight := k.InFlight()
		if selected == nil || inFlight < selectedInFlight {
			selected = k
			selectedInFlight = inFlight
			if inFlight == 0 {
				break
			}
		}
	}
	if selected == nil {
		return nil, ErrNoAvailableKey
	}
	selected.BeginRequest()
	return selected, nil
}

// Update 用新 key 列表更新 key 池；已存在 key 会保留状态和统计，新 key 从 active 开始。
// 仅在 key 列表实际发生变化时重置 cursor，避免频繁热重载导致 round-robin 负载不均。
func (p *Pool) Update(newValues []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if keysEqual(p.keys, newValues) {
		return
	}

	existing := make(map[string]*Key, len(p.keys))
	for _, k := range p.keys {
		existing[k.Value] = k
	}

	newKeys := make([]*Key, 0, len(newValues))
	for _, v := range newValues {
		if k, ok := existing[v]; ok {
			newKeys = append(newKeys, k)
		} else {
			newKeys = append(newKeys, newKey(v))
		}
	}
	p.keys = newKeys
	p.cursor.Store(0)
}

// keysEqual 比较当前 key 指针切片与新 key 值切片是否完全一致（顺序和值）。
func keysEqual(current []*Key, newValues []string) bool {
	if len(current) != len(newValues) {
		return false
	}
	for i, k := range current {
		if k.Value != newValues[i] {
			return false
		}
	}
	return true
}

// ResetKeyByID 按 key 哈希标识恢复对应 key 为 active，便于管理台手动解除摘除状态。
func (p *Pool) ResetKeyByID(keyID string) error {
	p.mu.RLock()
	keys := append([]*Key(nil), p.keys...)
	p.mu.RUnlock()

	for _, key := range keys {
		if state.KeyID(key.Value) != keyID {
			continue
		}
		key.ResetActive()
		return nil
	}
	return ErrKeyNotFound
}

// ResetAll 恢复当前池内所有 key 为 active，返回被处理的 key 数量。
func (p *Pool) ResetAll() int {
	p.mu.RLock()
	keys := append([]*Key(nil), p.keys...)
	p.mu.RUnlock()

	for _, key := range keys {
		key.ResetActive()
	}
	return len(keys)
}

// Snapshot 返回当前 key 池可持久化快照，使用 key hash 标识而不暴露完整 key。
func (p *Pool) Snapshot() []state.KeyRecord {
	p.mu.RLock()
	keys := append([]*Key(nil), p.keys...)
	p.mu.RUnlock()

	records := make([]state.KeyRecord, 0, len(keys))
	for _, k := range keys {
		records = append(records, state.KeyRecord{
			KeyID:          state.KeyID(k.Value),
			State:          stateName(k.State()),
			CoolUntil:      k.CoolUntil(),
			ReqCount:       k.ReqCount.Load(),
			ErrCount:       k.ErrCount.Load(),
			TotalLatencyMs: k.totalLatencyMs.Load(),
			Last401At:      k.Last401At(),
			InvalidReason:  k.InvalidReason(),
		})
	}
	return records
}

// Restore 根据状态文件恢复 key 池状态；配置中不存在的 key 状态会被忽略。
func (p *Pool) Restore(records []state.KeyRecord, invalidTTL time.Duration) {
	byID := make(map[string]state.KeyRecord, len(records))
	for _, record := range records {
		byID[record.KeyID] = record
	}

	now := time.Now()
	p.mu.RLock()
	keys := append([]*Key(nil), p.keys...)
	p.mu.RUnlock()

	for _, k := range keys {
		record, ok := byID[state.KeyID(k.Value)]
		if !ok {
			continue
		}
		k.ReqCount.Store(record.ReqCount)
		k.ErrCount.Store(record.ErrCount)
		k.totalLatencyMs.Store(record.TotalLatencyMs)
		k.last401At.Store(timeToUnixNano(record.Last401At))
		k.SetInvalidReason(record.InvalidReason)
		k.coolUntil.Store(0)
		k.state.Store(int32(StateActive))

		switch record.State {
		case "cooling":
			if record.CoolUntil.After(now) {
				k.coolUntil.Store(record.CoolUntil.UnixNano())
				k.state.Store(int32(StateCooling))
			}
		case "invalid":
			if shouldRestoreInvalid(record.Last401At, invalidTTL, now) {
				k.state.Store(int32(StateInvalid))
			}
		}
	}
}

// stateName 将内部状态转换为持久化使用的字符串。
func stateName(s KeyState) string {
	switch s {
	case StateCooling:
		return "cooling"
	case StateInvalid:
		return "invalid"
	default:
		return "active"
	}
}

// shouldRestoreInvalid 判断 invalid 状态是否仍在 TTL 内。
func shouldRestoreInvalid(last401At time.Time, invalidTTL time.Duration, now time.Time) bool {
	if invalidTTL <= 0 || last401At.IsZero() {
		return false
	}
	return now.Sub(last401At) <= invalidTTL
}

// timeToUnixNano 把零值时间安全转换为 unix nano。
func timeToUnixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

type KeyStatus struct {
	Index         int       `json:"index"`
	KeyID         string    `json:"key_id"`
	MaskedKey     string    `json:"masked_key"`
	State         string    `json:"state"`
	ReqCount      int64     `json:"req_count"`
	ErrCount      int64     `json:"err_count"`
	InFlight      int64     `json:"in_flight"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	CoolUntil     time.Time `json:"cool_until,omitempty"`
	Last401At     time.Time `json:"last_401_at,omitempty"`
	InvalidReason string    `json:"invalid_reason,omitempty"`
}

func (p *Pool) Status() []KeyStatus {
	p.mu.RLock()
	keys := p.keys
	p.mu.RUnlock()

	out := make([]KeyStatus, len(keys))
	for i, k := range keys {
		s := KeyStatus{
			Index:         i,
			KeyID:         state.KeyID(k.Value),
			MaskedKey:     logx.MaskSecret(k.Value),
			ReqCount:      k.ReqCount.Load(),
			ErrCount:      k.ErrCount.Load(),
			InFlight:      k.InFlight(),
			AvgLatencyMs:  k.AvgLatencyMs(),
			Last401At:     k.Last401At(),
			InvalidReason: k.InvalidReason(),
		}
		switch k.State() {
		case StateActive:
			s.State = "active"
		case StateCooling:
			s.State = "cooling"
			s.CoolUntil = k.CoolUntil()
		case StateInvalid:
			s.State = "invalid"
		}
		out[i] = s
	}
	return out
}

// ActiveCount 返回当前可用 key 数量。
func (p *Pool) ActiveCount() int {
	p.mu.RLock()
	keys := p.keys
	p.mu.RUnlock()

	count := 0
	for _, k := range keys {
		if k.IsAvailable() {
			count++
		}
	}
	return count
}

// NextAvailableIn 返回最近一个 cooling key 恢复为可用所需的时间。
func (p *Pool) NextAvailableIn(now time.Time) (time.Duration, bool) {
	p.mu.RLock()
	keys := p.keys
	p.mu.RUnlock()

	var next time.Duration
	found := false
	for _, key := range keys {
		if key.State() != StateCooling {
			continue
		}
		coolUntil := key.CoolUntil()
		if coolUntil.IsZero() || !coolUntil.After(now) {
			return 0, true
		}
		wait := coolUntil.Sub(now)
		if !found || wait < next {
			next = wait
			found = true
		}
	}
	return next, found
}

// TotalCount 返回 key 池总数量。
func (p *Pool) TotalCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.keys)
}

// AnyKeyValue 返回池中第一个可用 key 的值，供管理接口调用上游 API 时使用。
func (p *Pool) AnyKeyValue() string {
	p.mu.RLock()
	keys := p.keys
	p.mu.RUnlock()

	for _, k := range keys {
		if k.IsAvailable() {
			return k.Value
		}
	}
	// 所有 key 都不可用时返回第一把 key 的值。
	if len(keys) > 0 {
		return keys[0].Value
	}
	return ""
}

// CoolingDuration 把冷却秒数转换为 time.Duration。
func (p *Pool) CoolingDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
