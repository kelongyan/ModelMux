# AGENTS.md

## Project

ModelMux is a Go reverse proxy that centralizes multiple model providers and uses the selected `active_provider` at runtime. Each provider has an independent key pool; the proxy rotates keys only inside the active provider, not across providers. Single binary for local personal use.

Repository: `github.com/kelongyan/ModelMux`.

## Commands

```
make build          # builds modelmux binary (stripped, -trimpath)
make test           # go test ./...
make run            # build + run with config.json
```

Run a single test:

```
go test ./pool/ -run TestRoundRobin -v
```

Cross-compile: `make build-linux`, `make build-windows`, `make build-mac`.

## Architecture

- `main.go` — entrypoint, starts two HTTP servers: proxy (default `:8080`) and admin (default `127.0.0.1:8081`), initializes logging, config watching, and graceful shutdown.
- `config/` — JSON config loading, validation, defaults, hot-reload, and fsnotify-based config file watching.
- `pool/` — provider pool registry plus per-provider key pools with in-flight-aware round-robin selection. Keys have three states: active, cooling, invalid. Cooling keys auto-recover when their timer expires, and the pool can expose the soonest cooling key recovery window for short waits.
- `proxy/` — reverse proxy handler. Runtime config is stored as an atomic snapshot containing the active provider URL and pool, so reloads affect new requests while in-flight requests keep their old snapshot. Retries distinguish key-level failures (401/429/quota 403), connection-level transient failures, and provider-level transient failures inside the active provider only. Streams SSE responses without buffering.
- `admin/` — `/admin/status`, `/admin/health` (GET), `/admin/reload` (POST).
- `logx/` — structured logging helpers, category/event fields, and key masking.

## Key Design Decisions

- **Dependencies are allowed when useful.** Keep dependencies small and justified. Current direct third-party dependencies are `github.com/fsnotify/fsnotify` for config watching and `gopkg.in/natefinch/lumberjack.v2` for log rotation.
- **Config is JSON, not YAML/TOML.** See `config.example.json` for schema. Preferred schema uses `active_provider` plus `providers[]` with `id`, `target_url`, and `keys`. Legacy `target_url` + `keys` is still accepted as provider `default`.
- **Concurrency:** `pool.Key` state uses `atomic.Int32` + `atomic.Int64` (for cool-until nanos). Each key pool's cursor is `atomic.Int64`. `sync.RWMutex` protects provider maps and key slices during `Update()`. Proxy runtime config uses `atomic.Value`.
- **Retry logic:** key-level retries use `max_retries`; transient network/provider retries use `max_transient_retries`. `429` respects `Retry-After`; quota-exhausted `403` invalidates the key; connection-level transient failures can temporarily cool the current key; provider-level transient failures do not poison key state.
- **Availability smoothing:** when every key is only temporarily cooling, the proxy may wait up to `wait_for_key_timeout_ms` for the soonest key to recover before returning `503`.
- **Stream proxying:** `streamBody()` reads in 4KB chunks and calls `Flush()` after each write. Do not introduce buffering that would break SSE.
- **Hop-by-hop headers** are stripped from proxied requests/responses (`Connection`, `Transfer-Encoding`, etc.).
- **Logging:** use `log/slog` with `category` and `event` fields. Full API keys must not be logged; use masked key identifiers.

## Testing

Tests currently cover pool state transitions/concurrency, provider pool isolation, proxy auth rewriting/retry/body limit/runtime config update/provider switching, admin active-provider health, config defaults/validation/watcher behavior, state persistence, and log helpers/writer setup.

Before handoff, prefer running:

```
go test ./...
go vet ./...
go test -race ./...
```

## Config Hot-Reload

Config file changes are watched automatically via `fsnotify`, and `POST /admin/reload` triggers the same reload path manually.

Hot-reloaded fields:

- `active_provider`
- `providers`
- `providers[].keys`
- `providers[].target_url`
- `cooling_seconds`
- `max_retries`
- `max_transient_retries`
- `request_timeout_seconds`
- `connect_timeout_seconds`
- `response_header_timeout_seconds`
- `transient_cooling_seconds`
- `wait_for_key_timeout_ms`
- `max_body_bytes`

Restart-required fields:

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
