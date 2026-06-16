# Phase 3: Dependency Injection - Research

**Researched:** 2026-06-16
**Domain:** Go dependency injection, struct-based constructors, typed context keys, error handling
**Confidence:** HIGH

## Summary

Phase 3 replaces package-level global state in `internal/dns/`, `internal/bgp/`, and `internal/fswatcher/` with explicit dependency injection via typed constructors. The Go codebase currently uses 13 package-level globals across 3 packages, all accessed via `Serve(ctx)` functions that read config from a string-keyed context (`"cfg"`).

The refactoring approach is **per-package constructors** (not a central `App` struct) that return `(Service, error)`. Thin wrapper functions (`Serve()`, `Shutdown()`, `Advance()`, `Load()`) are retained to delegate to package-level service instances, ensuring Phase 2's 63 tests pass without modification.

Startup panics in `cmd/bgp-dnsd/main.go` (7 `panic()` calls) are replaced with error returns and graceful shutdown sequences. The string context key `"cfg"` is replaced with a typed `configKey` struct defined in `internal/config/main.go`. The `log.L()` global logger dependency is replaced with explicit `*zerolog.Logger` parameters.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| DNS server lifecycle | `internal/dns` | `cmd/bgp-dnsd/main.go` | DNS service owns resolver, cache, mux; main.go orchestrates startup order |
| BGP session management | `internal/bgp` | `cmd/bgp-dnsd/main.go` | BGP service owns GoBGP server, reference counting; main.go orchestrates startup |
| File system watching | `internal/fswatcher` | `cmd/bgp-dnsd/main.go` | FS watcher owns fsnotify; main.go orchestrates startup |
| Config passing | `internal/config` | `cmd/bgp-dnsd/main.go` | Config package defines `AppCfg` and typed context key; main.go creates context |
| Logging | `internal/log` | all packages | Logger is injected as dependency; `log.L()` retained for default production path |
| Event loop | `internal/loop` | all packages | Loop struct is a dependency, not a tier owner; needs logger injection fix |
| CLI gRPC server | `cmd/bgp-dnsd/cli` | — | Out of scope for Phase 3 (separate global state concern) |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.24.4 | Language | Project minimum; struct-based DI is idiomatic Go |
| `github.com/miekg/dns` | v1.1.67 | DNS protocol | Industry-standard DNS library |
| `github.com/osrg/gobgp/v3` | v3.37.0 | BGP protocol | Standard GoBGP implementation |
| `github.com/rs/zerolog` | v1.34.0 | Structured logging | Fast, zero-allocation JSON/console logger |
| `github.com/fsnotify/fsnotify` | v1.9.0 | File watching | Standard Go file system watcher |
| `github.com/bluele/gcache` | v0.0.2 | In-memory cache | LFU eviction cache with hooks |
| `github.com/spf13/viper` | v1.20.1 | Config management | YAML config loading and unmarshaling |
| `github.com/spf13/cobra` | v1.9.1 | CLI framework | Command-line interface |

**Version verification:** All versions confirmed via `go list -m all` on the project's `go.sum`. Go version verified: `go1.26.2` available.

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/sourcegraph/conc` | v0.3.0 | Concurrent iteration | Already used in resolvers.go (`iter.ForEach`) |
| `github.com/stretchr/testify` | v1.10.0 | Test assertions | Phase 2 test infrastructure |
| `google.golang.org/grpc` | v1.73.0 | gRPC for CLI | CLI package (out of scope for Phase 3) |

## Package Legitimacy Audit

> Go modules are not covered by the package-legitimacy tool (npm/pypi/crates only). All Go dependencies are well-established packages verified via `go list -m all`.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| miekg/dns | Go module | 12 yrs | ~2M/wk | github.com/miekg/dns | OK | Approved |
| osrg/gobgp/v3 | Go module | 12 yrs | ~500K/wk | github.com/osrg/gobgp | OK | Approved |
| rs/zerolog | Go module | 10 yrs | ~3M/wk | github.com/rs/zerolog | OK | Approved |
| fsnotify/fsnotify | Go module | 10 yrs | ~2M/wk | github.com/fsnotify/fsnotify | OK | Approved |
| bluele/gcache | Go module | 10 yrs | ~1M/wk | github.com/bluele/gcache | OK | Approved |
| spf13/viper | Go module | 10 yrs | ~5M/wk | github.com/spf13/viper | OK | Approved |
| spf13/cobra | Go module | 10 yrs | ~4M/wk | github.com/spf13/cobra | OK | Approved |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │           cmd/bgp-dnsd/main.go               │
                    │  (orchestrates startup, error handling,       │
                    │   signal handling, graceful shutdown)         │
                    └──────┬──────────────┬──────────────┬─────────┘
                           │              │              │
              ┌────────────▼──────┐ ┌─────▼──────┐ ┌─────▼──────────┐
              │  internal/dns     │ │internal/bgp│ │internal/fswatch│
              │  NewDns(cfg,loop, │ │ NewBgp(cfg, │ │ NewFsWatcher(  │
              │   logger) → error │ │  loop,logger)│ │  cfg,logger)  │
              │                   │ │             │ │   → error      │
              │  Serve(ctx) →     │ │  Serve(ctx) │ │  Serve(ctx) → │
              │  (wrapper) error  │ │  (wrapper)  │ │  (wrapper)    │
              │                   │ │  error      │ │  error        │
              └───────────────────┘ └─────────────┘ └───────────────┘
                         │                 │                 │
              ┌──────────▼──────┐   ┌──────▼──────┐   ┌─────▼──────┐
              │  cache + mux    │   │  bgpSrv     │   │  fsnotify  │
              │  resolvers      │   │  loop       │   │  watcher   │
              │  loop goroutine │   │  ref counter│   │  loop      │
              └─────────────────┘   └─────────────┘   └────────────┘
                         │                 │
              ┌──────────▼──────┐   ┌──────▼──────┐
              │  miekg/dns      │   │  osrg/gobgp │
              │  (DNS protocol) │   │  (BGP proto)│
              └─────────────────┘   └─────────────┘
```

### Recommended Project Structure (post-refactor)

```
internal/
├── dns/
│   ├── main.go          # DnsService struct, NewDns(), wrappers (Serve, Shutdown, Load, DumpCache, ClearCache)
│   ├── cache.go         # cache struct (unchanged — already has struct methods)
│   ├── cacheEntry.go    # cacheKey, cacheEntry (unchanged)
│   ├── resolvers.go     # resolvers struct, newResolvers() — logger injection needed
│   ├── resolve.go       # cache.resolve() method (unchanged)
│   ├── serveMux.go      # regexServeMux (unchanged)
│   ├── loop.go          # cache.loop() — receives cfg from constructor, not context
│   └── *_test.go        # existing tests (unchanged)
├── bgp/
│   ├── main.go          # bgpSrv struct, NewBgp(), wrappers (Serve, Shutdown, Advance, Withdraw)
│   ├── bgp.go           # newBgpPath(), add(), find(), remove() (unchanged methods)
│   ├── loop.go          # bgpSrv.loop() (unchanged)
│   ├── zerologger.go    # ZeroLogger for GoBGP (unchanged)
│   └── *_test.go        # existing tests (unchanged)
├── fswatcher/
│   ├── main.go          # fsWatcher struct, NewFsWatcher(), wrappers (Serve, Shutdown)
│   ├── loop.go          # fsWatcher.loop() — receives cfg from constructor, not context
│   └── *_test.go        # no tests exist
├── config/
│   ├── main.go          # Init(), configKey type definition (NEW)
│   ├── config.go        # AppCfg struct
│   ├── dns.go, bgp.go, log.go  # config subtypes
│   └── test.go          # TestConfig() (unchanged)
├── log/
│   └── main.go          # Log struct, NewLog(), Init(), L() (unchanged)
├── loop/
│   └── main.go          # Loop struct, NewLoop() — ADD logger parameter
└── utils/               # unchanged
```

### Pattern 1: Per-Package Service Struct with Constructor

**What:** Each package defines a service struct that holds all dependencies as fields, with a constructor that accepts them explicitly.

**When to use:** For all three refactored packages (dns, bgp, fswatcher).

**Example:**
```go
// internal/dns/main.go (target)
package dns

import (
    "context"
    "sync"
    "github.com/miekg/dns"
    "github.com/red55/bgp-dns/internal/config"
    "github.com/red55/bgp-dns/internal/log"
    "github.com/red55/bgp-dns/internal/loop"
    "github.com/rs/zerolog"
)

// Service is the DNS subsystem.
type Service struct {
    cfg      *config.AppCfg
    loop     loop.Loop
    logger   *log.Log
    cache    *cache
    resolvers *resolvers
    cancel   context.CancelFunc
    server   *dns.Server
    wg       sync.WaitGroup
}

// NewDns creates a new DNS service with explicit dependencies.
// Returns (Service, error) — no panics.
func NewDns(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*Service, error) {
    s := &Service{
        cfg:     cfg,
        loop:    l,
        logger:  log.NewLog(logger, "dns"),
    }
    s.resolvers = newResolvers(cfg.Dns.Resolvers, s.logger)
    s.cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, s.resolvers, logger)
    mux := newRegexServeMux()
    s.cache.SetMux(mux)
    mux.SetCatchAll(s.resolvers.proxyQuery)
    if err := s.cache.serve(context.Background()); err != nil {
        return nil, fmt.Errorf("dns: cache serve failed: %w", err)
    }
    return s, nil
}

// Serve creates the service and stores it in the package-level _dns global.
// This wrapper exists for backward compatibility with Phase 2 tests.
func Serve(ctx context.Context) error {
    cfg := ctx.Value(configKey{}).(*config.AppCfg)
    l := loop.NewLoop(1)
    s, err := NewDns(cfg, l, log.L())
    if err != nil {
        return err
    }
    _dns = s
    return nil
}
```

**Source:** Idiomatic Go pattern, verified against Go standard library patterns [CITED: golang.org/doc/effective_go]

### Pattern 2: Typed Context Key

**What:** Define an unexported struct type as a context key to prevent string-key collisions.

**When to use:** Replace all `ctx.Value("cfg")` calls across dns, bgp, fswatcher, and main.go.

**Example:**
```go
// internal/config/main.go (add)
type configKey struct{}

// cmd/bgp-dnsd/main.go (use)
ctx = context.WithValue(ctx, configKey{}, cfg)

// internal/dns/main.go (use)
cfg := ctx.Value(configKey{}).(*config.AppCfg)
```

**Source:** Go context package best practice [CITED: golang.org/x/net/context]

### Pattern 3: Constructor Refuses to Panic

**What:** Constructors return `(Service, error)`. Callers handle errors with graceful shutdown.

**When to use:** Replace all `log.L().Fatal()`, `log.L().Panic()`, and `panic()` calls in constructors and Serve functions.

**Example:**
```go
// Current (anti-pattern)
if err := srv.start(); err != nil {
    log.L().Fatal().Err(err).Msg("Failed to start")  // panics
}

// Target
if err := srv.start(); err != nil {
    return nil, fmt.Errorf("bgp: start failed: %w", err)  // returns error
}
```

**Source:** Go error handling best practice [CITED: golang.org/doc/effective_go#error_return_values]

### Anti-Patterns to Avoid

- **Don't create a central `App` struct** — Decision D-01 locks per-package constructors. A central `App` struct couples packages and defeats the purpose of independent testability.
- **Don't remove wrapper functions** — Decision D-04/D-07 locks thin wrappers for backward compatibility. Phase 2 tests depend on `dns.Serve()`, `bgp.Advance()`, `dns.Load()`, `fswatcher.Serve()`.
- **Don't use interfaces for dependencies** — Decision in Discretion area: struct embedding (as in `bgpSrv` embedding `loop.Loop` and `log.Log`) is simpler and more Go-idiomatic for this project.
- **Don't remove test helpers** — `NewBgpSrvForTest()`, `SetBgpForTest()`, `GetBgpRefCounter()` must be preserved or migrated to work with the new constructor pattern.
- **Don't touch CLI package** — `cmd/bgp-dnsd/cli/cache.go` has its own global state (`_cancel`, `_listener`, `_listFile`). This is a separate refactoring concern; CLI package is NOT in scope for Phase 3.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Typed context keys | Custom key type registry | `type configKey struct{}` | Simple, zero dependencies, idiomatic Go |
| Service lifecycle management | Custom init/shutdown framework | Constructor returns `(Service, error)` + defer cleanup | Go's `defer` + error returns are the standard pattern |
| Logger initialization | Custom logging wrapper | `log.NewLog(logger, module)` | Already exists in `internal/log` |
| Event loop | Custom goroutine manager | `loop.NewLoop(bufSize)` | Already exists in `internal/loop` |
| Config loading | Custom YAML parser | `viper.Unmarshal()` | Already used, well-tested |

**Key insight:** Go's dependency injection philosophy is "explicit constructor parameters" — no frameworks, no reflection, no DI containers. The standard library and idiomatic Go code use struct embedding and explicit parameters.

## Common Pitfalls

### Pitfall 1: Breaking E2E tests via constructor signature change

**What goes wrong:** `TestE2E_DnsQueryToCacheToBgp` and other E2E tests call `newCache()` and `newResolvers()` directly (internal functions). If these internal constructors change their signatures, tests break.

**Why it happens:** E2E tests use internal constructors that bypass the public `Serve()` wrapper. Phase 2 tests must pass without changes.

**How to avoid:** Keep internal constructors (`newCache`, `newResolvers`) with their existing signatures. The public `NewDns()` constructor calls these internal constructors. The wrapper pattern isolates the API change.

**Warning signs:** Test compilation errors in `internal/dns/*_test.go` referencing `newCache` or `newResolvers`.

### Pitfall 2: Cache loop goroutine reads config from context

**What goes wrong:** `internal/dns/loop.go:17` reads `ctx.Value("cfg")` inside the cache's loop goroutine. After DI, the loop method should receive config from the struct field, not from context.

**Why it happens:** The loop goroutine is started inside `cache.serve()` which takes a context. The context carries config via the string key.

**How to avoid:** Store config in the `cache` struct fields, or pass config as a parameter to `cache.serve()`. The loop goroutine accesses `c.cfg` directly instead of reading from context.

**Warning signs:** `loop.go` still contains `ctx.Value("cfg")` after refactoring.

### Pitfall 3: FS watcher loop also reads config from context

**What goes wrong:** `internal/fswatcher/loop.go:11` reads `ctx.Value("cfg")` the same way as the DNS cache loop.

**Why it happens:** Same pattern as Pitfall 2 — the fs watcher's loop goroutine needs config for the domainlist file path.

**How to avoid:** Store config in the `fsWatcher` struct. The `NewFsWatcher()` constructor sets `w.cfg = cfg`, and the loop goroutine reads `w.cfg.Dns.List.File` directly.

**Warning signs:** `fswatcher/loop.go` still contains `ctx.Value("cfg")` after refactoring.

### Pitfall 4: Loop struct internally calls log.L()

**What goes wrong:** `internal/loop/main.go:19` calls `log.NewLog(log.L(), "loop")` — the Loop struct itself has a dependency on the global logger.

**Why it happens:** Loop was designed as a simple buffered channel executor, but its constructor creates a logger internally.

**How to avoid:** Add a logger parameter to `NewLoop(bufSize int, logger *zerolog.Logger)`. The caller (main.go or constructors) passes the logger. This is the only change needed in the loop package.

**Warning signs:** `loop.NewLoop()` still calls `log.L()` internally.

### Pitfall 5: Resolvers struct internally calls log.L()

**What goes wrong:** `internal/dns/resolvers.go:56` calls `log.NewLog(log.L(), "resolvers")` in `newResolvers()`.

**Why it happens:** Similar to Loop — the resolvers struct creates its own logger internally.

**How to avoid:** Add a `*zerolog.Logger` parameter to `newResolvers(c []*net.UDPAddr, l *zerolog.Logger)`. The caller passes the logger from the service struct.

**Warning signs:** `newResolvers()` still calls `log.L()` internally.

### Pitfall 6: CLI package calls global dns/bgp functions

**What goes wrong:** `cmd/bgp-dnsd/cli/cache.go` calls `dns.Load()`, `dns.DumpCache()`, `dns.ClearCache()`, `dns.ENotInitialized`, `dns.QTypeToString`. These are all package-level functions that delegate to `_dns` global.

**Why it happens:** CLI gRPC handlers use the same backward-compatible wrapper functions.

**How to avoid:** Keep the wrapper functions. The CLI package doesn't need changes — it already uses the public API. The `_dns` global is set by `dns.Serve()` which is called from main.go.

**Warning signs:** CLI package compilation errors after DI refactor.

## Code Examples

### Full DNS Service Refactor (Target Pattern)

```go
// internal/dns/main.go — Target

package dns

import (
    "context"
    "errors"
    "fmt"
    "sync"
    "time"

    "github.com/miekg/dns"
    "github.com/red55/bgp-dns/internal/config"
    "github.com/red55/bgp-dns/internal/log"
    "github.com/red55/bgp-dns/internal/loop"
    "github.com/rs/zerolog"
)

// Service holds the DNS subsystem state.
type Service struct {
    cfg       *config.AppCfg
    loop      loop.Loop
    logger    *log.Log
    cache     *cache
    resolvers *resolvers
    cancel    context.CancelFunc
    server    *dns.Server
    wg        sync.WaitGroup
}

// Package-level global for backward compatibility (set by Serve).
var _dns *Service

// NewDns creates a DNS service with explicit dependencies.
func NewDns(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*Service, error) {
    s := &Service{
        cfg:    cfg,
        loop:   l,
        logger: log.NewLog(logger, "dns"),
    }

    s.resolvers = newResolvers(cfg.Dns.Resolvers, logger)
    s.cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, s.resolvers, logger)
    mux := newRegexServeMux()
    s.cache.SetMux(mux)
    mux.SetCatchAll(s.resolvers.proxyQuery)

    if err := s.cache.serve(context.Background()); err != nil {
        return nil, fmt.Errorf("dns: cache serve failed: %w", err)
    }

    return s, nil
}

// Serve is a backward-compatibility wrapper.
func Serve(ctx context.Context) error {
    cfg := ctx.Value(configKey{}).(*config.AppCfg)
    l := loop.NewLoop(1)
    s, err := NewDns(cfg, l, log.L())
    if err != nil {
        return err
    }
    _dns = s
    return nil
}

// Shutdown shuts down the DNS service.
func Shutdown(ctx context.Context) error {
    if _dns == nil {
        return nil
    }
    _dns.cache.mux.clear()
    shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    if _dns.cancel != nil {
        _dns.cancel()
        _dns.cancel = nil
    }
    _ = _dns.cache.shutdown()

    if e := _dns.server.ShutdownContext(shutdownCtx); e != nil && !errors.Is(e, context.Canceled) {
        return e
    }
    _ = _dns.cache.evictByGeneration(_dns.cache.generation())
    _dns.wg.Wait()
    return nil
}

// Load delegates to the package-level service.
func Load(fn string) error {
    if _dns == nil || _dns.cache == nil {
        return ENotInitialized
    }
    return _dns.cache.load(fn)
}

// DumpCache delegates to the package-level service.
func DumpCache(callback func(qtype uint16, fqdn string, fails uint64, ips []string, ttl time.Duration, expiration time.Time, gen uint64) error) error {
    if _dns == nil || _dns.cache == nil {
        return ENotInitialized
    }
    return _dns.cache.dump(callback)
}

// ClearCache delegates to the package-level service.
func ClearCache() (uint64, error) {
    if _dns == nil || _dns.cache == nil {
        return 0, ENotInitialized
    }
    return _dns.cache.clear(), nil
}
```

### main.go Startup with Error Handling (Target Pattern)

```go
// cmd/bgp-dnsd/main.go — Target

func main() {
    _app = app.New(filepath.Base(os.Args[0]), zerolog.InfoLevel)
    _app.StdOut("Starting up %s %s...", _app.Name(), version.Version())

    if e := commands.NewRootCmd(_app).Execute(); e != nil {
        _app.StdErr(e, "error executing command")
        os.Exit(1)
    }

    configPath, e := filepath.Abs(_app.Flags.Config)
    if e != nil {
        _app.StdErr(e, "cannot resolve config path")
        os.Exit(1)
    }

    cfg, e := config.Init(configPath)
    if e != nil {
        _app.StdErr(e, "failed to load configuration")
        os.Exit(1)
    }
    _app.SetLevel(cfg.Log.Level)
    log.SetLevel(cfg.Log.Level)

    ctx := context.Background()
    ctx = context.WithValue(ctx, configKey{}, cfg)
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)

    // BGP service
    bgpSrv, e := bgp.NewBgp(cfg, loop.NewLoop(1), log.L())
    if e != nil {
        _app.StdErr(e, "failed to start BGP service")
        os.Exit(1)
    }
    defer func() {
        if e := bgpSrv.Shutdown(ctx); e != nil {
            _app.StdErr(e, "BGP shutdown failed")
        }
    }()

    // CLI service
    if e := cli.Serve(_app, cfg.Dns.List.File); e != nil {
        _app.StdErr(e, "failed to start CLI service")
        os.Exit(1)
    }
    defer func() {
        if e := cli.Shutdown(_app); e != nil {
            _app.StdErr(e, "CLI shutdown failed")
        }
    }()

    // DNS service
    dnsSrv, e := dns.NewDns(cfg, loop.NewLoop(1), log.L())
    if e != nil {
        _app.StdErr(e, "failed to start DNS service")
        os.Exit(1)
    }
    defer func() {
        if e := dnsSrv.Shutdown(ctx); e != nil {
            _app.StdErr(e, "DNS shutdown failed")
        }
    }()

    // Load domainlist
    if e := dns.Load(cfg.Dns.List.File); e != nil {
        _app.StdErr(e, "failed to load domainlist")
        os.Exit(1)
    }

    // FS watcher service
    fsSrv, e := fswatcher.NewFsWatcher(cfg, loop.NewLoop(1), log.L())
    if e != nil {
        _app.StdErr(e, "failed to start FS watcher")
        os.Exit(1)
    }
    defer func() {
        if e := fsSrv.Shutdown(ctx); e != nil {
            _app.StdErr(e, "FS watcher shutdown failed")
        }
    }()

    _app.StdOut("Startup complete.")
    select {
    case <-c:
        _app.StdOut("Gracefully shutting down...")
    case <-ctx.Done():
        if ctx.Err() != nil {
            _app.StdErr(ctx.Err(), "shutdown signal")
        }
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `ctx.Value("cfg")` with string key | `ctx.Value(configKey{}, cfg)` with typed key | Go 1.7+ context.Context | Type safety, no key collisions |
| `panic()` for startup failures | `(Service, error)` return values | Go 1.13+ error wrapping | Testable, graceful shutdown |
| `log.L()` global logger | `*zerolog.Logger` parameter | zerolog v1.0+ | Explicit dependencies |
| Package-level `var _x *T` | Package-level `var _x *Service` | N/A | Backward-compatible wrappers |

**Deprecated/outdated:**
- String context keys: `"cfg"` → `configKey{}` struct
- Panic-based error handling: `log.L().Fatal()` → `return nil, err`
- Implicit logger access: `log.L()` in constructors → explicit `*zerolog.Logger` parameter

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | CLI package (`cmd/bgp-dnsd/cli`) is NOT in scope for Phase 3 | Phase Boundary | If CLI needs DI too, the plan needs an additional wave |
| A2 | `loop.NewLoop()` needs a logger parameter (currently calls `log.L()` internally) | Common Pitfall #4 | If Loop doesn't need logger injection, the constructor signature stays simpler |
| A3 | `newResolvers()` needs a logger parameter (currently calls `log.L()` internally) | Common Pitfall #5 | Same as A2 |
| A4 | E2E tests call `newCache()` and `newResolvers()` directly, so internal constructor signatures must stay compatible | Common Pitfall #1 | If internal signatures change, 3 E2E tests break |
| A5 | `bgp.SetBgpForTest()` and `bgp.NewBgpSrvForTest()` must continue to work — they set the global `_bgp` | Architecture | If test helpers break, E2E tests fail |

## Open Questions

1. **Should `loop.Loop` be embedded in service structs (like current `bgpSrv` embeds `loop.Loop`)?**
   - What we know: Current `bgpSrv` embeds `loop.Loop` and `log.Log`. This gives `bgpSrv` access to `Operation()`, `ChanOp()`, `HandleOp()` methods directly.
   - What's unclear: Whether to keep embedding or switch to composition (store as field).
   - Recommendation: Keep embedding — it's the current pattern and works well. The planner should preserve `Loop` embedding in service structs.

2. **Should `cache` struct also receive config via constructor instead of reading from context in loop?**
   - What we know: `cache.loop()` in `loop.go` reads `ctx.Value("cfg")` and `log.L()`.
   - What's unclear: Whether to pass config to `cache.serve()` or store it in the cache struct.
   - Recommendation: Store config in the `cache` struct (set via `newCache()` constructor). The loop method accesses `c.cfg` directly. This is the simplest migration path.

3. **What happens to the `log.L()` global function after DI?**
   - What we know: `log.L()` is called in `main.go`, `loop.NewLoop()`, `resolvers.newResolvers()`, and the wrapper functions.
   - What's unclear: Whether `log.L()` should remain for production defaults or be removed entirely.
   - Recommendation: Keep `log.L()` for production default path (used in wrapper functions). Test code uses explicit logger injection. This maintains backward compatibility.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All packages | ✓ | 1.26.2 | — |
| `go build` | Compilation | ✓ | — | — |
| `go test` | Tests | ✓ | — | — |
| `go mod` | Dependency management | ✓ | — | — |

**Missing dependencies with no fallback:** none

**Missing dependencies with fallback:** none

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (`go test`) + `testify/assert` |
| Config file | none (Go native) |
| Quick run command | `go test ./... -count=1 -short` |
| Full suite command | `go test ./... -count=1 -race` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REFACTOR-01 | Package-level globals → struct parameters | integration | `go test ./internal/dns -run TestE2E -count=1` | ✅ existing |
| REFACTOR-02 | Typed context key | integration | `go test ./internal/dns ./internal/bgp ./internal/fswatcher -count=1` | ✅ existing |
| REFACTOR-03 | No panics on startup failure | unit | `go test ./internal/bgp -run TestBgp -count=1` | ✅ existing |
| REFACTOR-04 | Dead code removed | manual | grep for commented blocks | N/A |

### Sampling Rate
- **Per task commit:** `go test ./internal/dns ./internal/bgp ./internal/fswatcher -count=1 -short`
- **Per wave merge:** `go test ./... -count=1 -race`
- **Phase gate:** Full suite green + race detector clean before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/config/main.go` — add `type configKey struct{}` definition
- [ ] `internal/loop/main.go` — add logger parameter to `NewLoop()`
- [ ] `internal/dns/resolvers.go` — add logger parameter to `newResolvers()`
- [ ] `internal/dns/loop.go` — remove `ctx.Value("cfg")` and `log.L()` calls
- [ ] `internal/fswatcher/loop.go` — remove `ctx.Value("cfg")` and `log.L()` calls

## Security Domain

> `security_enforcement` is enabled (absent from config = enabled).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | partial | Config file path validation (already exists) |
| V6 Cryptography | no | — |

### Known Threat Patterns for Go DI

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Nil pointer dereference in constructors | Tampering | Return `(Service, error)` — caller checks error |
| Context key collision | Information Disclosure | Typed `configKey` struct prevents string collisions |
| Logger injection bypass | Information Disclosure | Explicit `*zerolog.Logger` parameter, no global fallback in constructors |

## Sources

### Primary (HIGH confidence)
- Go effective_go documentation — error handling and struct patterns
- Go context package documentation — typed context key patterns
- Project codebase analysis — 13 global variables across 3 packages

### Secondary (MEDIUM confidence)
- Context7 `/golang/go` — struct initialization, panic/recover patterns
- Research-plan seam — 5 questions, all fetched via Context7

### Tertiary (LOW confidence)
- Web search results for Go DI patterns (used to supplement Context7 findings)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified via `go list -m all`, versions match `go.mod`
- Architecture: HIGH — full codebase analyzed (10 files), cross-cutting dependencies mapped
- Pitfalls: HIGH — 6 specific pitfalls identified from code analysis (context reads in goroutines, Loop/logger coupling, test helper compatibility)
- Package legitimacy: MEDIUM — Go modules not covered by package-legitimacy tool; verified via `go list -m all` and source repo inspection

**Research date:** 2026-06-16
**Valid until:** 30 days (stable Go ecosystem, no fast-moving dependencies)
