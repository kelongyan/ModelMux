package proxy

import (
	"sync"
	"time"

	"github.com/kelongyan/ModelMux/logx"
)

const (
	defaultProviderCircuitFailureThreshold      = 3
	defaultProviderCircuitOpenCooling           = 5 * time.Second
	defaultProviderCircuitMaxOpenCooling        = 60 * time.Second
	defaultProviderCircuitHalfOpenMax           = 1
	defaultProviderCircuitRejectedEventInterval = 30 * time.Second
)

type providerCircuitState int

const (
	providerCircuitStateClosed providerCircuitState = iota
	providerCircuitStateOpen
	providerCircuitStateHalfOpen
)

func (s providerCircuitState) String() string {
	switch s {
	case providerCircuitStateOpen:
		return "open"
	case providerCircuitStateHalfOpen:
		return "half_open"
	default:
		return "closed"
	}
}

type providerCircuitOptions struct {
	failureThreshold int
	openCooling      time.Duration
	maxOpenCooling   time.Duration
	halfOpenMax      int
	now              func() time.Time
}

type providerCircuit struct {
	mu sync.Mutex

	failureThreshold int
	openCooling      time.Duration
	maxOpenCooling   time.Duration
	halfOpenMax      int
	now              func() time.Time

	state                  providerCircuitState
	consecutiveFailures    int
	openUntil              time.Time
	currentCooling         time.Duration
	halfOpenInFlight       int
	rejectedCount          int
	lastRejectedEventAt    time.Time
	lastRejectedEventCount int
}

type providerCircuitDecision struct {
	allowed bool
	event   *providerCircuitEvent
}

type providerCircuitEvent struct {
	name                string
	state               string
	openUntil           time.Time
	consecutiveFailures int
	rejectedCount       int
	rejectedDelta       int
}

type providerCircuitSnapshot struct {
	state               string
	consecutiveFailures int
	openUntil           time.Time
	halfOpenInFlight    int
	currentCooling      time.Duration
}

// ProviderCircuitSnapshot is the admin-facing view of the active provider circuit.
type ProviderCircuitSnapshot struct {
	ProviderID            string     `json:"provider_id"`
	State                 string     `json:"state"`
	ConsecutiveFailures   int        `json:"consecutive_failures"`
	OpenUntil             *time.Time `json:"open_until,omitempty"`
	HalfOpenInFlight      int        `json:"half_open_in_flight"`
	CurrentCoolingSeconds int        `json:"current_cooling_seconds"`
}

func newProviderCircuit(options providerCircuitOptions) *providerCircuit {
	if options.failureThreshold <= 0 {
		options.failureThreshold = defaultProviderCircuitFailureThreshold
	}
	if options.openCooling <= 0 {
		options.openCooling = defaultProviderCircuitOpenCooling
	}
	if options.maxOpenCooling <= 0 {
		options.maxOpenCooling = defaultProviderCircuitMaxOpenCooling
	}
	if options.maxOpenCooling < options.openCooling {
		options.maxOpenCooling = options.openCooling
	}
	if options.halfOpenMax <= 0 {
		options.halfOpenMax = defaultProviderCircuitHalfOpenMax
	}
	if options.now == nil {
		options.now = time.Now
	}

	return &providerCircuit{
		failureThreshold: options.failureThreshold,
		openCooling:      options.openCooling,
		maxOpenCooling:   options.maxOpenCooling,
		halfOpenMax:      options.halfOpenMax,
		now:              options.now,
		state:            providerCircuitStateClosed,
	}
}

func (c *providerCircuit) beforeRequest() providerCircuitDecision {
	if c == nil {
		return providerCircuitDecision{allowed: true}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	switch c.state {
	case providerCircuitStateOpen:
		if now.Before(c.openUntil) {
			return providerCircuitDecision{
				allowed: false,
				event:   c.rejectedEventLocked(now),
			}
		}
		c.state = providerCircuitStateHalfOpen
		c.halfOpenInFlight = 1
		return providerCircuitDecision{
			allowed: true,
			event:   c.eventLocked(logx.EventProviderCircuitHalfOpen),
		}
	case providerCircuitStateHalfOpen:
		if c.halfOpenInFlight >= c.halfOpenMax {
			return providerCircuitDecision{
				allowed: false,
				event:   c.rejectedEventLocked(now),
			}
		}
		c.halfOpenInFlight++
		return providerCircuitDecision{allowed: true}
	default:
		return providerCircuitDecision{allowed: true}
	}
}

func (c *providerCircuit) recordProviderFailure() *providerCircuitEvent {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFailures++
	switch c.state {
	case providerCircuitStateHalfOpen:
		c.decrementHalfOpenInFlightLocked()
		return c.openLocked()
	case providerCircuitStateClosed:
		if c.consecutiveFailures >= c.failureThreshold {
			return c.openLocked()
		}
	}
	return nil
}

func (c *providerCircuit) recordSuccess() *providerCircuitEvent {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case providerCircuitStateHalfOpen:
		c.decrementHalfOpenInFlightLocked()
		c.state = providerCircuitStateClosed
		c.consecutiveFailures = 0
		c.openUntil = time.Time{}
		c.currentCooling = 0
		c.resetRejectedEventsLocked()
		return c.eventLocked(logx.EventProviderCircuitClosed)
	case providerCircuitStateClosed:
		c.consecutiveFailures = 0
		c.currentCooling = 0
	}
	return nil
}

func (c *providerCircuit) recordNeutral() {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == providerCircuitStateHalfOpen {
		c.decrementHalfOpenInFlightLocked()
	}
}

func (c *providerCircuit) snapshot() providerCircuitSnapshot {
	if c == nil {
		return providerCircuitSnapshot{state: providerCircuitStateClosed.String()}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return providerCircuitSnapshot{
		state:               c.state.String(),
		consecutiveFailures: c.consecutiveFailures,
		openUntil:           c.openUntil,
		halfOpenInFlight:    c.halfOpenInFlight,
		currentCooling:      c.currentCooling,
	}
}

func (c *providerCircuit) openLocked() *providerCircuitEvent {
	cooling := c.openCooling
	if c.currentCooling > 0 {
		cooling = c.currentCooling * 2
		if cooling > c.maxOpenCooling {
			cooling = c.maxOpenCooling
		}
	}
	c.currentCooling = cooling
	c.state = providerCircuitStateOpen
	c.openUntil = c.now().Add(cooling)
	c.halfOpenInFlight = 0
	c.resetRejectedEventsLocked()
	return c.eventLocked(logx.EventProviderCircuitOpened)
}

func (c *providerCircuit) decrementHalfOpenInFlightLocked() {
	if c.halfOpenInFlight > 0 {
		c.halfOpenInFlight--
	}
}

func (c *providerCircuit) eventLocked(name string) *providerCircuitEvent {
	return &providerCircuitEvent{
		name:                name,
		state:               c.state.String(),
		openUntil:           c.openUntil,
		consecutiveFailures: c.consecutiveFailures,
	}
}

func (c *providerCircuit) rejectedEventLocked(now time.Time) *providerCircuitEvent {
	c.rejectedCount++
	if !c.lastRejectedEventAt.IsZero() && now.Sub(c.lastRejectedEventAt) < defaultProviderCircuitRejectedEventInterval {
		return nil
	}
	event := c.eventLocked(logx.EventProviderCircuitRejected)
	event.rejectedCount = c.rejectedCount
	event.rejectedDelta = c.rejectedCount - c.lastRejectedEventCount
	c.lastRejectedEventAt = now
	c.lastRejectedEventCount = c.rejectedCount
	return event
}

func (c *providerCircuit) resetRejectedEventsLocked() {
	c.rejectedCount = 0
	c.lastRejectedEventAt = time.Time{}
	c.lastRejectedEventCount = 0
}

func exportProviderCircuitSnapshot(providerID string, snapshot providerCircuitSnapshot) ProviderCircuitSnapshot {
	out := ProviderCircuitSnapshot{
		ProviderID:            providerID,
		State:                 snapshot.state,
		ConsecutiveFailures:   snapshot.consecutiveFailures,
		HalfOpenInFlight:      snapshot.halfOpenInFlight,
		CurrentCoolingSeconds: int(snapshot.currentCooling.Seconds()),
	}
	if !snapshot.openUntil.IsZero() {
		openUntil := snapshot.openUntil.UTC()
		out.OpenUntil = &openUntil
	}
	return out
}
