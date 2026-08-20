# Phase 03: Dependency Injection - Context

**Gathered:** 2026-06-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Replace global package-level state in `internal/dns/`, `internal/bgp/`, `internal/fswatcher/` with explicit dependency injection via typed constructors. Keep thin wrapper functions for backward compatibility so Phase 2 tests pass unchanged. Convert panic-on-init to error returns. Replace string `"cfg"` context key with typed `configKey` struct.

</domain>

<decisions>
## Implementation Decisions

### Constructor API Design (from Phase 01 discussion, locked)
- **D-01:** Per-package constructors — not a central `App` struct
  - Each package gets its own constructor: `NewDns(cfg, loop, logger)`, `NewBgp(cfg, loop, logger)`, `NewFsWatcher(cfg, logger)`
  - Keeps packages independent, makes testing straightforward
- **D-02:** Constructor signature returns `(Service, error)` — no panics
  - Phase 3 success criteria #3: "Startup failures return errors instead of panicking"
  - `cmd/bgp-dnsd/main.go` handles errors gracefully with defer cleanup
- **D-03:** Return type is a struct with methods — Go-idiomatic, explicit, readable
  - Example: `type DnsService struct { cfg *config.AppCfg; loop loop.Loop; logger *log.Log; cache *cache; mux *regexServeMux }`

### Global Function Retention (from Phase 01 discussion, locked)
- **D-04:** Keep thin wrapper functions for backward compatibility
  - `dns.Serve()` → creates and stores service in `_dns` global — same pattern as current code
  - `bgp.Advance()` → delegates to package-level `_dns.bgp.Operation(...)`
  - `dns.Load()` → delegates to `_dns.cache.load()`
  - Rationale: Phase 2 tests must pass without changes — this is the safety net purpose
  - Note: Wrappers are thin — they only exist for backward compatibility, not as a design goal

### Config Context Key (DISCUSSED)
- **D-05:** Typed context key (Option B)
  - `type configKey struct{}` defined in `internal/config/main.go`
  - `context.WithValue(ctx, configKey{}, cfg)` in `cmd/bgp-dnsd/main.go`
  - `cfg := ctx.Value(configKey{}).(*config.AppCfg)` in each package
  - Rationale: Keeps context pattern consistent across packages, easier migration from current code, typed key prevents collisions

### Logger Injection (DISCUSSED)
- **D-06:** Explicit logger parameter (Option A)
  - `NewLog(logger *zerolog.Logger, module string)` — caller passes zerolog instance
  - Remove `log.L()` global calls from constructors
  - Rationale: More explicit, easier to test, aligns with DI philosophy
  - Note: `log.L()` can remain for non-test code paths (e.g., production default)

### Backward Compatibility (DISCUSSED)
- **D-07:** Keep thin wrapper functions (Option A)
  - `bgp.Advance()` → delegates to `_bgp.Operation(...)` — tests don't need changes
  - `dns.Serve()` → creates and stores service in `_dns` global — same pattern
  - `dns.Load()` → delegates to `_dns.cache.load()` — tests don't need changes
  - Rationale: Phase 2 tests must pass without changes — this is the safety net purpose

### Dead Code Removal (from ROADMAP.md success criteria)
- **D-08:** Remove commented error handling in `internal/dns/resolvers.go:127-148` (22 lines of dead code)
- **D-09:** Remove commented hashmap in `internal/bgp/main.go:19` (`//ipRefCounter *hashmap.Map[string, *atomic.Uint64]`)

### the agent's Discretion
- Exact struct field names for service types
- Order of constructor parameters
- How to handle `internal/loop` dependency injection (passed as `loop.Loop` interface or struct)
- Whether to add `internal/config/test.go` helpers for Phase 3 tests (Phase 2 already created `TestConfig()`)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements
- `.planning/ROADMAP.md` — Phase 3 goal, success criteria, dependencies
- `.planning/REQUIREMENTS.md` — REFACTOR-01 through REFACTOR-04
- `.planning/PROJECT.md` — Key Decisions table (DI decisions pending)

### Prior Phase Context
- `.planning/phases/02-test-suite/02-CONTEXT.md` — Phase 2 test decisions (backward compatibility constraints)
- `.planning/phases/01-regex-domainlist/01-CONTEXT.md` — Phase 1 decisions (work within globals, defer DI)

### Verification
- `.planning/phases/02-test-suite/02-VERIFICATION.md` — Phase 2 verification (63 tests pass, race detector clean)
- `.planning/phases/01-regex-domainlist/01-VERIFICATION.md` — Phase 1 verification (38 tests pass)

### Codebase Maps
- `.planning/codebase/ARCHITECTURE.md` — Global state anti-patterns, data flow, entry points
- `.planning/codebase/CONCERNS.md` — Test coverage status, known technical debt
- `.planning/codebase/PATTERNS.md` — Existing test patterns (same-package, test helpers)
- `.planning/codebase/STACK.md` — Technology stack and dependencies

### Target Files (Must Refactor)
- `internal/dns/main.go` — Global `_server`, `_resolvers`, `_cancel`, `_cache` (lines 15-26)
- `internal/bgp/main.go` — Global `_bgp` (line 32), panic on BGP start (line 70)
- `internal/fswatcher/main.go` — Global `_watcher` (line 24)
- `cmd/bgp-dnsd/main.go` — Panic-on-init pattern (lines 35, 39, 57, 66, 75, 84, 88), string context key (line 49)
- `internal/dns/resolvers.go` — Dead code at lines 127-148
- `internal/bgp/main.go` — Commented hashmap at line 19

### Supporting Infrastructure
- `internal/config/main.go` — Config Init, context key type definition target
- `internal/log/main.go` — Logger global `log.L()` and explicit `NewLog()`
- `internal/loop/main.go` — `Loop` struct with `Operation(f, ret)` method
- `internal/dns/cache.go` — Cache struct with mux field, load(), register()
- `.opencode/skills/spike-findings-bgp-dns/SKILL.md` — Spike findings (regexServeMux blueprint)

</canonical_refs>

<code_context>
## Existing Code Insights

### Global State Locations
| Package | Global Vars | Files |
|---------|-------------|-------|
| `internal/dns` | `_server`, `_wg`, `_resolvers`, `_cancel`, `_cache` | `main.go:15-26` |
| `internal/bgp` | `_bgp`, `_v4Family` | `main.go:31-38` |
| `internal/fswatcher` | `_watcher` | `main.go:23-25` |
| `cmd/bgp-dnsd/cli` | `_listener`, `_listFile` | `cache.go:26-29` |

### Current DI Patterns (Partial)
- `internal/log/main.go` — `NewLog(logger *zerolog.Logger, module string)` — explicit logger injection already exists
- `internal/loop/main.go` — `NewLoop(bufSize int)` — no dependencies, pure constructor
- `internal/config/main.go` — `Init(path string)` — config loading, no DI needed

### Test Helpers (Phase 2)
- `internal/bgp/main.go` — `NewBgpSrvForTest(t)`, `SetBgpForTest(s)`, `GetBgpRefCounter()`
- `internal/config/test.go` — `TestConfig()` for minimal test config
- These helpers must be preserved or migrated to Phase 3 constructors

### Integration Points
- `cmd/bgp-dnsd/main.go` — bootstrap calls: `bgp.Serve(ctx)`, `cli.Serve()`, `dns.Serve(ctx)`, `dns.Load()`, `fswatcher.Serve(ctx)`
- `internal/dns/main.go` — `Serve(ctx)` reads config from context, creates resolvers, cache, mux
- `internal/bgp/main.go` — `Serve(ctx)` reads config from context, creates bgpSrv, starts GoBGP server
- `internal/fswatcher/main.go` — `Serve(ctx)` reads config from context, creates fsWatcher, starts fsnotify

</code_context>

<specifics>
## Specific Ideas

### Constructor Examples (from ARCHITECTURE.md anti-patterns)
```go
// Current (anti-pattern)
func Serve(ctx context.Context) error {
    cfg := ctx.Value("cfg").(*config.AppCfg) // string key!
    _bgp = &bgpSrv{...}
    // panic on error
}

// Target (DI)
func NewBgp(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*BgpService, error) {
    srv := &BgpService{
        cfg:    cfg,
        loop:   l,
        logger: logger,
        bgp:    bgpsrv.NewBgpServer(...),
    }
    if err := srv.start(); err != nil {
        return nil, err // no panic
    }
    return srv, nil
}
```

### Typed Context Key Pattern (from ARCHITECTURE.md)
```go
// internal/config/main.go
type configKey struct{}

// cmd/bgp-dnsd/main.go
ctx = context.WithValue(ctx, configKey{}, cfg)

// internal/dns/main.go
cfg := ctx.Value(configKey{}).(*config.AppCfg)
```

</specifics>

<deferred>
## Deferred Ideas

- **Full global state removal** — Keep thin wrappers for backward compatibility in Phase 3; full removal can be a future phase if needed
- **Interface-based DI** — Using interfaces for dependencies instead of concrete types; struct embedding is simpler and more Go-idiomatic for this use case
- **Config validation** — Adding validation logic during `NewXxx()` constructors; defer to Phase 4 (Reliability)
- **Multi-instance support** — Global state architecture prevents this; DI enables it but full multi-instance is out of scope for Phase 3

</deferred>

---

*Phase: 03-dependency-injection*
*Context gathered: 2026-06-15*
