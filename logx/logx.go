package logx

import "regexp"

const (
	CategoryAdmin     = "admin"
	CategoryConfig    = "config"
	CategoryLifecycle = "lifecycle"
	CategoryProxy     = "proxy"
	CategoryProvider  = "provider"
	CategoryRetry     = "retry"
	CategoryState     = "state"
	CategoryStats     = "stats"
	CategoryStream    = "stream"
	CategoryUpstream  = "upstream"

	EventAdminReloadFailed       = "admin.reload_failed"
	EventAdminReloadOK           = "admin.reload_ok"
	EventAdminListening          = "lifecycle.admin_listening"
	EventBodyReadFailed          = "proxy.body_read_failed"
	EventBodyTooLarge            = "proxy.body_too_large"
	EventConfigLoadFailed        = "config.load_failed"
	EventConfigWatchError        = "config.watch_error"
	EventConfigWatchFailed       = "config.watch_failed"
	EventConfigWatchStarted      = "config.watch_started"
	EventClientCanceled          = "proxy.client_canceled"
	EventConfigReloaded          = "config.reloaded"
	EventConfigReloadFailed      = "config.reload_failed"
	EventHandlerCreateFailed     = "lifecycle.handler_create_failed"
	EventKeyPoolInitialized      = "lifecycle.key_pool_initialized"
	EventKeyCooling              = "retry.key_cooling"
	EventKeyInvalid              = "retry.key_invalid"
	EventKeyQuotaExhausted       = "retry.key_quota_exhausted"
	EventKeyTransientFailure     = "retry.key_transient_failure"
	EventProviderTransient       = "retry.provider_transient"
	EventProviderCircuitClosed   = "provider.circuit_closed"
	EventProviderCircuitHalfOpen = "provider.circuit_half_open"
	EventProviderCircuitOpened   = "provider.circuit_opened"
	EventProviderCircuitRejected = "provider.circuit_rejected"
	EventNoAvailableKey          = "proxy.no_available_key"
	EventProxyListening          = "lifecycle.proxy_listening"
	EventProxyRequestCompleted   = "proxy.request_completed"
	EventProxyRequestFailed      = "proxy.request_failed"
	EventProxyRequestStarted     = "proxy.request_started"
	EventProxySuccess            = "proxy.success"
	EventRequestStart            = "proxy.request_start"
	EventRetryExhausted          = "retry.exhausted"
	EventRuntimeNotReady         = "proxy.runtime_not_ready"
	EventServerError             = "lifecycle.server_error"
	EventStateLoadFailed         = "state.load_failed"
	EventStateRestored           = "state.restored"
	EventStateSaveFailed         = "state.save_failed"
	EventStatsQueueDropped       = "stats.queue_dropped"
	EventShutdownComplete        = "lifecycle.shutdown_complete"
	EventShutdownStart           = "lifecycle.shutdown_start"
	EventStreamFailed            = "stream.failed"
	EventUpstreamUnreachable     = "upstream.unreachable"
	EventUpstreamUnexpected      = "upstream.unexpected_status"
)

// Fields 统一注入日志分类字段，方便用 category/event 过滤不同类型的日志。
func Fields(category, event string, attrs ...any) []any {
	fields := []any{"category", category, "event", event}
	return append(fields, attrs...)
}

// Event is the shared diagnostic event shape used by persistent logs and the admin event feed.
type Event struct {
	Level           string         `json:"level"`
	Category        string         `json:"category"`
	Event           string         `json:"event"`
	Message         string         `json:"message"`
	RequestID       string         `json:"request_id,omitempty"`
	ClientRequestID string         `json:"client_request_id,omitempty"`
	ProviderID      string         `json:"provider_id,omitempty"`
	KeyID           string         `json:"key_id,omitempty"`
	KeyHint         string         `json:"key_hint,omitempty"`
	Method          string         `json:"method,omitempty"`
	Path            string         `json:"path,omitempty"`
	Model           string         `json:"model,omitempty"`
	Stream          bool           `json:"stream,omitempty"`
	Status          int            `json:"status,omitempty"`
	LatencyMs       int64          `json:"latency_ms,omitempty"`
	Attempts        int            `json:"attempts,omitempty"`
	RetryScope      string         `json:"retry_scope,omitempty"`
	Data            map[string]any `json:"data,omitempty"`
}

var redactionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)[^\s"']+`),
	regexp.MustCompile(`(?i)(x-api-key\s*:\s*)[^\s"']+`),
	regexp.MustCompile(`(?i)(api[_-]?key\s*[=:]\s*)[^\s"'&]+`),
}

// Attrs returns slog attributes matching the same event shape exposed through the admin API.
func (e Event) Attrs() []any {
	fields := Fields(e.Category, e.Event)
	appendString := func(name, value string) {
		if value != "" {
			fields = append(fields, name, value)
		}
	}
	appendString("request_id", e.RequestID)
	appendString("client_request_id", e.ClientRequestID)
	appendString("provider_id", e.ProviderID)
	appendString("key_id", e.KeyID)
	appendString("key_hint", e.KeyHint)
	appendString("method", e.Method)
	appendString("path", e.Path)
	appendString("model", e.Model)
	if e.Stream {
		fields = append(fields, "stream", e.Stream)
	}
	if e.Status != 0 {
		fields = append(fields, "status", e.Status)
	}
	if e.LatencyMs != 0 {
		fields = append(fields, "latency_ms", e.LatencyMs)
	}
	if e.Attempts != 0 {
		fields = append(fields, "attempts", e.Attempts)
	}
	appendString("retry_scope", e.RetryScope)
	if len(e.Data) > 0 {
		fields = append(fields, "data", e.Data)
	}
	return fields
}

// MaskSecret 返回适合日志展示的密钥短标识，避免把完整 API Key 写入日志。
func MaskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 6 {
		return "***"
	}
	return "***" + secret[len(secret)-6:]
}

func RedactSensitiveText(text string) string {
	redacted := text
	for _, pattern := range redactionPatterns {
		redacted = pattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	}
	return redacted
}
