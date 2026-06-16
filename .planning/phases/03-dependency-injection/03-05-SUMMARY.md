---
phase: 03-dependency-injection
plan: 05
subsystem: refactor
tags: [error-handling, panic-replacement, typed-context-key, constructor-pattern, deferred-cleanup]

# Dependency graph
requires:
  - phase: 03-01
    provides: typed config.ConfigKey{} struct for context passing
  - phase: 03-02
    provides: NewDns(cfg, loop, logger) constructor, Service struct with shutdown
  - phase: 03-03
    provides: NewBgp(cfg, loop, logger) constructor, bgpSrv struct with shutdown
  - phase: 03-04
    provides: NewFsWatcher(cfg, loop, logger) constructor, fsWatcher struct with shutdown
provides:
  - main.go with zero panic() calls — all startup failures return errors
  - typed config.ConfigKey{} used in context.WithValue instead of string "cfg"
  - Services created via NewBgp/NewDns/NewFsWatcher constructors
  - Deferred cleanup for each service (BGP → CLI → DNS → FSWatcher)
  - Shutdown methods on bgpSrv, Service, fsWatcher structs for direct cleanup

# Tech tracking
tech-stack:
  added: []
  patterns: [constructor-with-error-return, typed-context-keys, deferred-cleanup, panic-to-error-migration]

key-files:
  created: []
  modified:
    - cmd/bgp-dnsd/main.go
    - internal/bgp/main.go
    - internal/dns/main.go
    - internal/fswatcher/main.go

key-decisions:
  - "Shutdown methods added to service structs (bgpSrv.Shutdown, Service.Shutdown, fsWatcher.Shutdown) to enable direct cleanup without relying on package-level globals"
  - "config.ConfigKey{} (exported) used as typed context key — plan referenced unexported configKey{} but Phase 01 exported it as ConfigKey for cross-package test use"

patterns-established:
  - "Panic-to-error migration: replace panic() with StdErr logging + os.Exit(1) for startup failures"
  - "Constructor-based service creation with deferred cleanup"
  - "Shutdown methods on service structs enable direct cleanup independent of package globals"

requirements-completed: [REFACTOR-03]

# Metrics
duration: 5min
completed: 2026-06-16
---

# Phase 03 Plan 05: main.go — Panic-to-Error Migration with Typed Context Key and Constructor-Based Services

**Replace all 7 panic() calls in main.go with error handling, use typed config.ConfigKey{} context key, and create services via NewBgp/NewDns/NewFsWatcher constructors with deferred cleanup.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-16T09:15:00Z
- **Completed:** 2026-06-16T09:20:12Z
- **Tasks:** 1/1
- **Files modified:** 4

## Accomplishments
- All 7 `panic()` calls in main.go replaced with proper error handling (log + os.Exit)
- Typed `config.ConfigKey{}` used instead of string `"cfg"` context key
- Services created via `bgp.NewBgp`, `dns.NewDns`, `fswatcher.NewFsWatcher` constructors
- Added `Shutdown` methods on `bgpSrv`, `Service`, and `fsWatcher` structs for direct cleanup
- Deferred cleanup runs in reverse startup order: BGP → CLI → DNS → FSWatcher

## Task Commits

1. **Task 1: Replace panic() calls with error handling, use typed configKey and constructors** - `77ced1c` (feat)

## Files Created/Modified
- `cmd/bgp-dnsd/main.go` — Rewritten: 7 panic() → error handling, typed configKey, constructor-based services, deferred cleanup
- `internal/bgp/main.go` — Added `bgpSrv.Shutdown(ctx)` method for direct service cleanup
- `internal/dns/main.go` — Added `Service.Shutdown(ctx)` method for direct service cleanup
- `internal/fswatcher/main.go` — Added `fsWatcher.Shutdown(ctx)` method for direct service cleanup

## Decisions Made
- **Shutdown methods on service structs**: Added `Shutdown(ctx context.Context)` methods to `bgpSrv`, `Service`, and `fsWatcher` structs so main.go can call them directly without relying on package-level globals. This enables proper deferred cleanup when services are created via constructors.
- **Exported ConfigKey**: Plan referenced `configKey{}` but Phase 01 exported it as `config.ConfigKey{}` for cross-package test use. Used the exported name.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added Shutdown methods on service structs**
- **Found during:** Task 1 (rewriting main.go)
- **Issue:** Plan action code calls `bgpSrv.Shutdown(ctx)`, `dnsSrv.Shutdown(ctx)`, `fsSrv.Shutdown(ctx)` but these methods don't exist on the service structs — only package-level `Shutdown(ctx)` functions exist
- **Fix:** Added `Shutdown(ctx context.Context)` methods to `bgpSrv`, `Service`, and `fsWatcher` structs that operate on the receiver directly. Refactored package-level Shutdown functions to delegate to these methods for backward compatibility with Phase 2 tests.
- **Files modified:** internal/bgp/main.go, internal/dns/main.go, internal/fswatcher/main.go
- **Verification:** `go build ./cmd/bgp-dnsd` succeeds, `go test ./... -race` passes
- **Committed in:** `77ced1c` (part of task commit)

**2. [Rule 1 - Bug] Used exported ConfigKey{} instead of unexported configKey{}**
- **Found during:** Task 1 (rewriting main.go)
- **Issue:** Plan references `configKey{}` but the type was exported as `ConfigKey` in Phase 01 for cross-package test use
- **Fix:** Used `config.ConfigKey{}` in main.go, consistent with how bgp, dns, and fswatcher packages reference it
- **Files modified:** cmd/bgp-dnsd/main.go
- **Verification:** Build succeeds, all tests pass
- **Committed in:** `77ced1c` (part of task commit)

---

**Total deviations:** 2 auto-fixed (1 missing critical, 1 bug)
**Impact on plan:** Both deviations necessary for correctness — Shutdown methods enable the deferred cleanup pattern, and using the exported ConfigKey name is consistent with the rest of the codebase.

## Issues Encountered
None — build and tests pass cleanly.

## Verification

- [x] `go build ./cmd/bgp-dnsd` succeeds
- [x] `go test ./... -race` — all packages pass
- [x] No `panic(` calls in cmd/bgp-dnsd/main.go
- [x] `config.ConfigKey{}` used in context.WithValue
- [x] All services created via NewXxx constructors
- [x] Deferred cleanup for BGP, CLI, DNS, and FSWatcher services

## Next Phase Readiness
- Plan 03-06 (remaining work in phase) can proceed — main.go error handling is complete
- All constructors and typed context keys are in place
- Phase 3 is nearly complete (plans 05 and 06 remain)

---
*Phase: 03-dependency-injection*
*Completed: 2026-06-16*
