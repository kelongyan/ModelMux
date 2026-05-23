package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/claude-key-proxy/config"
	"github.com/claude-key-proxy/pool"
)

var errClientCanceled = errors.New("client canceled request")

type Handler struct {
	pool      *pool.Pool
	targetURL *url.URL
	client    *http.Client
	cfg       *config.Config
}

func NewHandler(p *pool.Pool, cfg *config.Config) (*Handler, error) {
	target, err := url.Parse(cfg.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target_url: %w", err)
	}

	client := &http.Client{
		Timeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
		},
	}

	return &Handler{
		pool:      p,
		targetURL: target,
		client:    client,
		cfg:       cfg,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, err := readRequestBody(r)
	if err != nil {
		slog.Error("failed to read request body", "path", r.URL.Path, "err", err)
		writeProxyError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var lastStatus int

	for attempt := 0; attempt <= h.cfg.MaxRetries; attempt++ {
		key, err := h.pool.Next()
		if err != nil {
			slog.Error("no available keys", "path", r.URL.Path)
			writeProxyError(w, http.StatusServiceUnavailable, "no available API keys")
			return
		}

		status, retryAfter, err := h.forward(w, r, key, body)
		if err == nil {
			slog.Info("proxied",
				"path", r.URL.Path,
				"status", status,
				"attempt", attempt+1,
				"latency_ms", time.Since(start).Milliseconds(),
			)
			return
		}
		if errors.Is(err, errClientCanceled) {
			slog.Info("request canceled", "path", r.URL.Path, "attempt", attempt+1)
			return
		}

		lastStatus = status

		switch status {
		case http.StatusTooManyRequests:
			cooling := h.pool.CoolingDuration(h.cfg.CoolingSeconds)
			if retryAfter > 0 {
				cooling = retryAfter
			}
			key.MarkCooling(cooling)
			slog.Warn("key rate-limited, cooling",
				"attempt", attempt+1,
				"cooling_s", cooling.Seconds(),
			)
		case http.StatusUnauthorized:
			key.MarkInvalid()
			slog.Warn("key invalid, marking dead", "attempt", attempt+1)
		default:
			// Non-retryable — already written to w inside forward().
			slog.Error("upstream error", "status", status, "err", err)
			return
		}
	}

	slog.Error("all retries exhausted", "last_status", lastStatus)
	writeProxyError(w, http.StatusServiceUnavailable, "all keys exhausted after retries")
}

// forward sends the request upstream with the given key and streams the response back.
// Returns (0, 0, nil) on success.
// Returns (statusCode, retryAfter, err) for retryable errors (429/401).
// Returns (statusCode, 0, err) for non-retryable errors (already written to w).
func (h *Handler) forward(w http.ResponseWriter, r *http.Request, key *pool.Key, body []byte) (int, time.Duration, error) {
	outReq, err := h.buildRequest(r, key, body)
	if err != nil {
		writeProxyError(w, http.StatusInternalServerError, "failed to build request")
		return http.StatusInternalServerError, 0, err
	}

	upstreamStart := time.Now()
	resp, err := h.client.Do(outReq)
	if err != nil {
		if errors.Is(r.Context().Err(), context.Canceled) {
			return 0, 0, errClientCanceled
		}
		writeProxyError(w, http.StatusBadGateway, fmt.Sprintf("upstream unreachable: %s", err))
		return http.StatusBadGateway, 0, err
	}
	defer resp.Body.Close()

	// Retryable: extract Retry-After before returning without writing to w.
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return resp.StatusCode, retryAfter, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return resp.StatusCode, 0, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}

	key.RecordLatency(time.Since(upstreamStart))

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	streamBody(w, resp.Body)
	return resp.StatusCode, 0, nil
}

func (h *Handler) buildRequest(r *http.Request, key *pool.Key, body []byte) (*http.Request, error) {
	outURL := *h.targetURL
	outURL.Path = singleJoiningSlash(h.targetURL.Path, r.URL.Path)
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
	outReq.Host = h.targetURL.Host

	return outReq, nil
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// parseRetryAfter parses the Retry-After header value (seconds integer or HTTP-date).
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

func streamBody(w http.ResponseWriter, body io.Reader) {
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

func writeProxyError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error":{"message":"proxy: %s","type":"proxy_error"}}`, msg)
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

func isHopByHop(h string) bool {
	return hopByHopHeaders[http.CanonicalHeaderKey(h)]
}

func singleJoiningSlash(a, b string) string {
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
