# Spike Conventions

Patterns and stack choices established across spike sessions. New spikes follow these unless the question requires otherwise.

## Stack

- **Language:** Go 1.24+
- **DNS library:** `github.com/miekg/dns` — the project's DNS server uses this for both server and client resolution
- **BGP library:** `github.com/osrg/gobgp/v3` — BGP server implementation
- **CLI framework:** `github.com/spf13/cobra` — for bgp-dnsctl commands
- **Cache library:** `github.com/bluele/gcache` — LFU cache with eviction callbacks
- **File watcher:** `github.com/fsnotify/fsnotify` — for domainlist file change detection
- **Logging:** `github.com/rs/zerolog` — structured logging

## Structure

- Spike experiments are self-contained Go modules in `.planning/spikes/NNN-name/` with their own `go.mod`
- Spike experiments use the project's `miekg/dns` dependency (copied via `go mod edit -require`)
- Each spike directory contains: `main.go`, `README.md`, `go.mod`, `go.sum`
- Spike ports are hardcoded (e.g., `127.0.0.1:5553`) — no config files

## Patterns

- **Bias toward interactive demos:** Build something the user can run and verify, not just stdout output
- **Self-verifying tests:** Spike main.go includes automated test queries at the end
- **Regex compilation at load time:** Never compile regex per-query; compile once during domainlist loading
- **Priority-based routing:** When multiple handler types coexist (exact, wildcard, regex, catch-all), use explicit priority ordering

## Tools & Libraries

- `github.com/miekg/dns@v1.1.62` — DNS server/client library (no native regex support, requires custom wrapper)
- `regexp` (stdlib) — Go's standard regex package, pre-compile with `regexp.Compile()` at load time
- `sync.RWMutex` — for thread-safe handler registration and query routing
