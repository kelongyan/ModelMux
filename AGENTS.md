# AGENTS.md

## Project

Go reverse proxy that rotates multiple API keys against a single upstream (Anthropic-compatible) provider. Single binary, no external dependencies — pure stdlib only (`net/http`, `log/slog`, `encoding/json`). Module: `github.com/claude-key-proxy`.

## Commands

```
make build          # builds claude-key-proxy binary (stripped, -trimpath)
make test           # go test ./...
make run            # build + run with config.json
```

Run a single test:
```
go test ./pool/ -run TestRoundRobin -v
```

Cross-compile: `make build-linux`, `make build-windows`, `make build-mac`.

## Architecture

- `main.go` — entrypoint, starts two HTTP servers: proxy (configurable port, default `:8080`) and admin (`:8081`)
- `config/` — JSON config loading, validation, hot-reload via `config.Get()` singleton
- `pool/` — key pool with round-robin selection. Keys have three states: active, cooling (429), invalid (401). Cooling keys auto-recover when their timer expires.
- `proxy/` — reverse proxy handler. Retries on 429 (cooling + switch key) and 401 (mark invalid + switch key). Streams SSE responses without buffering.
- `admin/` — `/admin/status`, `/admin/health` (GET), `/admin/reload` (POST, hot-reloads config file)

## Key Design Decisions

- **No external dependencies.** Do not add third-party packages. The `go.mod` has zero `require` directives — keep it that way.
- **Config is JSON, not YAML/TOML.** See `config.example.json` for schema. Required fields: `target_url`, `keys` (at least one).
- **Concurrency:** `pool.Key` state uses `atomic.Int32` + `atomic.Int64` (for cool-until nanos). The pool's cursor is `atomic.Int64`. `sync.RWMutex` protects the keys slice during `Update()`.
- **Retry logic:** only 429 and 401 trigger retries (up to `max_retries`). Other errors are passed through to the caller. 429 respects `Retry-After` header.
- **Stream proxying:** `streamBody()` reads in 4KB chunks and calls `Flush()` after each write. Do not introduce buffering that would break SSE.
- **Hop-by-hop headers** are stripped from proxied requests/responses (`Connection`, `Transfer-Encoding`, etc.).

## Testing

Only `pool/pool_test.go` exists. Tests cover round-robin, cooling skip, invalid skip, auto-recovery, `Update()` key preservation, and concurrent access. No integration or proxy-level tests yet.

## Config Hot-Reload

`POST /admin/reload` calls `config.Reload()` which re-reads the config file and calls `pool.Update()`. Existing keys that appear in the new config preserve their state/stats. Removed keys are dropped. New keys start as active.

## Language Note

`DESIGN.md` is written in Chinese and contains the full design spec. It is the authoritative design document for this project.
