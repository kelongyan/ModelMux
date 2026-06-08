package proxy

import (
	"testing"
	"time"

	"github.com/kelongyan/ModelMux/logx"
)

func TestProviderCircuitOpensRejectsAndHalfOpenCloses(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	circuit := newProviderCircuit(providerCircuitOptions{
		now: func() time.Time { return now },
	})

	if got := circuit.snapshot().state; got != providerCircuitStateClosed.String() {
		t.Fatalf("initial state = %q, want closed", got)
	}
	if decision := circuit.beforeRequest(); !decision.allowed || decision.event != nil {
		t.Fatalf("initial beforeRequest = %+v, want allowed without event", decision)
	}

	for i := 0; i < defaultProviderCircuitFailureThreshold-1; i++ {
		if event := circuit.recordProviderFailure(); event != nil {
			t.Fatalf("recordProviderFailure(%d) event = %+v, want nil before threshold", i, event)
		}
	}
	event := circuit.recordProviderFailure()
	if event == nil || event.name != logx.EventProviderCircuitOpened {
		t.Fatalf("threshold event = %+v, want provider circuit opened", event)
	}
	snapshot := circuit.snapshot()
	if snapshot.state != providerCircuitStateOpen.String() {
		t.Fatalf("state = %q, want open", snapshot.state)
	}
	if !snapshot.openUntil.Equal(now.Add(defaultProviderCircuitOpenCooling)) {
		t.Fatalf("openUntil = %s, want %s", snapshot.openUntil, now.Add(defaultProviderCircuitOpenCooling))
	}

	decision := circuit.beforeRequest()
	if decision.allowed {
		t.Fatal("beforeRequest during open allowed request, want rejected")
	}
	if decision.event == nil || decision.event.name != logx.EventProviderCircuitRejected {
		t.Fatalf("open rejection event = %+v, want provider circuit rejected", decision.event)
	}

	now = now.Add(defaultProviderCircuitOpenCooling + time.Millisecond)
	decision = circuit.beforeRequest()
	if !decision.allowed {
		t.Fatal("half-open probe was rejected")
	}
	if decision.event == nil || decision.event.name != logx.EventProviderCircuitHalfOpen {
		t.Fatalf("half-open event = %+v, want provider circuit half-open", decision.event)
	}
	if got := circuit.snapshot().halfOpenInFlight; got != 1 {
		t.Fatalf("halfOpenInFlight = %d, want 1", got)
	}

	event = circuit.recordSuccess()
	if event == nil || event.name != logx.EventProviderCircuitClosed {
		t.Fatalf("success event = %+v, want provider circuit closed", event)
	}
	snapshot = circuit.snapshot()
	if snapshot.state != providerCircuitStateClosed.String() {
		t.Fatalf("state = %q, want closed after successful probe", snapshot.state)
	}
	if snapshot.consecutiveFailures != 0 {
		t.Fatalf("consecutiveFailures = %d, want 0", snapshot.consecutiveFailures)
	}
}

func TestProviderCircuitLimitsHalfOpenProbeConcurrency(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	circuit := newProviderCircuit(providerCircuitOptions{
		now: func() time.Time { return now },
	})
	openProviderCircuit(t, circuit)

	now = now.Add(defaultProviderCircuitOpenCooling + time.Millisecond)
	first := circuit.beforeRequest()
	if !first.allowed {
		t.Fatal("first half-open probe was rejected")
	}
	second := circuit.beforeRequest()
	if second.allowed {
		t.Fatal("second half-open probe was allowed, want rejected")
	}
	if second.event == nil || second.event.name != logx.EventProviderCircuitRejected {
		t.Fatalf("second half-open event = %+v, want provider circuit rejected", second.event)
	}
}

func TestProviderCircuitThrottlesRejectedEvents(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	circuit := newProviderCircuit(providerCircuitOptions{
		openCooling:    2 * defaultProviderCircuitRejectedEventInterval,
		maxOpenCooling: 2 * defaultProviderCircuitRejectedEventInterval,
		now:            func() time.Time { return now },
	})
	openProviderCircuit(t, circuit)

	first := circuit.beforeRequest()
	if first.allowed {
		t.Fatal("first open request was allowed, want rejected")
	}
	if first.event == nil || first.event.name != logx.EventProviderCircuitRejected {
		t.Fatalf("first event = %+v, want provider circuit rejected", first.event)
	}
	if first.event.rejectedCount != 1 || first.event.rejectedDelta != 1 {
		t.Fatalf("first rejected count/delta = %d/%d, want 1/1", first.event.rejectedCount, first.event.rejectedDelta)
	}

	second := circuit.beforeRequest()
	if second.allowed {
		t.Fatal("second open request was allowed, want rejected")
	}
	if second.event != nil {
		t.Fatalf("second event = %+v, want nil inside throttle window", second.event)
	}

	now = now.Add(defaultProviderCircuitRejectedEventInterval + time.Millisecond)
	third := circuit.beforeRequest()
	if third.allowed {
		t.Fatal("third open request was allowed, want rejected")
	}
	if third.event == nil || third.event.name != logx.EventProviderCircuitRejected {
		t.Fatalf("third event = %+v, want provider circuit rejected after throttle window", third.event)
	}
	if third.event.rejectedCount != 3 || third.event.rejectedDelta != 2 {
		t.Fatalf("third rejected count/delta = %d/%d, want 3/2", third.event.rejectedCount, third.event.rejectedDelta)
	}
}

func TestProviderCircuitHalfOpenFailureReopensWithBackoff(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	circuit := newProviderCircuit(providerCircuitOptions{
		now: func() time.Time { return now },
	})
	openProviderCircuit(t, circuit)

	now = now.Add(defaultProviderCircuitOpenCooling + time.Millisecond)
	if decision := circuit.beforeRequest(); !decision.allowed {
		t.Fatal("half-open probe was rejected")
	}
	event := circuit.recordProviderFailure()
	if event == nil || event.name != logx.EventProviderCircuitOpened {
		t.Fatalf("half-open failure event = %+v, want provider circuit opened", event)
	}
	if got, want := circuit.snapshot().openUntil, now.Add(2*defaultProviderCircuitOpenCooling); !got.Equal(want) {
		t.Fatalf("openUntil = %s, want %s", got, want)
	}
}

func TestProviderCircuitNeutralOutcomeReleasesHalfOpenProbe(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	circuit := newProviderCircuit(providerCircuitOptions{
		now: func() time.Time { return now },
	})
	openProviderCircuit(t, circuit)

	now = now.Add(defaultProviderCircuitOpenCooling + time.Millisecond)
	if decision := circuit.beforeRequest(); !decision.allowed {
		t.Fatal("first half-open probe was rejected")
	}
	circuit.recordNeutral()
	if got := circuit.snapshot().halfOpenInFlight; got != 0 {
		t.Fatalf("halfOpenInFlight = %d, want 0 after neutral outcome", got)
	}
	if decision := circuit.beforeRequest(); !decision.allowed {
		t.Fatal("second half-open probe was rejected after neutral outcome")
	}
}

func openProviderCircuit(t *testing.T, circuit *providerCircuit) {
	t.Helper()
	for i := 0; i < defaultProviderCircuitFailureThreshold; i++ {
		circuit.recordProviderFailure()
	}
	if got := circuit.snapshot().state; got != providerCircuitStateOpen.String() {
		t.Fatalf("state = %q, want open", got)
	}
}
