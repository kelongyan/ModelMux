# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

ModelMux is a local Go reverse proxy that fronts multiple model-API providers behind one address. Clients hit the local proxy; ModelMux substitutes the real upstream URL and a real API key from the **active provider's** key pool. Each provider has its own isolated key pool — keys never rotate across providers. Module path: `github.com/kelongyan/ModelMux`. Single binary; the React/Vite admin console is embedded via `go:embed`.

## Commands

Default shell on this machine is **PowerShell** (Windows). The Makefile is usable on Windows too — only `make clean` relies on POSIX `rm`; `make build`, `make test`, `make run`, and the `build-{linux,windows,mac}` cross-compile targets just shell out to `go`.

```powershell
# Build (produces modelmux.exe)
go build -trimpath -ldflags="-s -w" -o modelmux.exe .

# Run all Go tests
go test ./...

# Race detector + vet (run before handoff)
go vet ./...
go test -race ./...

# Single test (example)
go test ./pool/ -run TestRoundRobin -v

# Frontend build (output goes to web/dist, which is embedded by web/embed.go)
cd web; npm install; npm run build

# Frontend dev with hot reload (proxies to running modelmux on :18081)
cd web; npm run dev

# One-shot start: builds binary if missing, launches in background, opens console
.\start.ps1
```

The Go binary `embed`s `web/dist`, so a fresh checkout requires `npm run build` once before `go build` succeeds — `web/dist` is git-tracked but stale builds will silently ship old UI.

## Architecture (big picture)

Two HTTP servers run in the same process from `main.go`:

- **Proxy server** (default `127.0.0.1:18080`) — forwards client requests upstream. Long timeouts on read-header / idle only; no fixed write timeout, because SSE streams can run for minutes.
- **Admin server** (default `127.0.0.1:18081`) — legacy endpoints (`/admin/health`, `/admin/status`, `/admin/reload`) plus the v1 REST API under `/admin/api/v1/*` (`dashboard`, `providers`, `settings`, `events`, `about`, `reload`, `config/backup`, `state/backup`) and the embedded SPA at `/console/`. Conservative timeouts; bound to loopback by default.

Request lifecycle (proxy side, in `proxy/handler.go`):

1. Atomic snapshot of `runtimeConfig` is loaded — this means in-flight requests keep their old upstream URL / pool / client even during a config reload.
2. Body is read with a `max_body_bytes` limit.
3. Active provider's pool returns a key via in-flight-aware round-robin (`pool.Pool.Next` — the cursor advances, but a key with fewer in-flight requests can be preferred over a busier one). Hop-by-hop headers (`Connection`, `Transfer-Encoding`, etc.) are stripped; `Authorization` and `X-Api-Key` are overwritten with the chosen key.
4. The forward result is classified into a **retry scope** that determines how the failure is treated. See "Retry scopes" below.
5. If `Next()` returns `ErrNoAvailableKey` but the pool reports a cooling key recovering within `wait_for_key_timeout_ms`, the proxy short-waits (`waitForCoolingKey`) instead of immediately returning 503.
6. Streaming responses are copied in 4 KiB chunks with `Flush()` after each write — **do not** introduce buffering or reverse this; it will break SSE.

Each `proxy.Handler` exposes a `stateChangeHook`. `main.go` wires this to a debounced `stateSaver` (2 s default) that persists key-pool snapshots to `state.json` so cooling/invalid state survives restarts.

### Retry scopes (`proxy/handler.go`)

Three orthogonal retry budgets, each with different state side-effects:

| Scope | Triggers | Effect on the key | Budget |
|---|---|---|---|
| `retryScopeKey` | upstream `401`, `429`, quota-403 (matched against `quotaExhaustedIndicators` in the first 64 KiB of body) | `429` → `cooling` (respects `Retry-After`); `401` and quota-403 → `invalid` | `max_retries` |
| `retryScopeConnection` | EOF mid-response, response-header timeout, generic timeouts where the dial succeeded | current key briefly `cooling` for `transient_cooling_seconds` | `max_transient_retries` |
| `retryScopeProvider` | DNS error, connection refused, TLS handshake failure, upstream `502/503/504` | **none** — key state is not poisoned by infrastructure failures | `max_transient_retries` |

Classification lives in `classifyTransportRetryScope` (transport errors) and `isRetryableUpstreamStatus` (upstream status codes). Other 4xx/5xx pass through unchanged.

### Package layout

```
admin/   REST API + embedded SPA mounting (handler.go, api.go, console.go, events.go)
config/  JSON load + validation + defaults + fsnotify file watcher (config.go, manager.go, watcher.go, writer.go)
pool/    ProviderPools registry → per-provider Pool → []*Key with round-robin cursor (key.go, pool.go, providers.go)
proxy/   Reverse proxy handler with atomic runtimeConfig and retry/stream logic (handler.go)
state/   Key-pool persistence; only stores SHA-256 hashes of keys, never raw values (state.go)
logx/    slog category/event field helpers + secret masking (logx.go)
web/     React 18 + Vite + Antd + react-query + react-router admin console; embed.go bundles dist/
```

### Concurrency invariants

- `pool.Key.state` is `atomic.Int32`; `coolUntil` and `last401At` are `atomic.Int64` (unix nanos). Counters (`ReqCount`, `ErrCount`, `totalLatencyMs`, in-flight) are `atomic.Int64`.
- Each `Pool`'s round-robin cursor is `atomic.Int64`; the key slice itself is guarded by `sync.RWMutex` because `Update()` swaps it atomically while preserving per-key state for keys that still exist.
- `proxy.Handler.runtime` is an `atomic.Value` holding `*runtimeConfig`. Reloads build a new snapshot and `Store()` it; readers always `Load()` once at the start of a request.
- `ProviderPools` uses `sync.RWMutex` to protect the `providers` map and `order` slice.

### Hot-reload model

Config is watched via fsnotify on the **directory** containing the config file (handles editor atomic-rename saves). `POST /admin/reload` triggers the same path. Reloads validate first and keep the previous runtime config on failure.

Hot-reloadable: `active_provider`, `providers[]` (and their `keys` / `target_url`), `cooling_seconds`, `max_retries`, `max_transient_retries`, `request_timeout_seconds`, `connect_timeout_seconds`, `response_header_timeout_seconds`, `transient_cooling_seconds`, `wait_for_key_timeout_ms`, `max_body_bytes`.

Restart-required: `listen`, `admin_listen`, all `log_*` fields.

### Config schema notes

- Preferred shape: top-level `active_provider` + `providers[]` with `id`, `target_url`, `keys`. See `config.example.json`.
- Legacy single-provider shape (`target_url` + `keys` at top level) is still accepted and silently mapped to a provider with id `default` (`config.DefaultProviderID`).
- `persist_state` is `*bool` — `nil` defaults to enabled. Use `cfg.StatePersistenceEnabled()`, not `cfg.PersistState != nil && *cfg.PersistState`.
- `invalid_ttl_hours` (default 24): on startup, persisted `invalid` keys older than this TTL are restored to `active`. This is the only automatic recovery path for invalid keys — within a running process, invalid is sticky until the key list is updated.

## Key design decisions worth respecting

- **Logging**: always use `log/slog` via `logx.Fields(category, event, ...)`. Never log raw API keys — use `logx.MaskSecret(value)` or the SHA-256 `state.KeyID(value)` derived identifier.
- **State persistence is hash-only**: `state.json` stores `state.KeyID` (SHA-256 of the key) plus state and counters. Adding fields that could leak the raw key would break the security model.
- **Retries are intra-provider only**: ModelMux does **not** auto-failover to a different provider when the active one is exhausted. Provider switching is an explicit `active_provider` config change. Don't add cross-provider fallback without an explicit ask.
- **Provider-level transients don't poison keys**: DNS / connection-refused / TLS / 5xx gateway codes consume `max_transient_retries` but leave key state untouched. Don't "simplify" this by treating all transport errors as key failures — it would invalidate keys during upstream outages.
- **Dependencies stay minimal**: only `fsnotify` and `lumberjack` are direct third-party deps. Justify additions.
- **Default-deny on admin exposure**: `admin_listen` defaults to `127.0.0.1:18081`. Don't change defaults that would expose it on `0.0.0.0`.

## Files to never commit

`config.json`, `state.json`, anything in `logs/`, and the build artifact `modelmux.exe` (already in `.gitignore`, but worth re-checking on new files that might leak keys).
