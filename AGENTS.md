# AGENTS.md

## Project

ModelMux is a Go reverse proxy for model APIs. Clients hit one local proxy; the proxy uses `active_provider` and rotates keys only inside that provider's isolated key pool. It intentionally does not auto-failover across providers.

Module: `github.com/kelongyan/ModelMux`. Single binary; the React/Vite admin console is embedded from `web/dist` via `web/embed.go`.

## Commands

PowerShell is the default local shell.

```powershell
go test ./...
go vet ./...
go test -race ./...
go build -trimpath -ldflags="-s -w" -o modelmux.exe .
```

```powershell
go test ./pool/ -run TestRoundRobin -v
```

```powershell
make build    # stripped Go build to modelmux.exe
make test     # go test ./...
make run      # build, then ./modelmux.exe -config config.json
```

`make clean` uses POSIX `rm`; prefer `Remove-Item -LiteralPath modelmux.exe` on Windows if needed.

Frontend console:

```powershell
Set-Location web
npm ci
npm run build
npm run dev
```

`npm run build` runs `tsc -b && vite build` and writes `web/dist`; Go builds depend on that embedded directory. If `web/` changes, rebuild `web/dist` before handoff.

## Runtime Shape

- `main.go` starts two servers: proxy `cfg.Listen` default `:8080`, admin `cfg.AdminListen` default `127.0.0.1:8081`.
- Proxy server has no fixed write timeout because SSE streams may run for minutes.
- Admin server serves legacy endpoints `/admin/health`, `/admin/status`, `/admin/reload`, REST endpoints under `/admin/api/v1/*`, and SPA `/console/`.
- `start.ps1` requires `config.json`, auto-builds `modelmux.exe` if missing, starts it in the background, writes logs/PID under `logs/`, and opens `/console/`.

## Package Boundaries

- `admin/`: management API, event buffer, config/state backup, embedded console mount.
- `config/`: JSON load/validate/defaults, fsnotify watcher, config manager with atomic write + rollback.
- `pool/`: `ProviderPools` registry and per-provider key pools with in-flight-aware round-robin.
- `proxy/`: reverse proxy, auth header rewrite, retry classification, request body limit, streaming copy.
- `state/`: key-pool persistence. Stores `state.KeyID` SHA-256 hashes, never raw keys.
- `logx/`: `slog` helpers, category/event fields, secret masking.
- `web/`: React 18 + Vite + Antd + TanStack Query admin console.

## Config Rules

- Config is JSON. Preferred schema is `active_provider` plus `providers[]` with `id`, `target_url`, `keys`.
- Legacy top-level `target_url` + `keys` is still accepted and normalized to provider ID `default`.
- `admin_listen` defaults to loopback; do not expose it by changing defaults unless explicitly requested.
- `persist_state` is a `*bool`; use `cfg.StatePersistenceEnabled()` instead of checking the pointer directly.
- `config.Manager.Update` writes via temp file and rolls back if reload fails; preserve this behavior for admin writes.

Hot-reloadable fields: `active_provider`, `providers`, `cooling_seconds`, `max_retries`, `max_transient_retries`, `request_timeout_seconds`, `connect_timeout_seconds`, `response_header_timeout_seconds`, `transient_cooling_seconds`, `wait_for_key_timeout_ms`, `max_body_bytes`.

Restart-required fields: `listen`, `admin_listen`, all `log_*` fields, `persist_state`, `state_file`, `invalid_ttl_hours`.

## Proxy Invariants

- Runtime config is an atomic snapshot; reloads affect new requests while in-flight requests keep their old provider URL, pool, and client.
- Request bodies are read into memory for retries and capped by `max_body_bytes`.
- Upstream requests overwrite `Authorization: Bearer <key>` and `X-Api-Key: <key>`; client-supplied auth is not forwarded.
- Hop-by-hop headers and client IP forwarding headers are stripped from upstream requests/responses.
- `streamBody()` copies 4 KiB chunks and flushes after every write; do not add buffering that would break SSE.
- If every key is only cooling, proxy may wait up to `wait_for_key_timeout_ms` for the soonest key before returning 503.

## Retry And Key State

- Key-level failures use `max_retries`: `429` -> cooling with `Retry-After` support; `401` and quota/credit/balance `403` -> invalid.
- Connection-level transient failures use `max_transient_retries` and briefly cool the current key for `transient_cooling_seconds`.
- Provider-level transient failures use `max_transient_retries` but do not change key state: DNS, connection refused, TLS handshake, upstream `502/503/504`.
- Invalid keys are sticky during a process lifetime except manual reset/API/config key-list changes. On startup, persisted invalid keys older than `invalid_ttl_hours` recover.
- Do not add cross-provider fallback unless explicitly asked; provider switching is only via `active_provider`.

## Logging And Secrets

- Use `log/slog` with `logx.Fields(category, event, ...)`.
- Never log raw API keys. Use `logx.MaskSecret(value)` for logs or `state.KeyID(value)` for stable identifiers.
- Do not commit `config.json`, `state.json`, `logs/`, `web/node_modules/`, or build artifacts such as `modelmux.exe`; `.gitignore` already covers them.

## Existing Guidance

`CLAUDE.md` has a more verbose architecture walkthrough. Prefer executable sources (`go.mod`, `Makefile`, `web/package.json`, source/tests) when it conflicts with prose.
