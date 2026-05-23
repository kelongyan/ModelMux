package logx

const (
	CategoryAdmin     = "admin"
	CategoryConfig    = "config"
	CategoryLifecycle = "lifecycle"
	CategoryProxy     = "proxy"
	CategoryRetry     = "retry"
	CategoryState     = "state"
	CategoryStream    = "stream"
	CategoryUpstream  = "upstream"

	EventAdminReloadFailed   = "admin.reload_failed"
	EventAdminReloadOK       = "admin.reload_ok"
	EventAdminListening      = "lifecycle.admin_listening"
	EventBodyReadFailed      = "proxy.body_read_failed"
	EventBodyTooLarge        = "proxy.body_too_large"
	EventConfigLoadFailed    = "config.load_failed"
	EventConfigWatchError    = "config.watch_error"
	EventConfigWatchFailed   = "config.watch_failed"
	EventConfigWatchStarted  = "config.watch_started"
	EventClientCanceled      = "proxy.client_canceled"
	EventConfigReloaded      = "config.reloaded"
	EventConfigReloadFailed  = "config.reload_failed"
	EventHandlerCreateFailed = "lifecycle.handler_create_failed"
	EventKeyPoolInitialized  = "lifecycle.key_pool_initialized"
	EventKeyCooling          = "retry.key_cooling"
	EventKeyInvalid          = "retry.key_invalid"
	EventKeyQuotaExhausted   = "retry.key_quota_exhausted"
	EventNoAvailableKey      = "proxy.no_available_key"
	EventProxyListening      = "lifecycle.proxy_listening"
	EventProxySuccess        = "proxy.success"
	EventRequestStart        = "proxy.request_start"
	EventRetryExhausted      = "retry.exhausted"
	EventRuntimeNotReady     = "proxy.runtime_not_ready"
	EventServerError         = "lifecycle.server_error"
	EventStateLoadFailed     = "state.load_failed"
	EventStateRestored       = "state.restored"
	EventStateSaveFailed     = "state.save_failed"
	EventShutdownComplete    = "lifecycle.shutdown_complete"
	EventShutdownStart       = "lifecycle.shutdown_start"
	EventStreamFailed        = "stream.failed"
	EventUpstreamUnreachable = "upstream.unreachable"
	EventUpstreamUnexpected  = "upstream.unexpected_status"
)

// Fields 统一注入日志分类字段，方便用 category/event 过滤不同类型的日志。
func Fields(category, event string, attrs ...any) []any {
	fields := []any{"category", category, "event", event}
	return append(fields, attrs...)
}

// MaskSecret 返回适合日志展示的密钥短标识，避免把完整 API Key 写入日志。
func MaskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 6 {
		return "***" + secret
	}
	return "***" + secret[len(secret)-6:]
}
