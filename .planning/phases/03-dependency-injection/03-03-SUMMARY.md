---
phase: 03-dependency-injection
plan: 03
subsystem: infra
tags: [dependency-injection, constructor, error-handling, bgp]

# Dependency graph
requires:
  - phase: 03-01
    provides: "Typed configKey, NewLoop(logger) signature, log.L()"
provides:
  - "NewBgp(cfg, loop, logger) constructor returning (*bgpSrv, error)"
  - "Serve wrapper using typed configKey and NewBgp"
  - "Shutdown returns error instead of panicking"
  - "Advance/Withdraw with nil guard checks"
affects: [03-05, 03-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Constructor returns (Service, error) — no panics"
    - "Thin wrapper functions delegate through package-level _bgp global"
    - "Typed configKey{} for context config extraction"

key-files:
  created: []
  modified:
    - internal/bgp/main.go

key-decisions:
  - "Shutdown returns error instead of panicking on StopBgp failure"
  - "Advance/Withdraw guard with nil check for _bgp before Operation"

patterns-established:
  - "NewBgp pattern: explicit dependencies via parameters, error returns, no panics"
  - "Wrapper pattern: Serve/Shutdown/Advance/Withdraw delegate through _bgp global for backward compatibility"

requirements-completed: ["REFACTOR-01"]

# Metrics
duration: 15min
completed: 2026-06-16
---

# Phase 03 Plan 03: NewBgp Constructor with Error Returns

**NewBgp(cfg, loop, logger) constructor returning (*bgpSrv, error), updated Serve/Shutdown/Advance/Withdraw wrappers using typed configKey{}, all panic-on-init replaced with error returns.**

## Performance

- **Duration:** 15 min
- **Started:** 2026-06-16T08:28:00Z
- **Completed:** 2026-06-16T08:43:58Z
- **Tasks:** 3 (2 plan tasks + 1 auto-fix)
- **Files modified:** 1

## Accomplishments
- NewBgp constructor with explicit dependencies (cfg, loop, logger) returns `(*bgpSrv, error)` — zero panics in constructor
- Serve wrapper uses typed `config.ConfigKey{}` and calls NewBgp, storing result in `_bgp` global
- Shutdown returns errors instead of calling `Panic()` on StopBgp failure
- Advance/Withdraw guard with nil check for `_bgp` before delegating to Operation
- All 6 bgp unit tests pass, all package tests pass with `-race`

## Task Commits

Each task was committed atomically:

1. **Task 1: Add NewBgp constructor to bgp/main.go** - `fc41f3f` (feat)
2. **Task 2: Update Serve, Shutdown, Advance, Withdraw wrappers** - `29d6b42` (feat)
3. **Task 3: Fix fmt.Errorf arg type for peer.Address** - `1b92445` (fix)

**Plan metadata:** `1b92445` (docs: complete plan)

## Files Created/Modified
- `internal/bgp/main.go` — Added NewBgp constructor, updated Serve/Shutdown/Advance/Withdraw wrappers, replaced all panic-on-init with error returns

## Decisions Made
- Shutdown returns `fmt.Errorf` instead of calling `_bgp.L().Panic()` — callers get error to handle gracefully
- Advance/Withdraw check `_bgp == nil` and return descriptive error — prevents nil pointer dereference in tests that don't call Serve
- Used `peer.Address.String()` in error message (net.TCPAddr → string conversion)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed fmt.Errorf format verb type mismatch**
- **Found during:** Task 1/3 verification
- **Issue:** `fmt.Errorf("bgp: add peer %s failed: %w", peer.Address, e)` — `peer.Address` is `net.TCPAddr`, not a string. `%s` format expects string or fmt.Stringer.
- **Fix:** Changed to `peer.Address.String()` to produce correct string representation.
- **Files modified:** `internal/bgp/main.go`
- **Verification:** `go build ./internal/bgp` succeeds, `go test ./internal/bgp -count=1` passes
- **Committed in:** `1b92445` (fix commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug)
**Impact on plan:** Necessary type fix for correctness. No scope creep.

## Issues Encountured
- `fmt` import was missing after the Serve function replacement edit — added back in first edit
- `ctx` variable declaration in NewBgp: `ctx, s.cancel = context.WithCancel(...)` doesn't work because `s.cancel` is a field assignment (not a variable). Fixed by splitting into `ctx, cancel := context.WithCancel(...)` then `s.cancel = cancel`.

## Test Results

```
=== RUN   TestBgp_ReferenceCounting_SingleIP
--- PASS: TestBgp_ReferenceCounting_SingleIP (0.00s)
=== RUN   TestBgp_ReferenceCounting_MultiDomainSharing
--- PASS: TestBgp_ReferenceCounting_MultiDomainSharing (0.00s)
=== RUN   TestBgp_ReferenceCounting_ConcurrentAdvances
--- PASS: TestBgp_ReferenceCounting_ConcurrentAdvances (0.00s)
=== RUN   TestBgp_ReferenceCounting_WithdrawNonExistent
--- PASS: TestBgp_ReferenceCounting_WithdrawNonExistent (0.00s)
=== RUN   TestBgp_ReferenceCounting_MixedSequence
--- PASS: TestBgp_ReferenceCounting_MixedSequence (0.00s)
=== RUN   TestBgp_AddRemove_NilSafe
--- PASS: TestBgp_AddRemove_NilSafe (0.00s)
PASS
ok  	github.com/red55/bgp-dns/internal/bgp	0.012s
```

All tests pass with `-race` flag. No data races detected.

## Self-Check

- [x] `go build ./internal/bgp` succeeds
- [x] `go test ./internal/bgp -count=1` passes (6/6 tests)
- [x] `go test ./... -race -count=1` passes (all packages)
- [x] No `panic()` calls in NewBgp constructor
- [x] Shutdown returns error instead of panicking
- [x] Serve uses typed `config.ConfigKey{}`
- [x] Test helpers (`newTestBgpSrv`, `NewBgpSrvForTest`) already use correct `loop.NewLoop(1, &l)` signature

## Next Phase Readiness
- Plan 03-05 (main.go error handling) depends on NewBgp constructor and typed configKey — both are ready
- Plan 03-06 (dead code removal) can proceed after 03-05

---
*Phase: 03-dependency-injection*
*Completed: 2026-06-16*
