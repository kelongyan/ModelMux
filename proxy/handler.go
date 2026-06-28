package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
	"github.com/kelongyan/ModelMux/state"
	"github.com/kelongyan/ModelMux/stats"
)

var (
	errClientCanceled            = errors.New("client canceled request")
	errStreamFailed              = errors.New("stream failed")
	errRequestBodyTooLarge       = errors.New("request body too large")
	errKeyQuotaExhausted         = errors.New("key quota exhausted")
	errRetryableUpstream         = errors.New("retryable upstream failure")
	errStreamIdleTimeout         = errors.New("stream idle timeout exceeded")
	errStreamMaxDurationExceeded = errors.New("stream max duration exceeded")
	requestSeq                   atomic.Uint64
)

const (
	quotaErrorInspectBytes        int64 = 64 * 1024
	responseUsageInspectBytes     int64 = 256 * 1024
	responseUsageInspectHeadBytes int64 = 64 * 1024
	upstreamErrorExcerptBytes     int   = 2048
	upstreamRetryDrainBytes       int64 = 64 * 1024
	upstreamMaxIdleConns          int   = 256
	upstreamMaxIdleConnsPerHost   int   = 64
	statsDroppedEventInterval           = 30 * time.Second
	connectionCoolingBase               = 2 * time.Second
	defaultStreamKeepAliveComment       = ": modelmux keepalive\n\n"
	maxRetryAfter                       = 5 * time.Minute
)

var streamBufPool = sync.Pool{New: func() any {
	buf := make([]byte, 4096)
	return &buf
}}

var toolFieldNames = []string{"tools", "tool_choice", "parallel_tool_calls", "functions", "function_call"}

type Handler struct {
	pools                         *pool.ProviderPools
	runtime                       atomic.Pointer[runtimeConfig]
	stateChangeHook               atomic.Value
	statsRecorder                 atomic.Value
	eventRecorder                 atomic.Value
	lastStatsDroppedReported      atomic.Uint64
	lastStatsDroppedEventUnixNano atomic.Int64
}

type stateChangeHook func(immediate bool)
type statsRecorder interface {
	Append(stats.CallRecord) error
}

type statsHealthRecorder interface {
	DroppedRecords() uint64
	QueueDepth() int
	QueueCapacity() int
}

type eventRecorder interface {
	AddEvent(logx.Event)
}

type runtimeConfig struct {
	providerID              string
	targetURL               *url.URL
	keyPool                 *pool.Pool
	client                  *http.Client
	transport               *http.Transport
	circuit                 *providerCircuit
	requestTimeout          time.Duration
	stripTools              bool
	coolingSeconds          int
	maxRetries              int
	maxTransientRetries     int
	transientCoolingSeconds int
	waitForKeyTimeout       time.Duration
	streamKeepAlive         time.Duration
	streamIdleTimeout       time.Duration
	streamMaxDuration       time.Duration
	maxBodyBytes            int64
}

type retryScope int
type streamFailureSide string

const (
	retryScopeNone retryScope = iota
	retryScopeKey
	retryScopeConnection
	retryScopeProvider
)

const (
	streamFailureSideUpstreamRead   streamFailureSide = "upstream_read"
	streamFailureSideClientWrite    streamFailureSide = "client_write"
	streamFailureSideClientCanceled streamFailureSide = "client_canceled"
)

func (s retryScope) String() string {
	switch s {
	case retryScopeKey:
		return "key"
	case retryScopeConnection:
		return "connection"
	case retryScopeProvider:
		return "provider"
	default:
		return "none"
	}
}

type streamFailureError struct {
	side streamFailureSide
	err  error
}

func (e *streamFailureError) Error() string {
	if e == nil || e.err == nil {
		return "stream failed"
	}
	return e.err.Error()
}

func (e *streamFailureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func streamFailureDetails(err error) (streamFailureSide, error) {
	var streamErr *streamFailureError
	if errors.As(err, &streamErr) {
		return streamErr.side, streamErr.err
	}
	return "", err
}

func streamFailureMessage(side streamFailureSide, err error) string {
	if err == nil {
		return "stream failed"
	}
	message := logx.RedactSensitiveText(err.Error())
	switch side {
	case streamFailureSideUpstreamRead:
		return "stream upstream read failed: " + message
	case streamFailureSideClientWrite:
		return "stream client write failed: " + message
	case streamFailureSideClientCanceled:
		return "stream client canceled: " + message
	default:
		return "stream failed: " + message
	}
}

func streamFailureLogLevel(side streamFailureSide) slog.Level {
	if side == streamFailureSideClientCanceled {
		return slog.LevelInfo
	}
	return slog.LevelWarn
}

func streamFailureLogMessage(side streamFailureSide) string {
	if side == streamFailureSideClientCanceled {
		return "stream canceled by client"
	}
	return "stream failed"
}

// NewHandler 创建代理处理器，并初始化带超时的上游 HTTP client。
func NewHandler(pools *pool.ProviderPools, cfg *config.Config) (*Handler, error) {
	h := &Handler{pools: pools}
	if err := h.UpdateConfig(cfg); err != nil {
		return nil, err
	}
	return h, nil
}

// SetStateChangeHook 设置 key 池状态变化回调，用于触发状态持久化。
func (h *Handler) SetStateChangeHook(fn func(immediate bool)) {
	if fn == nil {
		fn = func(bool) {}
	}
	h.stateChangeHook.Store(stateChangeHook(fn))
}

// SetStatsRecorder 设置调用统计记录器；nil 表示不记录调用统计。
func (h *Handler) SetStatsRecorder(recorder statsRecorder) {
	if recorder == nil {
		recorder = noopStatsRecorder{}
	}
	h.statsRecorder.Store(recorder)
}

// SetEventRecorder sets the shared diagnostic event sink used by the admin event feed.
func (h *Handler) SetEventRecorder(recorder eventRecorder) {
	if recorder == nil {
		recorder = noopEventRecorder{}
	}
	h.eventRecorder.Store(recorder)
}

// notifyStateChanged 通知外部保存 key 池状态；immediate 表示需要尽快落盘。
func (h *Handler) notifyStateChanged(immediate bool) {
	hook, _ := h.stateChangeHook.Load().(stateChangeHook)
	if hook != nil {
		hook(immediate)
	}
}

func (h *Handler) emitEvent(ctx context.Context, event logx.Event) {
	switch event.Level {
	case "debug":
		slog.DebugContext(ctx, event.Message, event.Attrs()...)
	case "warn":
		slog.WarnContext(ctx, event.Message, event.Attrs()...)
	case "error":
		slog.ErrorContext(ctx, event.Message, event.Attrs()...)
	default:
		slog.InfoContext(ctx, event.Message, event.Attrs()...)
	}

	recorder, _ := h.eventRecorder.Load().(eventRecorder)
	if recorder != nil {
		recorder.AddEvent(event)
	}
}

func (h *Handler) recordProviderCircuitSuccess(ctx context.Context, rt *runtimeConfig, r *http.Request, requestID, clientRequestID string) {
	if rt == nil || rt.circuit == nil {
		return
	}
	if event := rt.circuit.recordSuccess(); event != nil {
		h.emitProviderCircuitEvent(ctx, event, r, rt, requestID, clientRequestID)
	}
}

func (h *Handler) emitProviderCircuitEvent(ctx context.Context, circuitEvent *providerCircuitEvent, r *http.Request, rt *runtimeConfig, requestID, clientRequestID string) {
	if circuitEvent == nil {
		return
	}
	data := map[string]any{
		"state":                circuitEvent.state,
		"consecutive_failures": circuitEvent.consecutiveFailures,
	}
	if !circuitEvent.openUntil.IsZero() {
		data["open_until"] = circuitEvent.openUntil.UTC().Format(time.RFC3339Nano)
	}
	if circuitEvent.rejectedCount > 0 {
		data["rejected_count"] = circuitEvent.rejectedCount
		data["rejected_delta"] = circuitEvent.rejectedDelta
	}

	event := logx.Event{
		Level:           providerCircuitEventLevel(circuitEvent.name),
		Category:        logx.CategoryProvider,
		Event:           circuitEvent.name,
		Message:         providerCircuitEventMessage(circuitEvent.name),
		RequestID:       requestID,
		ClientRequestID: clientRequestID,
		Data:            data,
	}
	if rt != nil {
		event.ProviderID = rt.providerID
	}
	if r != nil {
		event.Method = r.Method
		event.Path = r.URL.Path
	}
	if circuitEvent.name == logx.EventProviderCircuitRejected {
		event.Status = http.StatusServiceUnavailable
	}
	h.emitEvent(ctx, event)
}

func providerCircuitEventLevel(name string) string {
	switch name {
	case logx.EventProviderCircuitOpened:
		return "warn"
	case logx.EventProviderCircuitRejected:
		return "warn"
	default:
		return "info"
	}
}

func providerCircuitEventMessage(name string) string {
	switch name {
	case logx.EventProviderCircuitOpened:
		return "provider circuit opened"
	case logx.EventProviderCircuitHalfOpen:
		return "provider circuit half-open"
	case logx.EventProviderCircuitClosed:
		return "provider circuit closed"
	case logx.EventProviderCircuitRejected:
		return "provider circuit rejected request"
	default:
		return "provider circuit event"
	}
}

func (h *Handler) recordCallStats(ctx context.Context, record stats.CallRecord) {
	recorder, _ := h.statsRecorder.Load().(statsRecorder)
	if recorder == nil {
		return
	}
	if err := recorder.Append(record); err != nil {
		slog.Warn("stats record failed", logx.Fields(logx.CategoryProxy, "proxy.stats_record_failed",
			"provider_id", record.ProviderID,
			"model", record.Model,
			"err", err,
		)...)
	}
	h.emitStatsDroppedEventIfNeeded(ctx, recorder, record)
}

func (h *Handler) emitStatsDroppedEventIfNeeded(ctx context.Context, recorder statsRecorder, record stats.CallRecord) {
	health, ok := recorder.(statsHealthRecorder)
	if !ok || health == nil {
		return
	}

	dropped := health.DroppedRecords()
	previous := h.lastStatsDroppedReported.Load()
	if dropped <= previous {
		return
	}

	now := time.Now()
	lastEventAt := h.lastStatsDroppedEventUnixNano.Load()
	if lastEventAt > 0 && now.Sub(time.Unix(0, lastEventAt)) < statsDroppedEventInterval {
		return
	}
	if !h.lastStatsDroppedEventUnixNano.CompareAndSwap(lastEventAt, now.UnixNano()) {
		return
	}

	previous = h.lastStatsDroppedReported.Swap(dropped)
	if dropped <= previous {
		return
	}
	h.emitEvent(ctx, logx.Event{
		Level:      "warn",
		Category:   logx.CategoryStats,
		Event:      logx.EventStatsQueueDropped,
		Message:    "stats queue dropped records",
		ProviderID: record.ProviderID,
		Method:     record.Method,
		Path:       record.Endpoint,
		Model:      record.Model,
		Data: map[string]any{
			"dropped_records": dropped,
			"dropped_delta":   dropped - previous,
			"queue_depth":     health.QueueDepth(),
			"queue_capacity":  health.QueueCapacity(),
		},
	})
}

// UpdateConfig 原子替换代理运行时配置，新请求会使用新快照，已有请求继续使用旧快照。
func (h *Handler) UpdateConfig(cfg *config.Config) error {
	provider, ok := cfg.ActiveProviderConfig()
	if !ok {
		return fmt.Errorf("active provider %q not found", cfg.ActiveProvider)
	}
	keyPool, err := h.pools.Get(provider.ID)
	if err != nil {
		return err
	}
	next, err := newRuntimeConfig(cfg, provider, keyPool)
	if err != nil {
		return err
	}

	old := h.runtime.Load()
	h.runtime.Store(next)
	if old != nil && old.transport != nil {
		old.transport.CloseIdleConnections()
	}
	return nil
}

// ValidateConfig 校验代理运行时需要的 active provider 与上游 URL，供热重载提交前预检查。
func ValidateConfig(cfg *config.Config) error {
	provider, ok := cfg.ActiveProviderConfig()
	if !ok {
		return fmt.Errorf("active provider %q not found", cfg.ActiveProvider)
	}
	if _, err := parseTargetURL(provider.TargetURL); err != nil {
		return err
	}
	return nil
}

// snapshot 读取当前运行时配置快照。
func (h *Handler) snapshot() *runtimeConfig {
	rt := h.runtime.Load()
	return rt
}

// ProviderCircuitSnapshot returns the current active provider circuit state.
func (h *Handler) ProviderCircuitSnapshot() ProviderCircuitSnapshot {
	rt := h.snapshot()
	if rt == nil || rt.circuit == nil {
		return ProviderCircuitSnapshot{State: providerCircuitStateClosed.String()}
	}
	return exportProviderCircuitSnapshot(rt.providerID, rt.circuit.snapshot())
}

// newRuntimeConfig 从配置构造不可变运行时快照。
func newRuntimeConfig(cfg *config.Config, provider config.ProviderConfig, keyPool *pool.Pool) (*runtimeConfig, error) {
	target, err := parseTargetURL(provider.TargetURL)
	if err != nil {
		return nil, err
	}

	requestTimeoutSeconds := effectiveInt(cfg.RequestTimeoutSeconds, config.DefaultRequestTimeoutSeconds)
	connectTimeoutSeconds := effectiveInt(cfg.ConnectTimeoutSeconds, config.DefaultConnectTimeoutSeconds)
	responseHeaderTimeoutSeconds := effectiveInt(cfg.ResponseHeaderTimeoutSeconds, config.DefaultResponseHeaderTimeoutSeconds)
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(connectTimeoutSeconds) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        upstreamMaxIdleConns,
		MaxIdleConnsPerHost: upstreamMaxIdleConnsPerHost,
		ForceAttemptHTTP2:   true,

		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   time.Duration(connectTimeoutSeconds) * time.Second,
		ResponseHeaderTimeout: time.Duration(responseHeaderTimeoutSeconds) * time.Second,
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: transport,
	}

	return &runtimeConfig{
		providerID: provider.ID,
		targetURL:  target,
		keyPool:    keyPool,
		client:     client,
		transport:  transport,
		circuit: newProviderCircuit(providerCircuitOptions{
			failureThreshold: effectiveInt(cfg.ProviderCircuitFailureThreshold, config.DefaultProviderCircuitFailureThreshold),
			openCooling:      time.Duration(effectiveInt(cfg.ProviderCircuitOpenSeconds, config.DefaultProviderCircuitOpenSeconds)) * time.Second,
			maxOpenCooling:   time.Duration(effectiveInt(cfg.ProviderCircuitMaxOpenSeconds, config.DefaultProviderCircuitMaxOpenSeconds)) * time.Second,
			halfOpenMax:      effectiveInt(cfg.ProviderCircuitHalfOpenMax, config.DefaultProviderCircuitHalfOpenMax),
		}),
		requestTimeout:          time.Duration(requestTimeoutSeconds) * time.Second,
		stripTools:              provider.StripTools,
		coolingSeconds:          effectiveInt(cfg.CoolingSeconds, config.DefaultCoolingSeconds),
		maxRetries:              effectiveInt(cfg.MaxRetries, config.DefaultMaxRetries),
		maxTransientRetries:     effectiveInt(cfg.MaxTransientRetries, config.DefaultMaxTransientRetries),
		transientCoolingSeconds: effectiveInt(cfg.TransientCoolingSeconds, config.DefaultTransientCoolingSeconds),
		waitForKeyTimeout:       time.Duration(effectiveInt(cfg.WaitForKeyTimeoutMS, config.DefaultWaitForKeyTimeoutMS)) * time.Millisecond,
		streamKeepAlive:         time.Duration(effectiveInt(cfg.StreamKeepAliveSeconds, config.DefaultStreamKeepAliveSeconds)) * time.Second,
		streamIdleTimeout:       time.Duration(effectiveInt(cfg.StreamIdleTimeoutSeconds, config.DefaultStreamIdleTimeoutSeconds)) * time.Second,
		streamMaxDuration:       time.Duration(effectiveInt(cfg.StreamMaxDurationSeconds, config.DefaultStreamMaxDurationSeconds)) * time.Second,
		maxBodyBytes:            effectiveInt64(cfg.MaxBodyBytes, config.DefaultMaxBodyBytes),
	}, nil
}

// parseTargetURL 解析并校验上游 URL 必须带 scheme 和 host。
func parseTargetURL(rawURL string) (*url.URL, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target_url: %w", err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("invalid target_url: absolute URL with scheme and host is required")
	}
	return target, nil
}

// effectiveInt 在测试或手写配置未填时补齐运行时默认整数值。
func effectiveInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// effectiveInt64 在测试或手写配置未填时补齐运行时默认整数值。
func effectiveInt64(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

// ServeHTTP 读取请求体后执行带 key 轮换的上游转发。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := nextRequestID(start)
	clientRequestID := sanitizeClientRequestID(r.Header.Get("X-Request-ID"))
	rt := h.snapshot()
	if rt == nil {
		h.emitEvent(r.Context(), logx.Event{
			Level:           "error",
			Category:        logx.CategoryProxy,
			Event:           logx.EventRuntimeNotReady,
			Message:         "proxy runtime config is not ready",
			RequestID:       requestID,
			ClientRequestID: clientRequestID,
			Method:          r.Method,
			Path:            r.URL.Path,
		})
		writeProxyError(w, http.StatusServiceUnavailable, "proxy runtime config is not ready")
		return
	}
	slog.Debug("proxy request start", logx.Fields(logx.CategoryProxy, logx.EventRequestStart,
		"method", r.Method,
		"path", r.URL.Path,
		"provider_id", rt.providerID,
	)...)

	body, err := readRequestBody(r, rt.maxBodyBytes)
	if err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			slog.Warn("request body too large", logx.Fields(logx.CategoryProxy, logx.EventBodyTooLarge,
				"path", r.URL.Path,
				"max_body_bytes", rt.maxBodyBytes,
			)...)
			writeProxyError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		slog.Error("failed to read request body", logx.Fields(logx.CategoryProxy, logx.EventBodyReadFailed,
			"path", r.URL.Path,
			"err", err,
		)...)
		writeProxyError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	requestMeta := extractRequestMeta(body)
	requestModel := requestMeta.model
	requestStream := requestMeta.stream
	toolsPresent := requestMeta.toolsPresent
	toolsStripped := rt.stripTools && toolsPresent

	diagCtx := diagnosticContext{
		requestID:       requestID,
		clientRequestID: clientRequestID,
		request:         r,
		runtime:         rt,
		model:           requestModel,
		stream:          requestStream,
		toolsPresent:    toolsPresent,
		toolsStripped:   toolsStripped,
	}

	callRecord := stats.CallRecord{
		At:          start.UTC(),
		ProviderID:  rt.providerID,
		Model:       requestModel,
		Stream:      requestStream,
		Endpoint:    r.URL.Path,
		Method:      r.Method,
		UsageSource: stats.UsageSourceUnknown,
	}
	defer func() {
		if callRecord.Status == 0 {
			return
		}
		callRecord.LatencyMs = time.Since(start).Milliseconds()
		h.recordCallStats(r.Context(), callRecord)
	}()

	decision := rt.circuit.beforeRequest()
	if decision.event != nil {
		h.emitProviderCircuitEvent(r.Context(), decision.event, r, rt, requestID, clientRequestID)
	}
	if !decision.allowed {
		callRecord.Status = http.StatusServiceUnavailable
		callRecord.Error = "active provider is temporarily unavailable"
		writeProxyError(w, http.StatusServiceUnavailable, "active provider is temporarily unavailable")
		return
	}
	circuitOutcomeRecorded := false
	defer func() {
		if !circuitOutcomeRecorded {
			rt.circuit.recordNeutral()
		}
	}()

	var lastStatus int
	var lastErr error
	lastScope := retryScopeNone
	transientFailures := 0
	waitBudget := rt.waitForKeyTimeout
	waitRetries := rt.maxRetries

	for attempt := 0; attempt <= rt.maxRetries; attempt++ {
		key, err := rt.keyPool.Next()
		if err != nil {
			if errors.Is(err, pool.ErrNoAvailableKey) && waitRetries > 0 {
				if waited := h.waitForCoolingKey(r.Context(), rt, waitBudget); waited > 0 {
					waitBudget -= waited
					waitRetries--
					attempt--
					continue
				}
			}
			slog.Error("no available keys", logx.Fields(logx.CategoryProxy, logx.EventNoAvailableKey,
				"path", r.URL.Path,
				"provider_id", rt.providerID,
				"wait_budget_ms", waitBudget.Milliseconds(),
			)...)
			callRecord.Status = http.StatusServiceUnavailable
			callRecord.Error = "no available API keys"
			h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildEvent(
				"error", logx.EventProxyRequestFailed, "no available API keys",
				nil, http.StatusServiceUnavailable, time.Since(start).Milliseconds(), attempt, retryScopeNone,
			)))
			writeProxyError(w, http.StatusServiceUnavailable, "no available API keys")
			return
		}

		status, retryAfter, scope, usage, upstreamErrorExcerpt, err := h.forward(w, r, rt, key, body, requestStream, requestMeta)
		key.FinishRequest()
		callRecord.Attempts = attempt + 1
		callRecord.Status = status
		callRecord.KeyID = state.KeyID(key.Value)
		applyUsageToRecord(&callRecord, usage)
		if err == nil {
			key.ResetConnectionFailures()
			circuitOutcomeRecorded = true
			h.recordProviderCircuitSuccess(r.Context(), rt, r, requestID, clientRequestID)
			callRecord.Success = status >= http.StatusOK && status < http.StatusBadRequest
			if !callRecord.Success {
				callRecord.Error = http.StatusText(status)
			}
			h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildEventWithExcerpt(
				levelForStatus(status), eventForStatus(status), messageForStatus(status),
				key, status, time.Since(start).Milliseconds(), attempt+1, retryScopeNone, upstreamErrorExcerpt,
			)))
			h.notifyStateChanged(false)
			return
		}
		if errors.Is(err, errClientCanceled) {
			callRecord.Status = 0
			circuitOutcomeRecorded = true // 客户端取消与服务端健康无关，不影响熔断器
			slog.Info("request canceled", logx.Fields(logx.CategoryProxy, logx.EventClientCanceled,
				"path", r.URL.Path,
				"attempt", attempt+1,
				"provider_id", rt.providerID,
				"key_hint", logx.MaskSecret(key.Value),
			)...)
			return
		}
		if errors.Is(err, errStreamFailed) {
			side, streamErr := streamFailureDetails(err)
			if side == streamFailureSideClientCanceled {
				callRecord.Status = 0
				circuitOutcomeRecorded = true // 客户端取消与服务端健康无关，不影响熔断器
				h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildStreamFailure(
					"info", logx.EventClientCanceled, "request canceled",
					key, status, time.Since(start).Milliseconds(), attempt+1, retryScopeNone, side, streamErr,
				)))
				return
			}
			callRecord.Error = streamFailureMessage(side, streamErr)
			h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildStreamFailure(
				"warn", logx.EventProxyRequestFailed, "stream failed",
				key, status, time.Since(start).Milliseconds(), attempt+1, retryScopeNone, side, streamErr,
			)))
			return
		}

		lastStatus = status
		lastErr = err
		lastScope = scope
		stopRetrying := false

		switch scope {
		case retryScopeKey:
			key.ResetConnectionFailures()
			circuitOutcomeRecorded = true
			h.recordProviderCircuitSuccess(r.Context(), rt, r, requestID, clientRequestID)
			switch status {
			case http.StatusTooManyRequests:
				cooling := rt.keyPool.CoolingDuration(rt.coolingSeconds)
				if retryAfter > 0 {
					cooling = retryAfter
				}
				key.MarkCooling(cooling)
				h.notifyStateChanged(true)
				slog.Warn("key rate-limited, cooling", logx.Fields(logx.CategoryRetry, logx.EventKeyCooling,
					"attempt", attempt+1,
					"provider_id", rt.providerID,
					"key_hint", logx.MaskSecret(key.Value),
					"cooling_s", cooling.Seconds(),
				)...)
			case http.StatusUnauthorized:
				key.MarkInvalidWithReason(pool.InvalidReasonUnauthorized)
				h.notifyStateChanged(true)
				slog.Warn("key invalid, marking dead", logx.Fields(logx.CategoryRetry, logx.EventKeyInvalid,
					"attempt", attempt+1,
					"provider_id", rt.providerID,
					"key_hint", logx.MaskSecret(key.Value),
				)...)
			case http.StatusForbidden:
				if !errors.Is(err, errKeyQuotaExhausted) {
					slog.Error("upstream error", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnexpected,
						"status", status,
						"provider_id", rt.providerID,
						"err", err,
					)...)
					callRecord.Error = err.Error()
					return
				}
				key.MarkInvalidWithReason(pool.InvalidReasonQuotaExhausted)
				h.notifyStateChanged(true)
				slog.Warn("key quota exhausted, marking dead", logx.Fields(logx.CategoryRetry, logx.EventKeyQuotaExhausted,
					"attempt", attempt+1,
					"provider_id", rt.providerID,
					"key_hint", logx.MaskSecret(key.Value),
				)...)
			default:
				slog.Error("upstream error", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnexpected,
					"status", status,
					"provider_id", rt.providerID,
					"err", err,
				)...)
				callRecord.Error = err.Error()
				h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildEventWithExcerpt(
					levelForStatus(status), logx.EventProxyRequestFailed, messageForStatus(status),
					key, status, time.Since(start).Milliseconds(), attempt+1, scope, upstreamErrorExcerpt,
				)))
				return
			}
		case retryScopeConnection:
			transientFailures++
			cooling := connectionCoolingDuration(rt.keyPool.CoolingDuration(rt.transientCoolingSeconds), key.ConnectionFailureCount())
			if retryAfter > 0 {
				cooling = retryAfter
			}
			key.MarkConnectionCooling(cooling)
			h.notifyStateChanged(true)
			slog.Warn("key transient failure, cooling", logx.Fields(logx.CategoryRetry, logx.EventKeyTransientFailure,
				"attempt", attempt+1,
				"provider_id", rt.providerID,
				"key_hint", logx.MaskSecret(key.Value),
				"status", status,
				"scope", scope.String(),
				"cooling_s", cooling.Seconds(),
				"transient_failures", transientFailures,
				"max_transient_retries", rt.maxTransientRetries,
				"err", err,
			)...)
			stopRetrying = transientFailures > rt.maxTransientRetries
		case retryScopeProvider:
			key.ResetConnectionFailures()
			transientFailures++
			circuitOutcomeRecorded = true
			if event := rt.circuit.recordProviderFailure(); event != nil {
				h.emitProviderCircuitEvent(r.Context(), event, r, rt, requestID, clientRequestID)
				stopRetrying = true
			}
			slog.Warn("provider transient failure", logx.Fields(logx.CategoryRetry, logx.EventProviderTransient,
				"attempt", attempt+1,
				"provider_id", rt.providerID,
				"key_hint", logx.MaskSecret(key.Value),
				"status", status,
				"scope", scope.String(),
				"transient_failures", transientFailures,
				"max_transient_retries", rt.maxTransientRetries,
				"err", err,
			)...)
			stopRetrying = stopRetrying || transientFailures > rt.maxTransientRetries
		default:
			// 非重试错误已经在 forward 中写出响应，这里只记录分类日志。
			slog.Error("upstream error", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnexpected,
				"status", status,
				"provider_id", rt.providerID,
				"err", err,
			)...)
			callRecord.Error = err.Error()
			h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildEventWithExcerpt(
				levelForStatus(status), logx.EventProxyRequestFailed, messageForStatus(status),
				key, status, time.Since(start).Milliseconds(), attempt+1, scope, upstreamErrorExcerpt,
			)))
			return
		}
		if stopRetrying {
			break
		}
	}

	slog.Error("all retries exhausted", logx.Fields(logx.CategoryRetry, logx.EventRetryExhausted,
		"last_status", lastStatus,
		"provider_id", rt.providerID,
		"retry_scope", lastScope.String(),
		"err", lastErr,
	)...)
	if lastScope == retryScopeProvider || lastScope == retryScopeConnection {
		callRecord.Status = http.StatusServiceUnavailable
		callRecord.Error = "upstream temporarily unavailable after retries"
		h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildEvent(
			"error", logx.EventProxyRequestFailed, "upstream temporarily unavailable after retries",
			nil, http.StatusServiceUnavailable, time.Since(start).Milliseconds(), callRecord.Attempts, lastScope,
		)))
		writeProxyError(w, http.StatusServiceUnavailable, "upstream temporarily unavailable after retries")
		return
	}
	callRecord.Status = http.StatusServiceUnavailable
	callRecord.Error = "all keys exhausted after retries"
	h.emitEvent(r.Context(), requestDiagnosticEvent(diagCtx.buildEvent(
		"error", logx.EventProxyRequestFailed, "all keys exhausted after retries",
		nil, http.StatusServiceUnavailable, time.Since(start).Milliseconds(), callRecord.Attempts, lastScope,
	)))
	writeProxyError(w, http.StatusServiceUnavailable, "all keys exhausted after retries")
}

// waitForCoolingKey 在所有 key 仅因 cooling 暂不可用时短等最近恢复窗口，尽量避免立即向用户暴露 503。
func (h *Handler) waitForCoolingKey(ctx context.Context, rt *runtimeConfig, budget time.Duration) time.Duration {
	if rt == nil || rt.keyPool == nil || budget <= 0 {
		return 0
	}

	waitFor, ok := rt.keyPool.NextAvailableIn(time.Now())
	if !ok || waitFor <= 0 || waitFor > budget {
		return 0
	}

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return 0
	case <-timer.C:
		return waitFor
	}
}

func connectionCoolingDuration(cap time.Duration, priorConnectionFailures int64) time.Duration {
	if cap <= 0 {
		return 0
	}
	if cap <= connectionCoolingBase {
		return cap
	}

	cooling := connectionCoolingBase
	for i := int64(0); i < priorConnectionFailures && cooling < cap; i++ {
		if cooling > cap/2 {
			return cap
		}
		cooling *= 2
	}
	if cooling > cap {
		return cap
	}
	return cooling
}

// forward 使用指定 key 请求上游，并把上游响应流式写回客户端。
// 成功时返回状态码和 nil；429、401 和余额不足类 403 返回可重试错误；其他错误会在本函数内写出响应。
func (h *Handler) forward(w http.ResponseWriter, r *http.Request, rt *runtimeConfig, key *pool.Key, body []byte, requestStream bool, meta requestMeta) (int, time.Duration, retryScope, stats.Usage, string, error) {
	unknownUsage := stats.Usage{Source: stats.UsageSourceUnknown}
	outReq, err := buildRequest(rt, r, key, body, meta)
	if err != nil {
		writeProxyError(w, http.StatusInternalServerError, "failed to build request")
		return http.StatusInternalServerError, 0, retryScopeNone, unknownUsage, "", err
	}
	if rt.requestTimeout > 0 && !requestStream {
		ctx, cancel := context.WithTimeout(outReq.Context(), rt.requestTimeout)
		defer cancel()
		outReq = outReq.WithContext(ctx)
	}

	upstreamStart := time.Now()
	resp, err := rt.client.Do(outReq)
	if err != nil {
		// 客户端主动取消或代理请求超时都不应被视为 key 级故障，
		// 用 outReq.Context() 而非 r.Context() 可以同时覆盖两种情况：
		// 非流式请求的 ctx 是从 r.Context() 派生的 WithTimeout，超时后返回 DeadlineExceeded；
		// 流式请求直接用 r.Context()，客户端断开返回 Canceled。
		if ctxErr := outReq.Context().Err(); ctxErr != nil {
			return 0, 0, retryScopeNone, unknownUsage, "", errClientCanceled
		}
		scope := classifyTransportRetryScope(err)
		slog.Error("upstream unreachable", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnreachable,
			"path", r.URL.Path,
			"provider_id", rt.providerID,
			"key_hint", logx.MaskSecret(key.Value),
			"scope", scope.String(),
			"err", err,
		)...)
		return http.StatusBadGateway, 0, scope, unknownUsage, "", fmt.Errorf("%w: %v", errRetryableUpstream, err)
	}
	// 429/401 需要换 key 重试，因此不能提前向客户端写响应头。
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		_ = drainAndClose(resp.Body, upstreamRetryDrainBytes)
		return resp.StatusCode, retryAfter, retryScopeKey, unknownUsage, "", fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		_ = drainAndClose(resp.Body, upstreamRetryDrainBytes)
		return resp.StatusCode, 0, retryScopeKey, unknownUsage, "", fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	if isRetryableUpstreamStatus(resp.StatusCode) {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		_ = drainAndClose(resp.Body, upstreamRetryDrainBytes)
		return resp.StatusCode, retryAfter, retryScopeProvider, unknownUsage, "", fmt.Errorf("%w: upstream returned %d", errRetryableUpstream, resp.StatusCode)
	}

	responseBody := io.Reader(resp.Body)
	if resp.StatusCode == http.StatusForbidden {
		prefix, replayBody, err := readResponsePrefix(resp.Body, quotaErrorInspectBytes)
		if err != nil {
			_ = resp.Body.Close()
			writeProxyError(w, http.StatusBadGateway, "failed to read upstream error body")
			return http.StatusBadGateway, 0, retryScopeNone, unknownUsage, "", err
		}
		if isQuotaExhaustedBody(prefix) {
			_ = drainAndClose(resp.Body, upstreamRetryDrainBytes)
			return resp.StatusCode, 0, retryScopeKey, unknownUsage, errorExcerpt(prefix), fmt.Errorf("%w: upstream returned %d", errKeyQuotaExhausted, resp.StatusCode)
		}
		responseBody = replayBody
	}
	defer resp.Body.Close()

	key.RecordLatency(time.Since(upstreamStart))

	capturedBody := newCaptureReader(responseBody, responseUsageInspectHeadBytes, responseUsageInspectBytes)
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	streamOpts := streamOptions{contentType: resp.Header.Get("Content-Type")}
	if requestStream || isEventStreamContentType(streamOpts.contentType) {
		streamOpts.keepAlive = rt.streamKeepAlive
		streamOpts.idleTimeout = rt.streamIdleTimeout
		streamOpts.maxDuration = rt.streamMaxDuration
		streamOpts.keepAliveComment = defaultStreamKeepAliveComment
	}
	if err := streamBody(w, capturedBody, func() { _ = resp.Body.Close() }, streamOpts); err != nil {
		side, streamErr := streamFailureDetails(err)
		slog.Log(r.Context(), streamFailureLogLevel(side), streamFailureLogMessage(side), logx.Fields(logx.CategoryStream, logx.EventStreamFailed,
			"path", r.URL.Path,
			"status", resp.StatusCode,
			"provider_id", rt.providerID,
			"key_hint", logx.MaskSecret(key.Value),
			"stream_failure_side", side,
			"err", logx.RedactSensitiveText(streamErr.Error()),
		)...)
		return resp.StatusCode, 0, retryScopeNone, unknownUsage, "", fmt.Errorf("%w: %w", errStreamFailed, err)
	}
	captured := capturedBody.Bytes()
	return resp.StatusCode, 0, retryScopeNone, stats.ExtractUsage(captured), errorExcerptForStatus(resp.StatusCode, captured), nil
}

// readResponsePrefix 读取响应体前缀用于错误分类，并返回可重放完整响应体的 reader。
func readResponsePrefix(body io.Reader, limit int64) ([]byte, io.Reader, error) {
	if body == nil {
		return nil, http.NoBody, nil
	}
	if limit <= 0 {
		return nil, body, nil
	}

	var prefix bytes.Buffer
	if _, err := io.Copy(&prefix, io.LimitReader(body, limit)); err != nil {
		return nil, nil, err
	}
	prefixBytes := prefix.Bytes()
	return prefixBytes, io.MultiReader(bytes.NewReader(prefixBytes), body), nil
}

func drainAndClose(body io.ReadCloser, limit int64) error {
	if body == nil {
		return nil
	}
	var readErr error
	if limit > 0 {
		_, readErr = io.Copy(io.Discard, io.LimitReader(body, limit))
	}
	closeErr := body.Close()
	if readErr != nil {
		return readErr
	}
	return closeErr
}

type noopStatsRecorder struct{}

func (noopStatsRecorder) Append(stats.CallRecord) error {
	return nil
}

type noopEventRecorder struct{}

func (noopEventRecorder) AddEvent(logx.Event) {}

func applyUsageToRecord(record *stats.CallRecord, usage stats.Usage) {
	if record == nil {
		return
	}
	if usage.Source != "" {
		record.UsageSource = usage.Source
	}
	record.PromptTokens = usage.PromptTokens
	record.CompletionTokens = usage.CompletionTokens
	record.TotalTokens = usage.TotalTokens
	// 用上游返回的实际模型 ID 覆盖客户端请求中的模型名，
	// 确保调用统计反映真实使用的模型（例如 gpt-4 → gpt-4-turbo-2024-04-09）。
	if usage.Model != "" {
		record.Model = usage.Model
	}
}

func extractRequestMeta(body []byte) requestMeta {
	if len(bytes.TrimSpace(body)) == 0 {
		return requestMeta{}
	}
	var payloadMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &payloadMap); err != nil {
		return requestMeta{}
	}
	meta := requestMeta{payload: payloadMap}
	if raw, ok := payloadMap["model"]; ok {
		var model string
		if err := json.Unmarshal(raw, &model); err == nil {
			meta.model = strings.TrimSpace(model)
		}
	}
	if raw, ok := payloadMap["stream"]; ok {
		var stream bool
		if err := json.Unmarshal(raw, &stream); err == nil {
			meta.stream = stream
		}
	}
	for _, field := range toolFieldNames {
		if _, ok := payloadMap[field]; ok {
			meta.toolsPresent = true
			break
		}
	}
	return meta
}

type captureReader struct {
	reader    io.Reader
	headLimit int64
	tailLimit int64
	captured  int64
	buffer    bytes.Buffer
	tail      []byte
}

func newCaptureReader(reader io.Reader, headLimit, tailLimit int64) *captureReader {
	if headLimit < 0 {
		headLimit = 0
	}
	if tailLimit < 0 {
		tailLimit = 0
	}
	if tailLimit < headLimit {
		tailLimit = headLimit
	}
	return &captureReader{
		reader:    reader,
		headLimit: headLimit,
		tailLimit: tailLimit,
	}
}

func (r *captureReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.capture(p[:n])
	}
	return n, err
}

func (r *captureReader) capture(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	if r.headLimit > r.captured {
		remaining := r.headLimit - r.captured
		captureN := len(chunk)
		if int64(captureN) > remaining {
			captureN = int(remaining)
		}
		_, _ = r.buffer.Write(chunk[:captureN])
		r.captured += int64(captureN)
		chunk = chunk[captureN:]
	}
	if len(chunk) == 0 {
		return
	}

	tailCapacity := int(r.tailLimit - r.headLimit)
	if tailCapacity <= 0 {
		return
	}
	if len(chunk) >= tailCapacity {
		r.tail = append(r.tail[:0], chunk[len(chunk)-tailCapacity:]...)
		return
	}
	overflow := len(r.tail) + len(chunk) - tailCapacity
	if overflow > 0 {
		copy(r.tail, r.tail[overflow:])
		r.tail = r.tail[:len(r.tail)-overflow]
	}
	r.tail = append(r.tail, chunk...)
}

func (r *captureReader) Bytes() []byte {
	if len(r.tail) == 0 {
		return r.buffer.Bytes()
	}
	out := make([]byte, 0, r.buffer.Len()+len(r.tail))
	out = append(out, r.buffer.Bytes()...)
	out = append(out, r.tail...)
	return out
}

type requestDiagnosticInput struct {
	level                string
	event                string
	message              string
	requestID            string
	clientRequestID      string
	request              *http.Request
	runtime              *runtimeConfig
	key                  *pool.Key
	model                string
	stream               bool
	status               int
	latencyMs            int64
	attempts             int
	retryScope           retryScope
	toolsPresent         bool
	toolsStripped        bool
	upstreamErrorExcerpt string
	streamFailureSide    streamFailureSide
	streamError          error
}

type diagnosticContext struct {
	requestID       string
	clientRequestID string
	request         *http.Request
	runtime         *runtimeConfig
	model           string
	stream          bool
	toolsPresent    bool
	toolsStripped   bool
}

func (dc diagnosticContext) buildEvent(level, event, message string, key *pool.Key, status int, latencyMs int64, attempts int, scope retryScope) requestDiagnosticInput {
	return requestDiagnosticInput{
		level:           level,
		event:           event,
		message:         message,
		requestID:       dc.requestID,
		clientRequestID: dc.clientRequestID,
		request:         dc.request,
		runtime:         dc.runtime,
		key:             key,
		model:           dc.model,
		stream:          dc.stream,
		status:          status,
		latencyMs:       latencyMs,
		attempts:        attempts,
		retryScope:      scope,
		toolsPresent:    dc.toolsPresent,
		toolsStripped:   dc.toolsStripped,
	}
}

func (dc diagnosticContext) buildEventWithExcerpt(level, event, message string, key *pool.Key, status int, latencyMs int64, attempts int, scope retryScope, excerpt string) requestDiagnosticInput {
	input := dc.buildEvent(level, event, message, key, status, latencyMs, attempts, scope)
	input.upstreamErrorExcerpt = excerpt
	return input
}

func (dc diagnosticContext) buildStreamFailure(level, event, message string, key *pool.Key, status int, latencyMs int64, attempts int, scope retryScope, side streamFailureSide, streamErr error) requestDiagnosticInput {
	input := dc.buildEvent(level, event, message, key, status, latencyMs, attempts, scope)
	input.streamFailureSide = side
	input.streamError = streamErr
	return input
}

type requestMeta struct {
	model        string
	stream       bool
	toolsPresent bool
	payload      map[string]json.RawMessage // 预解析的请求体，供 rewriteRequestBody 复用
}

func requestDiagnosticEvent(input requestDiagnosticInput) logx.Event {
	data := map[string]any{
		"tools_present":  input.toolsPresent,
		"tools_stripped": input.toolsStripped,
	}
	if input.upstreamErrorExcerpt != "" {
		data["upstream_error_excerpt"] = logx.RedactSensitiveText(input.upstreamErrorExcerpt)
	}
	if input.streamFailureSide != "" {
		data["stream_failure_side"] = string(input.streamFailureSide)
	}
	if input.streamError != nil {
		data["stream_error"] = logx.RedactSensitiveText(input.streamError.Error())
	}

	event := logx.Event{
		Level:           input.level,
		Category:        logx.CategoryProxy,
		Event:           input.event,
		Message:         input.message,
		RequestID:       input.requestID,
		ClientRequestID: input.clientRequestID,
		Model:           input.model,
		Stream:          input.stream,
		Status:          input.status,
		LatencyMs:       input.latencyMs,
		Attempts:        input.attempts,
		RetryScope:      input.retryScope.String(),
		Data:            data,
	}
	if input.request != nil {
		event.Method = input.request.Method
		event.Path = input.request.URL.Path
	}
	if input.runtime != nil {
		event.ProviderID = input.runtime.providerID
	}
	if input.key != nil {
		event.KeyID = state.KeyID(input.key.Value)
		event.KeyHint = logx.MaskSecret(input.key.Value)
	}
	return event
}

func nextRequestID(at time.Time) string {
	return fmt.Sprintf("req_%d_%06d", at.UnixNano(), requestSeq.Add(1))
}

func sanitizeClientRequestID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		return value[:128]
	}
	return value
}

func levelForStatus(status int) string {
	switch {
	case status >= http.StatusInternalServerError:
		return "error"
	case status >= http.StatusBadRequest:
		return "warn"
	default:
		return "info"
	}
}

func eventForStatus(status int) string {
	if status >= http.StatusBadRequest {
		return logx.EventProxyRequestFailed
	}
	return logx.EventProxyRequestCompleted
}

func messageForStatus(status int) string {
	if status >= http.StatusBadRequest {
		return "upstream rejected request"
	}
	return "request completed"
}

func errorExcerptForStatus(status int, body []byte) string {
	if status < http.StatusBadRequest {
		return ""
	}
	return errorExcerpt(body)
}

func errorExcerpt(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}
	if len(body) > upstreamErrorExcerptBytes {
		body = body[:upstreamErrorExcerptBytes]
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err == nil {
		if rawError, ok := root["error"]; ok {
			var errObj map[string]json.RawMessage
			if json.Unmarshal(rawError, &errObj) == nil {
				if rawMsg, ok := errObj["message"]; ok {
					var msg string
					if json.Unmarshal(rawMsg, &msg) == nil {
						return strings.TrimSpace(msg)
					}
				}
			}
			var errText string
			if json.Unmarshal(rawError, &errText) == nil {
				return strings.TrimSpace(errText)
			}
		}
		if rawMsg, ok := root["message"]; ok {
			var msg string
			if json.Unmarshal(rawMsg, &msg) == nil {
				return strings.TrimSpace(msg)
			}
		}
	}

	return strings.TrimSpace(string(body))
}

// quotaExhaustedIndicators 覆盖常见中英文余额、额度和信用不足错误文案（统一小写）。
var quotaExhaustedIndicators = func() [][]byte {
	raw := []string{
		"预扣费额度失败",
		"用户剩余额度",
		"余额不足",
		"额度不足",
		"insufficient_balance",
		"insufficient account balance",
		"insufficient_quota",
		"insufficient quota",
		"quota_exceeded",
		"quota exceeded",
		"insufficient credit",
		"insufficient credits",
		"insufficient balance",
		"not enough credit",
		"not enough credits",
		"balance not enough",
	}
	out := make([][]byte, len(raw))
	for i, s := range raw {
		out[i] = []byte(strings.ToLower(s))
	}
	return out
}()

// isQuotaExhaustedBody 判断 403 响应是否属于 key 级余额或额度不足。
func isQuotaExhaustedBody(body []byte) bool {
	if isQuotaExhaustedErrorCode(body) {
		return true
	}
	lowered := bytes.ToLower(body)
	for _, indicator := range quotaExhaustedIndicators {
		if bytes.Contains(lowered, indicator) {
			return true
		}
	}
	return false
}

func isQuotaExhaustedErrorCode(body []byte) bool {
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(payload.Error.Code)) {
	case "insufficient_balance", "insufficient_quota", "quota_exceeded":
		return true
	default:
		return false
	}
}

// buildRequest 基于指定运行时快照构造上游请求，并覆盖认证头为当前选中的 key。
func buildRequest(rt *runtimeConfig, r *http.Request, key *pool.Key, body []byte, meta requestMeta) (*http.Request, error) {
	if rt == nil || rt.targetURL == nil {
		return nil, errors.New("proxy runtime config is not ready")
	}

	outURL := *rt.targetURL
	outURL.Path = singleJoiningSlash(rt.targetURL.Path, r.URL.Path)
	outURL.RawQuery = r.URL.RawQuery

	outBody := rewriteRequestBody(body, rt.stripTools, meta)

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, outURL.String(), bytes.NewReader(outBody))
	if err != nil {
		return nil, err
	}

	copyHeaders(outReq.Header, r.Header)
	outReq.Header.Set("Authorization", "Bearer "+key.Value)
	outReq.Header.Set("X-Api-Key", key.Value)
	outReq.Header.Del("X-Forwarded-For")
	outReq.Header.Del("X-Real-Ip")
	outReq.Host = rt.targetURL.Host

	return outReq, nil
}

func rewriteRequestBody(body []byte, stripTools bool, meta requestMeta) []byte {
	if len(bytes.TrimSpace(body)) == 0 {
		return body
	}

	// 复用 extractRequestMeta 预解析的 payload，避免重复 JSON 解析。
	var payload map[string]json.RawMessage
	if meta.payload != nil {
		payload = meta.payload
	} else {
		if err := json.Unmarshal(body, &payload); err != nil {
			return body
		}
	}

	changed := false
	if stripTools {
		for _, field := range []string{"tools", "tool_choice", "parallel_tool_calls", "functions", "function_call"} {
			if _, ok := payload[field]; ok {
				delete(payload, field)
				changed = true
			}
		}
	}
	if ensureStreamIncludeUsage(payload) {
		changed = true
	}
	if !changed {
		return body
	}

	out, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return out
}

func ensureStreamIncludeUsage(payload map[string]json.RawMessage) bool {
	rawStream, ok := payload["stream"]
	if !ok {
		return false
	}
	var stream bool
	if err := json.Unmarshal(rawStream, &stream); err != nil || !stream {
		return false
	}

	options := make(map[string]json.RawMessage)
	if rawOptions, ok := payload["stream_options"]; ok && len(bytes.TrimSpace(rawOptions)) > 0 && string(bytes.TrimSpace(rawOptions)) != "null" {
		if err := json.Unmarshal(rawOptions, &options); err != nil {
			return false
		}
	}

	if rawIncludeUsage, ok := options["include_usage"]; ok {
		var includeUsage bool
		if err := json.Unmarshal(rawIncludeUsage, &includeUsage); err == nil {
			return false
		}
	}

	options["include_usage"] = json.RawMessage("true")
	rawOptions, err := json.Marshal(options)
	if err != nil {
		return false
	}
	payload["stream_options"] = rawOptions
	return true
}

// readRequestBody 在支持重试的前提下读取请求体，并限制最大内存占用。
func readRequestBody(r *http.Request, maxBodyBytes int64) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	defer r.Body.Close()
	if maxBodyBytes <= 0 {
		maxBodyBytes = config.DefaultMaxBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, errRequestBodyTooLarge
	}
	return body, nil
}

// parseRetryAfter 解析 Retry-After 头，支持秒数和 HTTP-date 两种格式。
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(header); err == nil {
		d = time.Until(t)
	}
	if d <= 0 {
		return 0
	}
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}

// isRetryableUpstreamStatus 判断哪些网关类状态适合在同一 provider 内换 key 再试。
func isRetryableUpstreamStatus(status int) bool {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// classifyTransportRetryScope 区分换 key 可能有用的连接级故障和换 key 通常无效的 provider 级故障。
func classifyTransportRetryScope(err error) retryScope {
	if err == nil {
		return retryScopeNone
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return retryScopeProvider
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return retryScopeProvider
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "tls handshake") || strings.Contains(msg, "dial tcp") {
			return retryScopeProvider
		}
		return retryScopeConnection
	}

	msg := strings.ToLower(err.Error())
	providerIndicators := []string{
		"no such host",
		"connection refused",
		"actively refused",
		"network is unreachable",
		"tls handshake",
		"certificate",
	}
	for _, indicator := range providerIndicators {
		if strings.Contains(msg, indicator) {
			return retryScopeProvider
		}
	}
	return retryScopeConnection
}

// copyHeaders 复制普通 HTTP 头，并跳过逐跳头，避免代理协议层头泄漏。
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		if isHopByHop(k) {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

type streamOptions struct {
	keepAlive        time.Duration
	idleTimeout      time.Duration
	maxDuration      time.Duration
	contentType      string
	keepAliveComment string
}

type streamReadResult struct {
	buf *[]byte
	n   int
	err error
}

// streamBody 按小块转发响应体并在每次写入后刷新，保证 SSE 不被代理缓冲。
// closer 用于在超时或错误退出时关闭上游 body，解除阻塞的 Read goroutine。
func streamBody(w http.ResponseWriter, body io.Reader, closer func(), options ...streamOptions) error {
	opts := effectiveStreamOptions(options)
	flusher, canFlush := w.(http.Flusher)
	canKeepAlive := canFlush && opts.keepAlive > 0 && isEventStreamContentType(opts.contentType)
	if opts.keepAliveComment == "" {
		opts.keepAliveComment = defaultStreamKeepAliveComment
	}
	if !canKeepAlive && opts.idleTimeout <= 0 && opts.maxDuration <= 0 {
		return streamBodySimple(w, body, canFlush, flusher)
	}

	readCh := make(chan streamReadResult, 1)
	startRead := func() {
		bufPtr := streamBufPool.Get().(*[]byte)
		go func() {
			n, err := body.Read(*bufPtr)
			readCh <- streamReadResult{buf: bufPtr, n: n, err: err}
		}()
	}

	cleanup := func() {
		if closer != nil {
			closer()
		}
	}

	start := time.Now()
	lastActivity := start

	var keepAliveTimer, idleTimer, maxTimer *time.Timer
	if canKeepAlive {
		keepAliveTimer = time.NewTimer(opts.keepAlive)
		defer keepAliveTimer.Stop()
	}
	if opts.idleTimeout > 0 {
		idleTimer = time.NewTimer(opts.idleTimeout)
		defer idleTimer.Stop()
	}
	if opts.maxDuration > 0 {
		maxTimer = time.NewTimer(opts.maxDuration)
		defer maxTimer.Stop()
	}

	startRead()
	for {
		var keepAliveC, idleC, maxC <-chan time.Time
		if keepAliveTimer != nil {
			keepAliveC = keepAliveTimer.C
		}
		if idleTimer != nil {
			idleC = idleTimer.C
		}
		if maxTimer != nil {
			maxC = maxTimer.C
		}

		var result streamReadResult
		select {
		case result = <-readCh:
		case <-keepAliveC:
			if _, writeErr := w.Write([]byte(opts.keepAliveComment)); writeErr != nil {
				cleanup()
				if errors.Is(writeErr, context.Canceled) {
					return &streamFailureError{side: streamFailureSideClientCanceled, err: writeErr}
				}
				return &streamFailureError{side: streamFailureSideClientWrite, err: writeErr}
			}
			flusher.Flush()
			resetStreamTimer(keepAliveTimer, opts.keepAlive)
			resetStreamTimer(idleTimer, opts.idleTimeout-time.Since(lastActivity))
			resetStreamTimer(maxTimer, opts.maxDuration-time.Since(start))
			continue
		case <-idleC:
			cleanup()
			return &streamFailureError{side: streamFailureSideUpstreamRead, err: errStreamIdleTimeout}
		case <-maxC:
			cleanup()
			return &streamFailureError{side: streamFailureSideUpstreamRead, err: errStreamMaxDurationExceeded}
		}

		n, err := result.n, result.err
		if n > 0 {
			if _, writeErr := w.Write((*result.buf)[:n]); writeErr != nil {
				streamBufPool.Put(result.buf)
				cleanup()
				if errors.Is(writeErr, context.Canceled) {
					return &streamFailureError{side: streamFailureSideClientCanceled, err: writeErr}
				}
				return &streamFailureError{side: streamFailureSideClientWrite, err: writeErr}
			}
			if canFlush {
				flusher.Flush()
			}
			lastActivity = time.Now()
		}
		streamBufPool.Put(result.buf)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			cleanup()
			if errors.Is(err, context.Canceled) {
				return &streamFailureError{side: streamFailureSideClientCanceled, err: err}
			}
			return &streamFailureError{side: streamFailureSideUpstreamRead, err: err}
		}

		resetStreamTimer(keepAliveTimer, opts.keepAlive)
		resetStreamTimer(idleTimer, opts.idleTimeout-time.Since(lastActivity))
		resetStreamTimer(maxTimer, opts.maxDuration-time.Since(start))
		startRead()
	}
}

func streamBodySimple(w http.ResponseWriter, body io.Reader, canFlush bool, flusher http.Flusher) error {
	bufPtr := streamBufPool.Get().(*[]byte)
	defer streamBufPool.Put(bufPtr)
	buf := *bufPtr
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				if errors.Is(writeErr, context.Canceled) {
					return &streamFailureError{side: streamFailureSideClientCanceled, err: writeErr}
				}
				return &streamFailureError{side: streamFailureSideClientWrite, err: writeErr}
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return &streamFailureError{side: streamFailureSideClientCanceled, err: err}
			}
			return &streamFailureError{side: streamFailureSideUpstreamRead, err: err}
		}
	}
}

func effectiveStreamOptions(options []streamOptions) streamOptions {
	if len(options) == 0 {
		return streamOptions{}
	}
	return options[0]
}

func isEventStreamContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "text/event-stream"
}

func resetStreamTimer(t *time.Timer, d time.Duration) {
	if t == nil {
		return
	}
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	if d < 0 {
		d = 0
	}
	t.Reset(d)
}

// writeProxyError 输出统一代理 JSON 错误，使用编码器保证消息被正确转义。
func writeProxyError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := map[string]any{
		"error": map[string]string{
			"message": "proxy: " + msg,
			"type":    "proxy_error",
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

var hopByHopHeaders = map[string]bool{
	"Accept-Encoding":     true,
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// isHopByHop 判断 HTTP 头是否属于逐跳头，这类头不能透传给上游或客户端。
func isHopByHop(h string) bool {
	return hopByHopHeaders[http.CanonicalHeaderKey(h)]
}

// singleJoiningSlash 拼接上游基础路径和客户端请求路径，避免多斜杠或缺斜杠。
// 当两侧都包含同一个 API 前缀（例如 /v1）时，只保留一份，避免转发到 /v1/v1/...
func singleJoiningSlash(a, b string) string {
	if hasPathPrefix(b, a) {
		return b
	}
	aSlash := strings.HasSuffix(a, "/")
	bSlash := strings.HasPrefix(b, "/")
	switch {
	case aSlash && bSlash:
		return a + b[1:]
	case !aSlash && !bSlash:
		return a + "/" + b
	}
	return a + b
}

func hasPathPrefix(path, prefix string) bool {
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return false
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
