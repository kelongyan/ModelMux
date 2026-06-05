# Repository Notes

## Commands

- PowerShell is the expected shell. Use `go build -trimpath -ldflags="-s -w" -o modelmux.exe .` for the local binary.
- Backend verification: `go test ./...`, then `go vet ./...`, then `go test -race ./...` before handoff for backend/concurrency changes.
- Focused Go test: `go test ./proxy -run TestServeHTTPUsesOnlyActiveProvider -v` or swap package/name as needed.
- Frontend install/build: `Push-Location web; npm ci; npm run build; Pop-Location`. `npm run build` runs `tsc -b` before Vite.
- Frontend dev server: `Push-Location web; npm run dev; Pop-Location`; Vite serves on `127.0.0.1:5173` and proxies `/admin` to `127.0.0.1:18081`.
- For frontend-only refactors/cleanup, `Push-Location web; npm run build; Pop-Location` is enough verification; do not rebuild/restart the Go service unless the user asks to serve the embedded UI.
- After console/UI changes, use the handoff flow `Push-Location web; npm run build; Pop-Location`, then `go build -trimpath -ldflags="-s -w" -o modelmux.exe .`, then restart the running service so `/console/` serves the new embedded assets.
- For automated restarts, do not use `.\start.ps1`; it may hang after opening the console. Stop the existing `modelmux.exe`, then start with `Start-Process -FilePath .\modelmux.exe -ArgumentList @("-config", "config.json") -WorkingDirectory (Get-Location) -WindowStyle Hidden` and poll `/admin/health`.
- `.\start.ps1` requires `config.json`, builds `modelmux.exe` only if missing, starts it in the background, writes logs/PID under `logs/`, and opens `/console/`.

## Architecture

- `main.go` starts two HTTP servers: proxy on `listen` default `127.0.0.1:18080`, admin/API/SPA on `admin_listen` default `127.0.0.1:18081`.
- The admin console is React/Vite in `web/src`; production assets in `web/dist` are embedded by `web/embed.go` with `//go:embed all:dist` and served at `/console/`.
- After any `web/src` or Vite change, rebuild `web/dist` before Go build/release or the binary will ship stale UI.
- Package boundaries: `proxy/` forwards/retries/streams, `pool/` owns provider/key rotation and state, `config/` loads/watches/writes JSON config, `admin/` registers REST plus console, `state/` persists key state, `stats/` stores/query JSONL call records, `logx/` centralizes slog fields and secret masking.

## Runtime Invariants

- Provider key pools are isolated. Requests use only `active_provider`; do not add cross-provider failover unless explicitly asked.
- `proxy.Handler.runtime` is an immutable snapshot in `atomic.Value`; reloads must build/validate the next snapshot and keep old in-flight requests on the old snapshot.
- Streaming responses are copied in chunks and flushed; do not add response buffering or fixed proxy server `WriteTimeout`, because SSE can run for minutes.
- Retry scopes are intentionally separate: 401/429/quota-403 affect keys, connection-level errors briefly cool keys, provider/DNS/TLS/502/503/504 failures must not poison keys.
- State persistence is hash-only. `state.json` stores `state.KeyID` values, never raw keys.

## Config And Admin

- Preferred config shape is top-level `active_provider` plus `providers[]`; legacy top-level `target_url` + `keys` still maps to provider ID `default`.
- `persist_state` and `stats_enabled` are `*bool` fields where nil means enabled; use `StatePersistenceEnabled()` and `StatsCollectionEnabled()`.
- Hot reload handles `active_provider`, `providers`, retry/timeout/body-limit fields. `listen`, `admin_listen`, log settings, persistence settings, and stats settings require restart.
- Admin endpoints include legacy `/admin/health`, `/admin/status`, `/admin/reload` plus `/admin/api/v1/*` for dashboard, providers, settings, events, about, reload, backups, and stats including `/stats/logs`.

## Security And Files

- Never log raw API keys. Use `logx.MaskSecret(value)` for display or `state.KeyID(value)` for stable identifiers.
- Keep admin loopback-only by default; do not change defaults to `0.0.0.0`.
- Do not commit `config.json`, `state.json`, `logs/`, `stats_data/`, `modelmux.exe`, generated `web/*.tsbuildinfo`, or `web/node_modules/`.
