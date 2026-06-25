package admin

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/kelongyan/ModelMux/logx"
)

// AdminEvent 表示管理台最近发生的一条可视化事件。
type AdminEvent struct {
	Seq int64     `json:"seq"`
	At  time.Time `json:"at"`
	logx.Event
}

// EventBuffer 保存最近发生的管理事件，供 Dashboard 和事件页轮询读取。
// 内部使用环形缓冲区，预分配固定容量切片，避免每次超容量时分配新内存。
type EventBuffer struct {
	mu       sync.RWMutex
	capacity int
	seq      atomic.Int64
	ring     []AdminEvent // 预分配的环形缓冲区
	head     int          // 下一个写入位置
	count    int          // 当前已写入的事件数（<= capacity）
}

// NewEventBuffer 创建一个固定容量的事件缓冲区。
func NewEventBuffer(capacity int) *EventBuffer {
	if capacity <= 0 {
		capacity = 200
	}
	return &EventBuffer{
		capacity: capacity,
		ring:     make([]AdminEvent, capacity),
	}
}

// Add 追加一条事件，并在超过容量时丢弃最旧的数据。
func (b *EventBuffer) Add(level, category, event, message string, data map[string]any) {
	b.AddEvent(logx.Event{
		Level:    level,
		Category: category,
		Event:    event,
		Message:  message,
		Data:     data,
	})
}

// AddEvent appends a structured diagnostic event and keeps its fields aligned with slog output.
func (b *EventBuffer) AddEvent(event logx.Event) {
	if b == nil {
		return
	}

	entry := AdminEvent{
		Seq:   b.seq.Add(1),
		At:    time.Now(),
		Event: event,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.ring[b.head] = entry
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
}

// List 返回最近 limit 条事件（按时间升序），limit<=0 时返回全部。
func (b *EventBuffer) List(limit int) []AdminEvent {
	if b == nil {
		return []AdminEvent{}
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return []AdminEvent{}
	}

	// 环形缓冲区中事件的时间顺序：从 oldest 开始到最新。
	// 当 buffer 未满时，oldest 在 index 0，最新在 index head-1。
	// 当 buffer 已满时，oldest 在 index head（因为 head 刚覆盖了最旧的位置），最新在 head-1。
	// 但由于 Seq 是递增的，我们按 Seq 排序输出更安全。
	var startIdx int
	if b.count < b.capacity {
		// 未满：有效数据在 [0, count)
		startIdx = 0
	} else {
		// 已满：head 指向最旧的位置
		startIdx = b.head
	}

	// 收集有效事件到有序切片
	ordered := make([]AdminEvent, b.count)
	for i := 0; i < b.count; i++ {
		ordered[i] = b.ring[(startIdx+i)%b.capacity]
	}

	if limit <= 0 || limit >= len(ordered) {
		return ordered
	}
	// 返回最后 limit 条（最新的）
	return ordered[len(ordered)-limit:]
}

// EventFilter 事件过滤条件。
type EventFilter struct {
	Level    string
	Category string
	Since    *time.Time
	Limit    int
}

// Since 返回指定 seq 之后的所有事件，用于 SSE 增量推送。
func (b *EventBuffer) Since(lastSeq int64) []AdminEvent {
	if b == nil {
		return nil
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	// 确定起始索引
	var startIdx int
	if b.count < b.capacity {
		startIdx = 0
	} else {
		startIdx = b.head
	}

	// 收集 seq > lastSeq 的事件
	result := make([]AdminEvent, 0)
	for i := 0; i < b.count; i++ {
		event := b.ring[(startIdx+i)%b.capacity]
		if event.Seq > lastSeq {
			result = append(result, event)
		}
	}

	return result
}

// Filtered 返回满足过滤条件的事件列表。
func (b *EventBuffer) Filtered(filter EventFilter) []AdminEvent {
	if b == nil {
		return []AdminEvent{}
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return []AdminEvent{}
	}

	// 确定起始索引
	var startIdx int
	if b.count < b.capacity {
		startIdx = 0
	} else {
		startIdx = b.head
	}

	// 收集并过滤事件
	result := make([]AdminEvent, 0, b.count)
	for i := 0; i < b.count; i++ {
		event := b.ring[(startIdx+i)%b.capacity]

		// 级别过滤
		if filter.Level != "" && event.Level != filter.Level {
			continue
		}
		// 类别过滤
		if filter.Category != "" && event.Category != filter.Category {
			continue
		}
		// 时间过滤
		if filter.Since != nil && event.At.Before(*filter.Since) {
			continue
		}

		result = append(result, event)
	}

	// 应用 limit（返回最新的 N 条）
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[len(result)-filter.Limit:]
	}

	return result
}
