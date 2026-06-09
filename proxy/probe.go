package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kelongyan/ModelMux/config"
)

const keyProbeMaxTimeout = 15 * time.Second

type KeyTestResult struct {
	OK                bool   `json:"ok"`
	StatusCode        int    `json:"status_code"`
	LatencyMs         int64  `json:"latency_ms"`
	Scope             string `json:"scope,omitempty"`
	Error             string `json:"error,omitempty"`
	RetryAfterSeconds int64  `json:"retry_after_seconds,omitempty"`
}

// ProbeKey performs a one-off /models request with the given key.
// It reports the result without mutating any runtime key state.
func ProbeKey(ctx context.Context, cfg *config.Config, provider config.ProviderConfig, key string) KeyTestResult {
	start := time.Now()
	result := KeyTestResult{}

	rt, err := newRuntimeConfig(cfg, provider, nil)
	if err != nil {
		result.Error = err.Error()
		result.LatencyMs = time.Since(start).Milliseconds()
		return result
	}
	if rt.transport != nil {
		defer rt.transport.CloseIdleConnections()
	}

	outURL := *rt.targetURL
	outURL.Path = singleJoiningSlash(rt.targetURL.Path, "/models")
	outURL.RawQuery = ""

	timeout := rt.requestTimeout
	if timeout <= 0 || timeout > keyProbeMaxTimeout {
		timeout = keyProbeMaxTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, outURL.String(), nil)
	if err != nil {
		result.Error = fmt.Sprintf("build request: %v", err)
		result.LatencyMs = time.Since(start).Milliseconds()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-Api-Key", key)
	req.Header.Set("Accept", "application/json")

	resp, err := rt.client.Do(req)
	result.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		scope := classifyTransportRetryScope(err)
		result.Scope = scope.String()
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.OK = resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")); retryAfter > 0 {
		result.RetryAfterSeconds = int64(retryAfter.Seconds())
	}

	switch {
	case result.OK:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, upstreamRetryDrainBytes))
	case resp.StatusCode == http.StatusUnauthorized:
		result.Scope = retryScopeKey.String()
		result.Error = "unauthorized"
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, upstreamRetryDrainBytes))
	case resp.StatusCode == http.StatusTooManyRequests:
		result.Scope = retryScopeKey.String()
		result.Error = "rate limited"
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, upstreamRetryDrainBytes))
	case resp.StatusCode == http.StatusForbidden:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, quotaErrorInspectBytes))
		if isQuotaExhaustedBody(body) {
			result.Scope = retryScopeKey.String()
			result.Error = "quota exhausted"
			break
		}
		result.Error = fmt.Sprintf("upstream returned %d", resp.StatusCode)
	case isRetryableUpstreamStatus(resp.StatusCode):
		result.Scope = retryScopeProvider.String()
		result.Error = fmt.Sprintf("upstream returned %d", resp.StatusCode)
	default:
		result.Error = fmt.Sprintf("upstream returned %d", resp.StatusCode)
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, upstreamRetryDrainBytes))
	}
	return result
}
