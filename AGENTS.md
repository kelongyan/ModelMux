# AGENTS.md

## Repository intent

- ModelMux is a single-binary local Go reverse proxy for model APIs. Live traffic always uses exactly one provider: `active_provider`.
- Key rotation is isolated per provider. Do not add cross-provider failover unless explicitly requested; the current design treats provider switching as an explicit config change.
- The admin console is a React/Vite SPA embedded into the Go binary from `web/dist` via `web/embed.go`.
- Go module path: `github.com/kelongyan/ModelMux`.
- `go.mod` declares Go `1.26.3`.

## Working environment

- The repository is worked on primarily from Windows/PowerShell. `start.ps1` is the main convenience entrypoint.
- No `.cursor` rules, `.cursorrules`, Copilot instructions, or GitHub Actions workflows were found in-repo.
- Direct Go dependencies are intentionally small: `fsnotify` for config watching and `lumberjack` for file log rotation.

## Commands

```powershell
# Prepare local config
Copy-Item config.example.json config.json

# Build
Go build -trimpath -ldflags="-s -w" -o modelmux.exe .

# Run directly
.\modelmux.exe -config config.json

# One-shot start (auto-builds if needed, backgrounds the process, opens /console/)
.\start.ps1

# Verification
Go test ./...
Go vet ./...
Go test -race ./...

# Make targets
make build
make test
make run
make build-linux
make build-windows
make build-mac

# Frontend
cd web
npm install
npm run build
npm run dev
npm run preview
```

- `make clean` runs `rm -f`, so it is the least Windows-friendly target; prefer a native delete when cleaning `modelmux.exe`.
- `make run` expects `config.json` in the repo root and runs `./modelmux.exe -config config.json` after building.
- `start.ps1` writes PID/stdout/stderr files under `logs/`, polls `/admin/health`, and opens `/console/` in the default browser.
- There is no CI workflow checked into this repository; the observable verification path is local test/vet/race execution plus `npm run build` for frontend work.

## Runtime architecture

### Process layout

- `main.go` starts two HTTP servers in one process:
  - proxy server on `listen` (default `127.0.0.1:18080`)
  - admin server on `admin_listen` (default `127.0.0.1:18081`)
- The admin server exposes both legacy endpoints (`/admin/health`, `/admin/status`, `/admin/reload`) and the REST API under `/admin/api/v1/*`.
- The SPA is mounted at `/console/`; `/console` redirects to `/console/`.

### Request flow

1. Config is loaded, normalized, and converted into per-provider `pool.ProviderPools`.
2. `proxy.Handler` stores an immutable runtime snapshot in `atomic.Value`; each request loads that snapshot once, so in-flight requests keep the old config during reload.
3. The proxy reads the full request body into memory with `max_body_bytes` so the same payload can be retried against another key.
4. A key is selected from the active provider pool using in-flight-aware round robin.
5. Upstream request construction rewrites `Authorization` and `X-Api-Key` with the selected real key and strips client forwarding headers such as `X-Forwarded-For` and `X-Real-Ip`.
6. Response handling classifies failures into retry scopes that determine whether the key is cooled, invalidated, or left untouched.
7. State changes trigger debounced persistence to `state.json`; call stats append to daily JSONL files in `stats_data`; diagnostic events feed the admin event stream.

### Retry model

The proxy has three distinct retry scopes in `proxy/handler.go`:

- `retryScopeKey`: upstream `401`, `429`, or quota/credit/balance-type `403`
  - `429` cools the key and honors `Retry-After` when present.
  - `401` and quota-style `403` mark the key invalid.
- `retryScopeConnection`: connection-level transient failures such as response-header timeouts or mid-stream failures
  - the current key gets a short cooling period (`transient_cooling_seconds`).
- `retryScopeProvider`: provider/infrastructure failures such as DNS, connection refused, TLS handshake, or upstream `502/503/504`
  - the key state must not be poisoned.

If no key is currently available but the pool only has cooling keys, the proxy can wait briefly for recovery using `wait_for_key_timeout_ms` before returning `503`.

## Package map

- `main.go`: process wiring, logger setup, state saver, config reload orchestration, proxy/admin server startup and shutdown.
- `admin/`: admin HTTP handlers, REST API, event buffer, stats endpoints, embedded console mount.
- `config/`: JSON load/validation/defaults, global current-config snapshot, fsnotify watcher, atomic file writes, config diffing, update/rollback logic.
- `pool/`: provider registry and per-provider key pools, stateful key rotation, pool snapshots/restores.
- `proxy/`: request forwarding, auth rewrite, retry classification, streaming copy, tool-field stripping, call stats extraction.
- `state/`: hash-only key-state persistence (`state.KeyID` is SHA-256, raw keys are never persisted).
- `stats/`: daily JSONL call log store plus summary/model/recent queries.
- `logx/`: shared log category/event names, structured field helpers, secret masking.
- `web/`: embedded frontend assets and SPA file serving with `/console/` base path.
- `web/src/`: React 18 + Vite + Ant Design + TanStack Query console.

## Config and reload conventions

- Preferred config shape is top-level `active_provider` plus `providers[]`.
- Legacy top-level `target_url` + `keys` is still accepted and normalized to a provider with ID `default`.
- `config.Load()` reads, validates, applies defaults, and updates the global current snapshot.
- `config.Read()` reads and normalizes config without replacing the global current snapshot. Use it for preview/validation paths.
- `config.Manager.Update()` is the safe mutation path for admin writes: clone current config, mutate, validate, atomically write, reload, and roll back on reload failure.
- `main.go` adds an extra `reloadMu` to serialize all reload paths so fsnotify-triggered reloads and admin-triggered reloads cannot interleave runtime swaps.
- The watcher observes the directory containing the config file, not just the file itself, so editor atomic-rename saves are handled. It debounces for 300 ms.

### Hot-reload boundaries

Observed as hot-reloadable in `config/manager.go`:

- `active_provider`
- `providers`
- `cooling_seconds`
- `max_retries`
- `max_transient_retries`
- `request_timeout_seconds`
- `connect_timeout_seconds`
- `response_header_timeout_seconds`
- `transient_cooling_seconds`
- `wait_for_key_timeout_ms`
- `max_body_bytes`

Observed as restart-required:

- `listen`
- `admin_listen`
- `log_level`
- `log_format`
- `log_output`
- `log_file`
- `log_max_size_mb`
- `log_max_backups`
- `log_max_age_days`
- `log_compress`
- `persist_state`
- `state_file`
- `invalid_ttl_hours`
- `stats_enabled`
- `stats_dir`
- `stats_retention_days`
- `stats_max_recent_records`

## Non-obvious invariants and gotchas

- `persist_state` and `stats_enabled` are `*bool` fields. Nil means enabled. Use `StatePersistenceEnabled()` and `StatsCollectionEnabled()` instead of dereferencing directly.
- When `log_output` is omitted but `log_file` is set, defaults promote output to `both`; otherwise the default is `stdout`.
- `pool.Pool.Update()` preserves existing key objects by raw key value, which means counters/state survive config edits when the same key string stays in the list.
- `invalid` keys do not recover automatically during a running process. They only recover by manual reset, replacing the key list, or after a restart when `invalid_ttl_hours` has expired.
- `state.json` stores only hashed key IDs and counters; adding raw key material there would violate the current security model.
- Logs and admin key summaries must use masking/hashes (`logx.MaskSecret`, `state.KeyID`) rather than raw API keys.
- Proxy streaming is intentionally unbuffered: request handling and server timeouts are tuned so SSE can run for minutes. Do not add a fixed proxy write timeout or buffering layer that delays flushes.
- The quota detector for `403` responses is string-based over the response body; changing those rules changes retry semantics, not just logging.
- `strip_tools` is a per-provider config field. When enabled, outgoing JSON bodies have `tools`, `tool_choice`, `parallel_tool_calls`, `functions`, and `function_call` removed before forwarding.
- `readRequestBody()` intentionally loads the full body into memory for retryability; large-request behavior is controlled by `max_body_bytes`, not streaming upload passthrough.
- `stats_data/` is operational data, not source. The `stats/` package is code; `stats_data/` is the default runtime directory.
- `stats.Store` writes one file per UTC day (`calls-YYYY-MM-DD.jsonl`), keeps an in-memory recent slice capped by `stats_max_recent_records`, and prunes old files by retention days.
- `stateSaver.Trigger(false)` is a non-resetting debounce window: repeated high-frequency state changes should still eventually flush instead of starving forever.
- `start.ps1` considers the service "already running" if `/admin/health` returns any HTTP response, not only `200`.

## Frontend notes

- Vite `base` is `/console/`, and the React router uses `BrowserRouter` with a basename derived from `import.meta.env.BASE_URL`.
- Local frontend development proxies `/admin` to `http://127.0.0.1:18081`.
- `web/embed.go` serves the built SPA with history fallback: requests under `/console/` that are not real static files fall back to `index.html`.
- `web/dist/` is embedded and tracked. If you change `web/src`, rebuild `web/dist` before handoff or the binary will ship stale UI assets.
- The frontend does not have a dedicated test suite in the repo; `npm run build` is the available verification step.

## Testing patterns

- Tests are colocated with packages and rely heavily on the standard library: `httptest`, temporary directories, and explicit config objects.
- `proxy/handler_test.go` uses `mustHandler()` to build a handler from a config and exercises retry/auth-rewrite behavior with `httptest.NewServer`.
- `admin/handler_test.go` typically builds a handler, registers routes on a fresh `http.ServeMux`, and asserts JSON payloads or config-file side effects.
- `config/manager_test.go` uses temp config files and `mustNormalizedConfig()` so tests match production defaulting/validation behavior.
- `stats/*_test.go` injects `Options.Now` to make time-based retention and aggregation deterministic.
- For backend changes, the repo’s own guidance is to run `go test ./...`, `go vet ./...`, and `go test -race ./...` before handoff.

## Files and artifacts to keep out of commits

- `config.json`
- `key.txt`
- `state.json` and `state.json.tmp`
- anything under `logs/`
- anything under `stats_data/`
- `modelmux.exe` and other built `modelmux-*` binaries
- `web/node_modules/`
- `web/.vite/`
- generated `web/*.tsbuildinfo`
