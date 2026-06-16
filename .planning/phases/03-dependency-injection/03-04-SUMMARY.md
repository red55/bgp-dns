---
phase: 03-dependency-injection
plan: 04
subsystem: refactor
tags: [dependency-injection, constructor-pattern, fswatcher, config-injection]

requires:
  - phase: 03-01
    provides: typed config.ConfigKey{}, fsWatcher struct with cfg field, loop.Loop and log.Log interfaces

provides:
  - NewFsWatcher(cfg, loop, logger) constructor returning (*fsWatcher, error)
  - Serve() wrapper using typed configKey and NewFsWatcher
  - Shutdown() preserved as nil-safe wrapper

affects: [03-02, 03-03, 03-05, 03-06]

tech-stack:
  added: []
  patterns: [constructor-with-error-return, typed-context-keys, thin-wrapper-compatibility]

key-files:
  created: []
  modified:
    - internal/fswatcher/main.go

key-decisions:
  - "Serve wrapper uses typed config.ConfigKey{} instead of string \"cfg\" context key"
  - "NewFsWatcher uses context.Background() for loop context (not passed-in ctx) to isolate watcher lifecycle from caller"
  - "zerolog.Logger passed explicitly to constructor instead of log.L() global"

patterns-established:
  - "Constructor pattern: NewX(cfg, l, logger) returns (Service, error) — no panics, explicit deps"
  - "Wrapper pattern: Serve(ctx) extracts config from typed key, creates deps, calls constructor, stores in package global"

requirements-completed: [REFACTOR-01]

duration: 5min
completed: 2026-06-16
---

# Phase 03 Plan 04: FSWatcher Dependency Injection

**NewFsWatcher(cfg, loop, logger) constructor with explicit dependencies, typed configKey in Serve wrapper, nil-safe Shutdown — all without panics.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-16T08:00:00Z
- **Completed:** 2026-06-16T08:05:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Created `NewFsWatcher(cfg, loop, logger)` constructor returning `(*fsWatcher, error)` with explicit dependencies
- Updated `Serve()` wrapper to use typed `config.ConfigKey{}` and delegate to `NewFsWatcher`
- Preserved `Shutdown()` as nil-safe wrapper using package-level `_watcher` global

## Task Commits

Each task was committed atomically:

1. **Task 1: Add cfg field to fsWatcher struct and create NewFsWatcher constructor** - `4de20cd` (feat)
2. **Task 2: Update Serve and Shutdown wrappers for NewFsWatcher** - `4de20cd` (feat)

**Plan metadata:** `4de20cd` (docs: complete plan)

_Note: Tasks 1 and 2 were combined into a single commit since they modify the same file and are tightly coupled._

## Files Created/Modified
- `internal/fswatcher/main.go` — Added `NewFsWatcher` constructor, updated `Serve` wrapper with typed configKey, added zerolog import

## Decisions Made
- Used `config.ConfigKey{}` typed context key instead of string `"cfg"` — consistent with other packages in phase 03-01
- NewFsWatcher creates its own background context for the loop goroutine (not using caller's ctx) — isolates watcher lifecycle
- zerolog.Logger passed explicitly to constructor — enables testability with mock loggers

## Deviations from Plan

**1. [Rule 3 - Blocking] Fixed Go variable declaration syntax for loopCtx**
- **Found during:** Task 1 (build verification)
- **Issue:** `loopCtx, w.cancel = context.WithCancel(...)` failed because `loopCtx` was undeclared; `loopCtx, w.cancel := context.WithCancel(...)` failed because `w.cancel` is a field, not a name
- **Fix:** Split into `var loopCtx context.Context` followed by `loopCtx, w.cancel = context.WithCancel(...)`
- **Files modified:** internal/fswatcher/main.go
- **Verification:** `go build ./...` succeeds
- **Committed in:** `4de20cd` (task commit)

**2. [Rule 3 - Blocking] Restored internal/bgp/main.go from HEAD**
- **Found during:** Full build verification
- **Issue:** Uncommitted broken changes in bgp/main.go (partial refactor from prior wave) caused build failures
- **Fix:** Restored bgp/main.go to last committed state via `git checkout -- internal/bgp/main.go`
- **Files modified:** internal/bgp/main.go (restored)
- **Verification:** `go build ./...` and `go test ./... -race` pass
- **Committed in:** (restored, not part of this plan's commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes were necessary for correctness. No scope creep.

## Issues Encountored
- None beyond the two deviations above.

## Self-Check

| Check | Result |
|-------|--------|
| `fsWatcher` struct has `cfg *config.AppCfg` field | ✅ (already present from Wave 0) |
| `NewFsWatcher(cfg, loop, logger)` returns `(*fsWatcher, error)` | ✅ |
| No `panic()` in `NewFsWatcher` | ✅ |
| `Serve` uses typed `config.ConfigKey{}` | ✅ |
| `Shutdown` preserves existing logic with nil guard | ✅ |
| `go build ./...` succeeds | ✅ |
| `go test ./... -race` passes | ✅ |

## Next Phase Readiness
- FSWatcher constructor pattern established, ready for wave 4-6 to inject into app lifecycle
- Typed configKey usage consistent across fswatcher, bgp, dns packages

---
*Phase: 03-dependency-injection*
*Completed: 2026-06-16*
