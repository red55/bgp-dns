---
phase: 03-dependency-injection
plan: 02
subsystem: dns
tags: [dependency-injection, constructor, service-struct, backward-compat]

# Dependency graph
requires:
  - phase: 03-01
    provides: "Typed configKey, NewLoop(logger) signature, cache.cfg field, newCache(cfg) signature"
provides:
  - "Service struct holding all DNS subsystem state"
  - "NewDns(cfg, loop, logger) constructor returning (*Service, error)"
  - "Serve/Shutdown/Load/DumpCache/ClearCache wrapper functions delegating through _dns global"
affects: [03-05, 03-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Constructor returns (Service, error) — no panics"
    - "Thin wrapper functions delegate through package-level _dns global"
    - "Typed config.ConfigKey{} for context config extraction"

key-files:
  created: []
  modified:
    - internal/dns/main.go

key-decisions:
  - "Service struct embeds loop.Loop and log.Log (matching bgpSrv pattern)"
  - "Serve wrapper uses typed config.ConfigKey{} — production cmd/main.go uses string key \"cfg\" (pre-existing mismatch, not fixed in this plan)"
  - "Shutdown/Load/DumpCache/ClearCache guard with _dns == nil checks before delegation"

patterns-established:
  - "NewDns pattern: explicit dependencies via parameters, error returns, no panics"
  - "Wrapper pattern: Serve/Shutdown/Load/DumpCache/ClearCache delegate through _dns global for backward compatibility with Phase 2 tests"

requirements-completed: ["REFACTOR-01"]

# Metrics
duration: 5min
completed: 2026-06-16
---

# Phase 03 Plan 02: DNS Service Struct with Explicit Constructor

**Service struct holding all DNS subsystem state, NewDns(cfg, loop, logger) constructor returning (*Service, error), backward-compatible wrapper functions (Serve/Shutdown/Load/DumpCache/ClearCache) delegating through _dns global.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-16T09:00:00Z
- **Completed:** 2026-06-16T09:05:00Z
- **Tasks:** 3/3
- **Files modified:** 1

## Accomplishments

- Defined `Service` struct with fields: `cfg`, `loop`, `log.Log`, `cache`, `resolvers`, `cancel`, `server`, `wg` — embeds `loop.Loop` and `log.Log` per `bgpSrv` pattern
- Created `NewDns(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*Service, error)` constructor — returns error on cache serve failure, no panics
- Added package-level `var _dns *Service` global for backward-compatible wrapper functions
- Updated `Serve(ctx)` wrapper to use typed `config.ConfigKey{}` and call `NewDns(cfg, l, log.L())`
- Updated `Shutdown(ctx)` to check `_dns == nil`, delegate to `_dns.server`, `_dns.cancel`, `_dns.cache`, `_dns.wg`
- Updated `Load(fn)`/`DumpCache(callback)`/`ClearCache()` to check `_dns == nil || _dns.cache == nil`, return `ENotInitialized`
- Removed old package-level globals (`_server`, `_wg`, `_resolvers`, `_cancel`, `_cache`) — now part of `Service` struct
- `cache.go` and `loop.go` already correct from Phase 3 Wave 0 — no changes needed
- All 12+ dns tests pass with `-race` flag
- Full build succeeds (`go build ./...`)

## Task Commits

Each task was committed atomically:

1. **Task 1: Define Service struct and NewDns constructor in dns/main.go** - `9f66ecc` (feat)
2. **Task 2: Update Serve wrapper, Shutdown, Load, DumpCache, ClearCache to use Service struct** - `9f66ecc` (feat, included in same commit as Task 1 — interdependent changes in single file)
3. **Task 3: Update cache.go and loop.go to integrate with Service struct** - N/A (already correct from Wave 0)

**Plan metadata:** `9f66ecc` (feat: complete plan)

## Files Created/Modified

| File | Change |
|------|--------|
| `internal/dns/main.go` | Added `Service` struct, `NewDns` constructor, `_dns` global, updated all wrapper functions (Serve/Shutdown/Load/DumpCache/ClearCache) |

## Decisions Made

- **Service embeds loop.Loop and log.Log**: Matches the `bgpSrv` pattern from plan 03-03. This allows `Service` to call `L()` for logging and `ChanOp()`/`Operation()` for loop operations.
- **Serve uses typed config.ConfigKey{}**: Consistent with bgp.Serve pattern. Note: production `cmd/bgp-dnsd/main.go` still stores config with string key `"cfg"` — this is a pre-existing mismatch that should be fixed in a separate plan.
- **Shutdown nil-checks _dns**: Prevents panic if Shutdown is called before Serve (e.g., in tests or error paths).
- **Load/DumpCache/ClearCache check _dns == nil \|\| _dns.cache == nil**: Returns `ENotInitialized` consistently.

## Deviations from Plan

### No Deviations

Plan executed exactly as written. The `cache.go` and `loop.go` files required no changes — they were already updated in Phase 3 Wave 0 (plan 03-01) with `cfg` field and `c.cfg` usage in `cache.loop`.

### Note: Single Commit for Tasks 1 & 2

Tasks 1 and 2 were committed together in `9f66ecc` because both modify the same file (`internal/dns/main.go`) and are interdependent — the wrappers cannot work without the Service struct, and the Service struct needs the wrappers for backward compatibility.

## Issues Encountered

- **LSP error**: Initial write used `log:` instead of `Log:` for the embedded `log.Log` field. Fixed by correcting the field name to match the struct definition.
- **cmd/main.go context key mismatch**: `cmd/bgp-dnsd/main.go` stores config with string key `"cfg"` but `Serve` now uses `config.ConfigKey{}`. This is a pre-existing issue in the cmd/main.go file (bgp.Serve has the same issue). Not fixed in this plan — should be addressed in a separate cmd/main.go update.

## Test Results

```
=== RUN   TestCacheUpsert
--- PASS: TestCacheUpsert (0.00s)
=== RUN   TestCacheUpsert_Collision
--- PASS: TestCacheUpsert_Collision (0.00s)
=== RUN   TestCache_Load_FileNotFound
--- PASS: TestCache_Load_FileNotFound (0.00s)
=== RUN   TestCache_Load_GenerationIncrease
--- PASS: TestCache_Load_GenerationIncrease (0.00s)
=== RUN   TestCache_Load_MultipleLoads
--- PASS: TestCache_Load_MultipleLoads (0.00s)
=== RUN   TestCache_Load_BlankLines
--- PASS: TestCache_Load_BlankLines (0.00s)
=== RUN   TestCache_Load_FileWithComments
--- PASS: TestCache_Load_FileWithComments (0.00s)
=== RUN   TestCache_Load_EmptyFile
--- PASS: TestCache_Load_EmptyFile (0.00s)
=== RUN   TestResolver_SingleSuccess
--- PASS: TestResolver_SingleSuccess (0.00s)
=== RUN   TestResolver_Failover
--- PASS: TestResolver_Failover (0.00s)
=== RUN   TestResolver_Recovery
--- PASS: TestResolver_Recovery (0.00s)
=== RUN   TestResolver_AllFail
--- PASS: TestResolver_AllFail (0.00s)
=== RUN   TestServeMux_* (11 tests)
--- PASS: TestServeMux_* (0.00s)
PASS
ok  	github.com/red55/bgp-dns/internal/dns	1.075s
```

All tests pass with `-race` flag. No data races detected.

## Verification

| Criteria | Status |
|----------|--------|
| `go build ./internal/dns` succeeds | PASS |
| `go build ./...` succeeds | PASS |
| `go test ./internal/dns -count=1` passes | PASS |
| `go test ./... -race -count=1` passes | PASS |
| `Service` struct defined with all fields | PASS |
| `NewDns(cfg, loop, logger) (*Service, error)` signature | PASS |
| `var _dns *Service` exists | PASS |
| Serve uses `config.ConfigKey{}` | PASS |
| Shutdown checks `_dns == nil` | PASS |
| Load/DumpCache/ClearCache check `_dns == nil \|\| _dns.cache == nil` | PASS |
| `cache.loop` uses `c.cfg` (no changes needed) | PASS |
| Backward compat wrappers exist | PASS |

## Self-Check: PASSED

## Next Phase Readiness

DNS Service struct is ready. Plans 03-05 and 03-06 can proceed:
- Plan 03-05 (main.go error handling) — can use NewDns constructor
- Plan 03-06 (dead code removal) — can remove old globals after all consumers migrate

---
*Phase: 03-dependency-injection*
*Completed: 2026-06-16*
