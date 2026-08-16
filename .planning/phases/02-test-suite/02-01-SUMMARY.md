---
phase: 02-test-suite
plan: 01
subsystem: testing
tags: [go, unit-tests, bgp, dns, race-detector, testify]

# Dependency graph
requires:
  - phase: 01-regex-domainlist
    provides: "existing codebase with BGP server, DNS resolvers, cache, and loop infrastructure"
provides:
  - "BGP reference counting unit tests (5 test functions + nil-safety helper)"
  - "DNS resolver failover unit tests (4 test functions with fake DNS servers)"
  - "Nil-safety guards on bgp add()/remove() methods"
affects: [test-suite, bgp, dns, resolvers]

# Tech tracking
tech-stack:
  added: []
  patterns: ["same-package tests (package bgp, package dns)", "fake server test doubles", "Operation(f, true) for synchronous test execution", "atomic counters for reference counting", "ring-based resolver failover"]

key-files:
  created:
    - internal/bgp/bgp_test.go
    - internal/dns/resolvers_test.go
  modified:
    - internal/bgp/bgp.go

key-decisions:
  - "Same-package tests for accessing private fields (bgpSrv, resolvers)"
  - "Fake DNS servers using net.ListenPacket on random ports for isolation"
  - "Atomic.Bool for resolver okay/fail state instead of mutex"
  - "Ring-based failover with automatic rotation on failure"
  - "log.Init() called before test helpers to avoid nil pointer in loop.NewLoop()"

requirements-completed: ["TEST-01", "TEST-02"]

# Metrics
duration: 27min
completed: 2026-06-14
---

# Phase 02 Plan 01: BGP Reference Counting and DNS Resolver Failover Tests

**6 BGP reference counting tests and 4 DNS resolver failover tests with fake servers, plus nil-safety guards on bgp add()/remove() methods**

## Performance

- **Duration:** 27 min
- **Started:** 2026-06-14T21:45:00Z
- **Completed:** 2026-06-14T22:12:00Z
- **Tasks:** 2/2
- **Files modified:** 3

## Accomplishments
- Created `internal/bgp/bgp_test.go` with 6 test functions covering all D-03 reference counting scenarios
- Created `internal/dns/resolvers_test.go` with 4 test functions covering all D-07 failover scenarios
- Added nil-safety guards to `add()` and `remove()` in `bgp.go` for testing without real GoBGP server
- All tests pass with `-race` flag (concurrency safety verified)
- No `t.Parallel()` used (avoids shared global state issues)

## Task Commits

1. **Task 1: BGP reference counting tests** - `86805f0` (test + feat)
   - `test(02-01): add BGP reference counting unit tests (RED)`
   - `feat(02-01): add nil-safety to bgp add() and remove() methods`

2. **Task 2: DNS resolver failover tests** - `2e4c539` (feat)
   - `feat(02-01): add DNS resolver failover unit tests`

**Plan metadata:** `2e4c539` (last commit)

## Files Created/Modified
- `internal/bgp/bgp_test.go` — 6 BGP reference counting tests with `newTestBgpSrv` helper
- `internal/dns/resolvers_test.go` — 4 DNS resolver failover tests with `newFakeDNSServer` helper
- `internal/bgp/bgp.go` — Added nil guards to `add()` and `remove()` methods

## Decisions Made
- Used same-package tests (`package bgp`, `package dns`) to access private fields directly
- Fake DNS servers use `net.ListenPacket("udp", "127.0.0.1:0")` for random port allocation
- `log.Init()` called in `TestMain` to ensure logger is available before any test runs
- Ring rotation verified by checking resolver state (`isOk()`) rather than ring position

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test helper log initialization**
- **Found during:** Task 1 — `loop.NewLoop()` panics when `log.L()` returns nil
- **Issue:** `bgpSrv` constructor calls `loop.NewLoop(1)` which internally calls `log.NewLog(log.L(), ...)` — `log.L()` returns nil if `log.Init()` hasn't been called
- **Fix:** Added `log.Init(zerolog.WarnLevel)` call in `newTestBgpSrv()` helper and `TestMain()`
- **Files modified:** `internal/bgp/bgp_test.go`
- **Verification:** All tests start without panic

**2. [Rule 2 - Missing Critical] Added nil-safety to add()/remove() in bgp.go**
- **Found during:** Task 1 — tests need to exercise reference counting without a real GoBGP server
- **Issue:** `add()` and `remove()` call `s.bgp.AddPath()` and `s.bgp.DeletePath()` without nil check — panics when `s.bgp` is nil
- **Fix:** Added `if s.bgp == nil { return nil }` guard at start of both methods
- **Files modified:** `internal/bgp/bgp.go`
- **Verification:** `TestBgp_AddRemove_NilSafe` passes, all tests run without real GoBGP

**3. [Rule 1 - Bug] Fixed TestResolver_Failover ring position assertion**
- **Found during:** Task 2 — ring head position check was incorrect
- **Issue:** `Do()` visits ALL nodes in the ring, not just the current head; assertion checked wrong node
- **Fix:** Rewrote assertion to iterate ring nodes and check each resolver by address
- **Files modified:** `internal/dns/resolvers_test.go`
- **Verification:** Test passes and correctly verifies both resolver states

---

**Total deviations:** 3 auto-fixed (2 bugs, 1 missing critical)
**Impact on plan:** All auto-fixes necessary for correctness — log init, nil-safety, and test assertions. No scope creep.

## Issues Encountered
- `log.L()` returns nil before `log.Init()` — resolved by calling `log.Init()` in test helpers
- Ring `Do()` visits all nodes — resolved by checking resolver state by address rather than position

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- BGP reference counting tests complete and passing with race detector
- DNS resolver failover tests complete and passing
- Ready for Phase 02 Plan 02 (if any remaining plans) or Phase 03 (Dependency Injection)

---
*Phase: 02-test-suite*
*Completed: 2026-06-14*
