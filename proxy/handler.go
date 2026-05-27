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
	"sync/atomic"
	"time"

	"github.com/kelongyan/ModelMux/config"
	"github.com/kelongyan/ModelMux/logx"
	"github.com/kelongyan/ModelMux/pool"
)

var (
	errClientCanceled      = errors.New("client canceled request")
	errStreamFailed        = errors.New("stream failed")
	errRequestBodyTooLarge = errors.New("request body too large")
	errKeyQuotaExhausted   = errors.New("key quota exhausted")
	errRetryableUpstream   = errors.New("retryable upstream failure")
)

const quotaErrorInspectBytes int64 = 64 * 1024

type Handler struct {
	pools           *pool.ProviderPools
	runtime         atomic.Value
	stateChangeHook atomic.Value
}

type stateChangeHook func(immediate bool)

type runtimeConfig struct {
	providerID              string
	targetURL               *url.URL
	keyPool                 *pool.Pool
	client                  *http.Client
	transport               *http.Transport
	coolingSeconds          int
	maxRetries              int
	maxTransientRetries     int
	transientCoolingSeconds int
	waitForKeyTimeout       time.Duration
	maxBodyBytes            int64
}

type retryScope int

const (
	retryScopeNone retryScope = iota
	retryScopeKey
	retryScopeConnection
	retryScopeProvider
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

// notifyStateChanged 通知外部保存 key 池状态；immediate 表示需要尽快落盘。
func (h *Handler) notifyStateChanged(immediate bool) {
	hook, _ := h.stateChangeHook.Load().(stateChangeHook)
	if hook != nil {
		hook(immediate)
	}
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

	old, _ := h.runtime.Load().(*runtimeConfig)
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
	rt, _ := h.runtime.Load().(*runtimeConfig)
	return rt
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
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   time.Duration(connectTimeoutSeconds) * time.Second,
		ResponseHeaderTimeout: time.Duration(responseHeaderTimeoutSeconds) * time.Second,
	}
	client := &http.Client{
		Timeout: time.Duration(requestTimeoutSeconds) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: transport,
	}

	return &runtimeConfig{
		providerID:              provider.ID,
		targetURL:               target,
		keyPool:                 keyPool,
		client:                  client,
		transport:               transport,
		coolingSeconds:          effectiveInt(cfg.CoolingSeconds, config.DefaultCoolingSeconds),
		maxRetries:              effectiveInt(cfg.MaxRetries, config.DefaultMaxRetries),
		maxTransientRetries:     effectiveInt(cfg.MaxTransientRetries, config.DefaultMaxTransientRetries),
		transientCoolingSeconds: effectiveInt(cfg.TransientCoolingSeconds, config.DefaultTransientCoolingSeconds),
		waitForKeyTimeout:       time.Duration(effectiveInt(cfg.WaitForKeyTimeoutMS, config.DefaultWaitForKeyTimeoutMS)) * time.Millisecond,
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
	rt := h.snapshot()
	if rt == nil {
		slog.Error("proxy runtime config is not ready", logx.Fields(logx.CategoryProxy, logx.EventRuntimeNotReady)...)
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

	var lastStatus int
	var lastErr error
	lastScope := retryScopeNone
	transientFailures := 0
	waitBudget := rt.waitForKeyTimeout

	for attempt := 0; attempt <= rt.maxRetries; attempt++ {
		key, err := rt.keyPool.Next()
		if err != nil {
			if errors.Is(err, pool.ErrNoAvailableKey) {
				if waited := h.waitForCoolingKey(r.Context(), rt, waitBudget); waited > 0 {
					waitBudget -= waited
					attempt--
					continue
				}
			}
			slog.Error("no available keys", logx.Fields(logx.CategoryProxy, logx.EventNoAvailableKey,
				"path", r.URL.Path,
				"provider_id", rt.providerID,
				"wait_budget_ms", waitBudget.Milliseconds(),
			)...)
			writeProxyError(w, http.StatusServiceUnavailable, "no available API keys")
			return
		}

		status, retryAfter, scope, err := h.forward(w, r, rt, key, body)
		key.FinishRequest()
		if err == nil {
			slog.Info("proxied", logx.Fields(logx.CategoryProxy, logx.EventProxySuccess,
				"path", r.URL.Path,
				"status", status,
				"attempt", attempt+1,
				"provider_id", rt.providerID,
				"key_id", logx.MaskSecret(key.Value),
				"latency_ms", time.Since(start).Milliseconds(),
			)...)
			h.notifyStateChanged(false)
			return
		}
		if errors.Is(err, errClientCanceled) {
			slog.Info("request canceled", logx.Fields(logx.CategoryProxy, logx.EventClientCanceled,
				"path", r.URL.Path,
				"attempt", attempt+1,
				"provider_id", rt.providerID,
				"key_id", logx.MaskSecret(key.Value),
			)...)
			return
		}
		if errors.Is(err, errStreamFailed) {
			return
		}

		lastStatus = status
		lastErr = err
		lastScope = scope
		stopRetrying := false

		switch scope {
		case retryScopeKey:
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
					"key_id", logx.MaskSecret(key.Value),
					"cooling_s", cooling.Seconds(),
				)...)
			case http.StatusUnauthorized:
				key.MarkInvalid()
				h.notifyStateChanged(true)
				slog.Warn("key invalid, marking dead", logx.Fields(logx.CategoryRetry, logx.EventKeyInvalid,
					"attempt", attempt+1,
					"provider_id", rt.providerID,
					"key_id", logx.MaskSecret(key.Value),
				)...)
			case http.StatusForbidden:
				if !errors.Is(err, errKeyQuotaExhausted) {
					slog.Error("upstream error", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnexpected,
						"status", status,
						"provider_id", rt.providerID,
						"err", err,
					)...)
					return
				}
				key.MarkInvalid()
				h.notifyStateChanged(true)
				slog.Warn("key quota exhausted, marking dead", logx.Fields(logx.CategoryRetry, logx.EventKeyQuotaExhausted,
					"attempt", attempt+1,
					"provider_id", rt.providerID,
					"key_id", logx.MaskSecret(key.Value),
				)...)
			default:
				slog.Error("upstream error", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnexpected,
					"status", status,
					"provider_id", rt.providerID,
					"err", err,
				)...)
				return
			}
		case retryScopeConnection:
			transientFailures++
			cooling := rt.keyPool.CoolingDuration(rt.transientCoolingSeconds)
			if retryAfter > 0 {
				cooling = retryAfter
			}
			key.MarkCooling(cooling)
			h.notifyStateChanged(true)
			slog.Warn("key transient failure, cooling", logx.Fields(logx.CategoryRetry, logx.EventKeyTransientFailure,
				"attempt", attempt+1,
				"provider_id", rt.providerID,
				"key_id", logx.MaskSecret(key.Value),
				"status", status,
				"scope", scope.String(),
				"cooling_s", cooling.Seconds(),
				"transient_failures", transientFailures,
				"max_transient_retries", rt.maxTransientRetries,
				"err", err,
			)...)
			stopRetrying = transientFailures > rt.maxTransientRetries
		case retryScopeProvider:
			transientFailures++
			slog.Warn("provider transient failure", logx.Fields(logx.CategoryRetry, logx.EventProviderTransient,
				"attempt", attempt+1,
				"provider_id", rt.providerID,
				"key_id", logx.MaskSecret(key.Value),
				"status", status,
				"scope", scope.String(),
				"transient_failures", transientFailures,
				"max_transient_retries", rt.maxTransientRetries,
				"err", err,
			)...)
			stopRetrying = transientFailures > rt.maxTransientRetries
		default:
			// 非重试错误已经在 forward 中写出响应，这里只记录分类日志。
			slog.Error("upstream error", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnexpected,
				"status", status,
				"provider_id", rt.providerID,
				"err", err,
			)...)
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
		writeProxyError(w, http.StatusServiceUnavailable, "upstream temporarily unavailable after retries")
		return
	}
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

// forward 使用指定 key 请求上游，并把上游响应流式写回客户端。
// 成功时返回状态码和 nil；429、401 和余额不足类 403 返回可重试错误；其他错误会在本函数内写出响应。
func (h *Handler) forward(w http.ResponseWriter, r *http.Request, rt *runtimeConfig, key *pool.Key, body []byte) (int, time.Duration, retryScope, error) {
	outReq, err := buildRequest(rt, r, key, body)
	if err != nil {
		writeProxyError(w, http.StatusInternalServerError, "failed to build request")
		return http.StatusInternalServerError, 0, retryScopeNone, err
	}

	upstreamStart := time.Now()
	resp, err := rt.client.Do(outReq)
	if err != nil {
		if errors.Is(r.Context().Err(), context.Canceled) {
			return 0, 0, retryScopeNone, errClientCanceled
		}
		scope := classifyTransportRetryScope(err)
		slog.Error("upstream unreachable", logx.Fields(logx.CategoryUpstream, logx.EventUpstreamUnreachable,
			"path", r.URL.Path,
			"provider_id", rt.providerID,
			"key_id", logx.MaskSecret(key.Value),
			"scope", scope.String(),
			"err", err,
		)...)
		return http.StatusBadGateway, 0, scope, fmt.Errorf("%w: %v", errRetryableUpstream, err)
	}
	defer resp.Body.Close()

	// 429/401 需要换 key 重试，因此不能提前向客户端写响应头。
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return resp.StatusCode, retryAfter, retryScopeKey, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return resp.StatusCode, 0, retryScopeKey, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	if isRetryableUpstreamStatus(resp.StatusCode) {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return resp.StatusCode, retryAfter, retryScopeProvider, fmt.Errorf("%w: upstream returned %d", errRetryableUpstream, resp.StatusCode)
	}

	responseBody := io.Reader(resp.Body)
	if resp.StatusCode == http.StatusForbidden {
		prefix, replayBody, err := readResponsePrefix(resp.Body, quotaErrorInspectBytes)
		if err != nil {
			writeProxyError(w, http.StatusBadGateway, "failed to read upstream error body")
			return http.StatusBadGateway, 0, retryScopeNone, err
		}
		if isQuotaExhaustedBody(prefix) {
			return resp.StatusCode, 0, retryScopeKey, fmt.Errorf("%w: upstream returned %d", errKeyQuotaExhausted, resp.StatusCode)
		}
		responseBody = replayBody
	}

	key.RecordLatency(time.Since(upstreamStart))

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if err := streamBody(w, responseBody); err != nil {
		slog.Warn("stream failed", logx.Fields(logx.CategoryStream, logx.EventStreamFailed,
			"path", r.URL.Path,
			"status", resp.StatusCode,
			"provider_id", rt.providerID,
			"key_id", logx.MaskSecret(key.Value),
			"err", err,
		)...)
		return resp.StatusCode, 0, retryScopeNone, fmt.Errorf("%w: %v", errStreamFailed, err)
	}
	return resp.StatusCode, 0, retryScopeNone, nil
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

// quotaExhaustedIndicators 覆盖常见中英文余额、额度和信用不足错误文案。
var quotaExhaustedIndicators = []string{
	"预扣费额度失败",
	"用户剩余额度",
	"余额不足",
	"额度不足",
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

// isQuotaExhaustedBody 判断 403 响应是否属于 key 级余额或额度不足。
func isQuotaExhaustedBody(body []byte) bool {
	normalized := strings.ToLower(string(body))
	for _, indicator := range quotaExhaustedIndicators {
		if strings.Contains(normalized, indicator) {
			return true
		}
	}
	return false
}

// buildRequest 基于原请求构造上游请求，并覆盖认证头为当前选中的 key。
func (h *Handler) buildRequest(r *http.Request, key *pool.Key, body []byte) (*http.Request, error) {
	return buildRequest(h.snapshot(), r, key, body)
}

// buildRequest 基于指定运行时快照构造上游请求，并覆盖认证头为当前选中的 key。
func buildRequest(rt *runtimeConfig, r *http.Request, key *pool.Key, body []byte) (*http.Request, error) {
	if rt == nil || rt.targetURL == nil {
		return nil, errors.New("proxy runtime config is not ready")
	}

	outURL := *rt.targetURL
	outURL.Path = singleJoiningSlash(rt.targetURL.Path, r.URL.Path)
	outURL.RawQuery = r.URL.RawQuery

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, outURL.String(), bytes.NewReader(body))
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
	if secs, err := strconv.Atoi(header); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
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

// streamBody 按小块转发响应体并在每次写入后刷新，保证 SSE 不被代理缓冲。
func streamBody(w http.ResponseWriter, body io.Reader) error {
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
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
