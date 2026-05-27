# AGENTS.md

## Project

- ModelMux is a Go reverse proxy for model APIs. Clients hit one local proxy; `active_provider` selects the only provider used for key rotation. Do not add cross-provider fallback unless explicitly requested.
- Module: `github.com/kelongyan/ModelMux`. The React/Vite admin console is embedded from `web/dist` by `web/embed.go`.

## Commands

- PowerShell is the local shell. Use `go test ./...`, `go vet ./...`, `go test -race ./...`, and `go build -trimpath -ldflags="-s -w" -o modelmux.exe .` for Go verification/builds.
- Focused test example: `go test ./pool/ -run TestRoundRobin -v`.
- Make shortcuts: `make build`, `make test`, `make run`; `make clean` uses POSIX `rm`, so prefer `Remove-Item -LiteralPath modelmux.exe` on Windows.
- Frontend: `Set-Location web`, `npm ci`, `npm run build`, `npm run dev`. If `web/` changes, rebuild `web/dist` before handoff because Go embeds it.

## Runtime

- Defaults are local-only: proxy `127.0.0.1:18080`, admin `127.0.0.1:18081`, console `http://127.0.0.1:18081/console/`.
- Client Base URL should normally be `http://127.0.0.1:18080/v1`; the client API key can be any non-empty value because proxy requests overwrite upstream auth.
- `start.ps1` requires `config.json`, auto-builds `modelmux.exe` if missing, starts it in the background, writes PID/logs under `logs/`, and opens `/console/`.
- Proxy server intentionally has no fixed write timeout; SSE streams can run for minutes.

## Boundaries

- `admin/`: management API, events, config/state backup, embedded console mount.
- `config/`: JSON load/validate/defaults, fsnotify watcher, atomic config write with rollback.
- `pool/`: provider registry and per-provider in-flight-aware round-robin key pools.
- `proxy/`: reverse proxy, auth rewrite, retry classification, request-body cap, streaming copy.
- `state/`: key state persistence using `state.KeyID` hashes only; never store raw keys.
- `web/`: React 18 + Vite + Antd + TanStack Query admin console.

## Invariants

- Config schema is `active_provider` plus `providers[]`; legacy top-level `target_url` + `keys` is still normalized to provider ID `default`.
- Hot reload affects new requests only. In-flight requests keep their old provider URL, pool, and HTTP client snapshot.
- `config.Manager.Update` writes via temp file and rolls back if reload fails; preserve that behavior for admin writes.
- Use `cfg.StatePersistenceEnabled()` instead of reading `cfg.PersistState` directly; nil means enabled.
- Upstream requests overwrite `Authorization: Bearer <key>` and `X-Api-Key: <key>`; client-supplied auth must not pass through.
- `streamBody()` copies 4 KiB chunks and flushes after every write. Do not add buffering that would break SSE.
- Key failures: `429` cools with `Retry-After`; `401` and quota/credit/balance `403` mark invalid. Provider transients like DNS, TLS, connection refused, and upstream `502/503/504` do not poison keys.
- Never log raw API keys. Use `logx.MaskSecret(value)` for logs or `state.KeyID(value)` for stable identifiers.
- Do not commit `config.json`, `state.json`, `logs/`, `web/node_modules/`, or build artifacts such as `modelmux.exe`.
