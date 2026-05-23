package pool

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNoAvailableKey = errors.New("no available keys in pool")

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

// Next returns the next available key using round-robin, skipping cooling/invalid keys.
func (p *Pool) Next() (*Key, error) {
	p.mu.RLock()
	keys := p.keys
	p.mu.RUnlock()

	n := int64(len(keys))
	if n == 0 {
		return nil, ErrNoAvailableKey
	}

	start := p.cursor.Load()
	for i := int64(0); i < n; i++ {
		idx := (start + i) % n
		k := keys[idx]
		if k.IsAvailable() {
			p.cursor.Store((idx + 1) % n)
			k.ReqCount.Add(1)
			return k, nil
		}
	}
	return nil, ErrNoAvailableKey
}

// Update replaces the key pool with a new set of keys.
// Existing keys that are still present preserve their state and stats.
// Keys removed from the new list are dropped; new keys start as Active.
func (p *Pool) Update(newValues []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	existing := make(map[string]*Key, len(p.keys))
	for _, k := range p.keys {
		existing[k.Value] = k
	}

	newKeys := make([]*Key, 0, len(newValues))
	for _, v := range newValues {
		if k, ok := existing[v]; ok {
			newKeys = append(newKeys, k) // preserve state + stats
		} else {
			newKeys = append(newKeys, newKey(v))
		}
	}

	p.keys = newKeys
	p.cursor.Store(0)
}

type KeyStatus struct {
	Index        int       `json:"index"`
	State        string    `json:"state"`
	ReqCount     int64     `json:"req_count"`
	ErrCount     int64     `json:"err_count"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	CoolUntil    time.Time `json:"cool_until,omitempty"`
}

func (p *Pool) Status() []KeyStatus {
	p.mu.RLock()
	keys := p.keys
	p.mu.RUnlock()

	out := make([]KeyStatus, len(keys))
	for i, k := range keys {
		s := KeyStatus{
			Index:        i,
			ReqCount:     k.ReqCount.Load(),
			ErrCount:     k.ErrCount.Load(),
			AvgLatencyMs: k.AvgLatencyMs(),
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

func (p *Pool) TotalCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.keys)
}

func (p *Pool) CoolingDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
