# Repository Guidelines

## Project Structure & Module Organization

ModelMux is a Go reverse proxy with an embedded React/Vite admin console. Core backend entrypoint is `main.go`; packages are split by responsibility: `proxy/` for forwarding and retries, `pool/` for provider key rotation, `config/` for load/reload/write paths, `admin/` for REST and console mounting, `state/` for hash-only key persistence, `stats/` for JSONL call metrics, and `logx/` for structured logging helpers. Frontend source lives in `web/src/`; built assets in `web/dist/` are embedded by `web/embed.go`.

## Build, Test, and Development Commands

Use PowerShell-friendly commands:

```powershell
go build -trimpath -ldflags="-s -w" -o modelmux.exe .
go test ./...
go vet ./...
go test -race ./...
.\start.ps1
```

`go build` creates the local binary, `go test ./...` runs backend tests, `go vet ./...` checks static diagnostics, and `go test -race ./...` verifies concurrency-sensitive code. `.\start.ps1` builds if needed, starts the service, and opens `/console/`. For frontend changes, run `cd web; npm install; npm run build` so `web/dist/` matches `web/src/`.

## Coding Style & Naming Conventions

Prefer standard-library Go patterns and keep dependencies minimal. Run `gofmt` on Go edits. Use package-local tests with `*_test.go` names and clear `TestXxx` functions. Logging must use `log/slog` via `logx.Fields(...)`; never log raw API keys. Preserve existing concurrency patterns: atomics for key counters/state, mutexes around mutable maps/slices, and immutable proxy runtime snapshots.

## Testing Guidelines

Backend tests use Go’s standard `testing` package, `httptest`, temp directories, and table-driven cases where helpful. Add or update tests when changing retry behavior, config validation, admin APIs, state persistence, stats aggregation, or frontend embedding. Before handoff for backend changes, run `go test ./...`, `go vet ./...`, and `go test -race ./...`; for frontend changes, run `npm run build` in `web/`.

## Commit & Pull Request Guidelines

Recent history uses short conventional prefixes such as `feat:`, `fix:`, `docs:`, `refactor:`, and `chore:`. Keep commit subjects concise and explain the user-visible reason. Pull requests should summarize changes, list validation commands run, note config or security impacts, and include screenshots for console UI updates.

## Security & Configuration Tips

Do not commit `config.json`, `state.json`, `logs/`, `stats_data/`, `modelmux.exe`, or `web/node_modules/`. State persistence must remain hash-only; use `state.KeyID` or `logx.MaskSecret` for key identifiers. Admin defaults should stay loopback-only, and provider key pools must remain isolated unless explicitly requested.
