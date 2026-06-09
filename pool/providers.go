package pool

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kelongyan/ModelMux/state"
)

var ErrProviderNotFound = errors.New("provider not found")

type ProviderSpec struct {
	ID   string
	Keys []string
}

type ProviderPools struct {
	mu        sync.RWMutex
	activeID  string
	order     []string
	providers map[string]*Pool
}

type ProviderStatus struct {
	ID          string      `json:"id"`
	Active      bool        `json:"active"`
	TotalKeys   int         `json:"total_keys"`
	ActiveKeys  int         `json:"active_keys"`
	CoolingKeys int         `json:"cooling_keys"`
	InvalidKeys int         `json:"invalid_keys"`
	Keys        []KeyStatus `json:"keys"`
}

// NewProviderPools 为每个 provider 创建独立 key 池，并设置当前选中的 provider。
func NewProviderPools(specs []ProviderSpec, activeID string) (*ProviderPools, error) {
	p := &ProviderPools{}
	if err := p.Update(specs, activeID); err != nil {
		return nil, err
	}
	return p, nil
}

// Update 更新 provider 集合；同 ID provider 会保留原有 key 状态和统计。
func (p *ProviderPools) Update(specs []ProviderSpec, activeID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if activeID == "" {
		return fmt.Errorf("active provider id is required")
	}

	normalized := make([]ProviderSpec, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	activeFound := false

	for _, spec := range specs {
		if spec.ID == "" {
			return fmt.Errorf("provider id is required")
		}
		if _, exists := seen[spec.ID]; exists {
			return fmt.Errorf("duplicate provider id %q", spec.ID)
		}
		seen[spec.ID] = struct{}{}
		normalized = append(normalized, ProviderSpec{
			ID:   spec.ID,
			Keys: append([]string(nil), spec.Keys...),
		})
		if spec.ID == activeID {
			activeFound = true
		}
	}

	if !activeFound {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, activeID)
	}

	nextProviders := make(map[string]*Pool, len(normalized))
	nextOrder := make([]string, 0, len(normalized))
	for _, spec := range normalized {
		keyPool, ok := p.providers[spec.ID]
		if ok {
			keyPool = &Pool{
				keys: keyPool.updatedKeys(spec.Keys),
			}
		} else {
			keyPool = New(spec.Keys)
		}
		nextProviders[spec.ID] = keyPool
		nextOrder = append(nextOrder, spec.ID)
	}

	p.providers = nextProviders
	p.order = nextOrder
	p.activeID = activeID
	return nil
}

// Active 返回当前选中的 provider ID 和对应 key 池。
func (p *ProviderPools) Active() (string, *Pool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keyPool, ok := p.providers[p.activeID]
	if !ok {
		return p.activeID, nil, fmt.Errorf("%w: %s", ErrProviderNotFound, p.activeID)
	}
	return p.activeID, keyPool, nil
}

// Get 返回指定 provider 的 key 池。
func (p *ProviderPools) Get(id string) (*Pool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keyPool, ok := p.providers[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, id)
	}
	return keyPool, nil
}

// ActiveID 返回当前选中的 provider ID。
func (p *ProviderPools) ActiveID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.activeID
}

// ProviderCount 返回 provider 数量。
func (p *ProviderPools) ProviderCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.providers)
}

// Snapshot 返回所有 provider 的可持久化状态快照，按配置顺序输出。
func (p *ProviderPools) Snapshot() []state.ProviderRecord {
	p.mu.RLock()
	order := append([]string(nil), p.order...)
	providers := make(map[string]*Pool, len(p.providers))
	for id, keyPool := range p.providers {
		providers[id] = keyPool
	}
	p.mu.RUnlock()

	records := make([]state.ProviderRecord, 0, len(order))
	for _, id := range order {
		keyPool, ok := providers[id]
		if !ok {
			continue
		}
		records = append(records, state.ProviderRecord{
			ID:   id,
			Keys: keyPool.Snapshot(),
		})
	}
	return records
}

// Restore 按 provider ID 恢复 key 池状态；配置中不存在的 provider 会被忽略。
func (p *ProviderPools) Restore(records []state.ProviderRecord, invalidTTL time.Duration) {
	byID := make(map[string][]state.KeyRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record.Keys
	}

	p.mu.RLock()
	providers := make(map[string]*Pool, len(p.providers))
	for id, keyPool := range p.providers {
		providers[id] = keyPool
	}
	p.mu.RUnlock()

	for id, keyRecords := range byID {
		if keyPool, ok := providers[id]; ok {
			keyPool.Restore(keyRecords, invalidTTL)
		}
	}
}

// Status 返回每个 provider 的 key 状态，便于管理接口展示。
func (p *ProviderPools) Status() []ProviderStatus {
	p.mu.RLock()
	activeID := p.activeID
	order := append([]string(nil), p.order...)
	providers := make(map[string]*Pool, len(p.providers))
	for id, keyPool := range p.providers {
		providers[id] = keyPool
	}
	p.mu.RUnlock()

	statuses := make([]ProviderStatus, 0, len(order))
	for _, id := range order {
		keyPool, ok := providers[id]
		if !ok {
			continue
		}
		statuses = append(statuses, buildProviderStatus(id, id == activeID, keyPool))
	}
	return statuses
}

// ActiveStatus 返回当前 active provider 的状态。
func (p *ProviderPools) ActiveStatus() (ProviderStatus, error) {
	activeID, keyPool, err := p.Active()
	if err != nil {
		return ProviderStatus{}, err
	}
	return buildProviderStatus(activeID, true, keyPool), nil
}

// buildProviderStatus 汇总单个 provider 的 key 状态数量。
func buildProviderStatus(id string, active bool, keyPool *Pool) ProviderStatus {
	keys := keyPool.Status()
	status := ProviderStatus{
		ID:        id,
		Active:    active,
		TotalKeys: keyPool.TotalCount(),
		Keys:      keys,
	}
	for _, keyStatus := range keys {
		switch keyStatus.State {
		case "active":
			status.ActiveKeys++
		case "cooling":
			status.CoolingKeys++
		case "invalid":
			status.InvalidKeys++
		}
	}
	return status
}
