# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`ModelMux` is a Go reverse proxy for Claude / Anthropic-compatible APIs. It exposes a local proxy endpoint, stores multiple upstream providers, and uses the configured `active_provider` at runtime. Each provider owns an independent API key pool; keys rotate only within the active provider, never across providers.

Repository: `github.com/kelongyan/ModelMux`.

The repository has been renamed to `ModelMux`, but the current Go module path, imports, binary name, and startup scripts still use `claude-key-proxy` to avoid unnecessary runtime changes. Do not rename module/import/binary paths unless explicitly requested as a code-level rename. The binary is intended for local personal use. The default proxy listener is `:8080`, and the default admin listener is `127.0.0.1:8081`.

## Common Commands

```powershell
# Build the stripped local binary
make build

# Build and run with config.json
make run

# Run all tests
make test

# Equivalent direct Go commands
go build -trimpath -ldflags="-s -w" -o claude-key-proxy.exe .
go test ./...
go vet ./...
go test -race ./...
```

Run a single package or test:

```powershell
go test ./pool -run TestRoundRobin -v
go test ./proxy -run TestHandlerRetriesOnRateLimit -v
```

Cross-compilation targets:

```powershell
make build-linux
make build-windows
make build-mac
```

Prepare a local config from the example:

```powershell
Copy-Item config.example.json config.json
```

Start the built binary directly:

```powershell
.\claude-key-proxy.exe -config config.json
```

Useful admin checks while the app is running:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/health
Invoke-RestMethod http://127.0.0.1:8081/admin/status
Invoke-RestMethod -Method Post http://127.0.0.1:8081/admin/reload
```

## Architecture

- `main.go` is the composition root. It loads config, initializes logging, creates provider key pools, restores optional persisted key state, starts the proxy and admin HTTP servers, wires config reload, starts fsnotify config watching, and performs graceful shutdown.
- `config/` owns JSON config loading, validation, defaults, legacy single-provider compatibility, and fsnotify-based hot reload. The preferred schema is `active_provider` plus `providers[]` entries containing `id`, `target_url`, and `keys`.
- `pool/` owns provider isolation and per-provider key state. `ProviderPools` maps provider IDs to independent `Pool` instances. Each `Pool` performs round-robin selection over available keys and preserves key state/statistics across config updates when key values remain present.
- `proxy/` owns request forwarding. `Handler` stores an immutable runtime snapshot in `atomic.Value`; config reloads replace the snapshot for new requests while in-flight requests keep the old one. Requests are retried only for `429` and `401`, and only within the active provider's pool.
- `admin/` exposes the local control plane: `GET /admin/health`, `GET /admin/status`, and `POST /admin/reload`.
- `state/` persists provider/key runtime state as JSON. It stores SHA-256 key identifiers, not full API keys. Version 2 stores state grouped by provider, with compatibility for older ungrouped state.
- `logx/` centralizes slog `category`/`event` fields and key masking. Logging should use masked key IDs rather than full secrets.

## Runtime Behavior to Preserve

- Provider failover is manual: the proxy does not automatically switch providers when the active provider has no available keys. Change `active_provider` and reload config to switch providers.
- `429` marks a key as cooling. The proxy honors `Retry-After` when present; otherwise it uses `cooling_seconds`.
- `401` marks a key invalid. During the same process, invalid keys do not auto-recover. With state persistence enabled, invalid keys older than `invalid_ttl_hours` recover on next startup.
- Streaming/SSE responses are forwarded in small chunks and flushed after each write. Avoid adding response buffering in `streamBody()` or related proxy paths.
- The proxy reads the full request body into memory to support retries, bounded by `max_body_bytes`.
- Hop-by-hop headers are stripped when copying request and response headers.
- Runtime hot reload currently applies provider selection, provider URLs/keys, cooling/retry/timeouts, and max body size. Listener addresses and log settings require restart.

## Configuration Notes

`config.example.json` is the reference schema. Important fields include:

- `listen`, `admin_listen`
- `active_provider`
- `providers[].id`, `providers[].target_url`, `providers[].keys`
- `cooling_seconds`, `max_retries`, `request_timeout_seconds`, `max_body_bytes`
- `log_level`, `log_format`, `log_output`, `log_file`, log rotation settings
- `persist_state`, `state_file`, `invalid_ttl_hours`

Legacy top-level `target_url` plus `keys` is still accepted and normalized as provider `default`.

## Dependencies

Most logic uses the Go standard library. Current direct third-party dependencies are:

- `github.com/fsnotify/fsnotify` for config file watching
- `gopkg.in/natefinch/lumberjack.v2` for log rotation

## Testing Focus

Existing tests cover config defaults/validation/watching, provider pool isolation, key state transitions and concurrency, proxy auth rewriting/retry/body limits/runtime config updates/provider switching, admin health/status behavior, state persistence, and logging helpers. Before handoff after code changes, prefer:

```powershell
go test ./...
go vet ./...
go test -race ./...
```
