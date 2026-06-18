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
type EventBuffer struct {
	mu       sync.RWMutex
	capacity int
	seq      atomic.Int64
	events   []AdminEvent
}

// NewEventBuffer 创建一个固定容量的事件缓冲区。
func NewEventBuffer(capacity int) *EventBuffer {
	if capacity <= 0 {
		capacity = 200
	}
	return &EventBuffer{capacity: capacity}
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

	b.mu.Lock()
	defer b.mu.Unlock()

	entry := AdminEvent{
		Seq:   b.seq.Add(1),
		At:    time.Now(),
		Event: event,
	}

	b.events = append(b.events, entry)
	if len(b.events) > b.capacity {
		start := len(b.events) - b.capacity
		next := make([]AdminEvent, b.capacity)
		copy(next, b.events[start:])
		b.events = next
	}
}

// List 返回最近 limit 条事件，limit<=0 时返回全部。
func (b *EventBuffer) List(limit int) []AdminEvent {
	if b == nil {
		return []AdminEvent{}
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.events) == 0 {
		return []AdminEvent{}
	}
	if limit <= 0 || limit >= len(b.events) {
		return append([]AdminEvent(nil), b.events...)
	}
	start := len(b.events) - limit
	return append([]AdminEvent(nil), b.events[start:]...)
}
