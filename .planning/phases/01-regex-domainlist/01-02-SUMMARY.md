---
phase: 01-regex-domainlist
plan: 02
subsystem: dns
tags: [regex, mux, safe-reload, dns-handlers]

# Dependency graph
requires:
  - phase: 01-regex-domainlist
    provides: regexServeMux foundation (exact/wildcard/regex/catch-all routing)
provides:
  - cache struct with mux field, SetMux method, registerRegex method; register/unregister delegate to mux
  - main.go Serve() creates mux, wires to cache and server; Shutdown() clears mux
  - safe reload pattern: builds tempMux first, swaps atomically on success
  - regex: prefix detection in domainlist loader with invalid pattern skip
affects: [domainlist reload, fswatcher, bgp advance/withdraw]

# Tech tracking
tech-stack:
  added: []
  patterns: [safe-reload, mux-delegation, generation-based-eviction]

key-files:
  created: []
  modified:
    - internal/dns/cache.go
    - internal/dns/main.go

key-decisions:
  - "Safe reload: increment generation BEFORE loading entries so new entries get current gen and aren't evicted by old-gen sweep"
  - "registerOn/registerRegexOn accept targetMux parameter for tempMux building during safe reload"
  - "doLookup=false during safe reload to avoid redundant DNS queries on reload"
  - "Invalid regex patterns produce Warn log and continue (not return) to avoid blocking domainlist reload"

patterns-established:
  - "Safe reload pattern: build on tempMux first, swap only on success, evict old generation"
  - "Mux delegation: cache.register() → registerOn(targetMux, doLookup), cache.unregister() → c.mux.HandleRemove()"
  - "Generation tracking: increment before load, evict after swap"

requirements-completed: [REGEX-01, REGEX-02, REGEX-03, REGEX-04, REGEX-05]

# Metrics
duration: 5min
completed: 2026-06-13
---

# Phase 01-02: Cache and Main Integration Summary

**Wired regexServeMux into cache and main, replaced all global dns.HandleFunc/dns.HandleRemove with mux delegation, and implemented safe reload pattern (tempMux build + atomic swap).**

## Performance

- **Duration:** 5 min
- **Started:** 2026-06-13T16:40:00Z
- **Completed:** 2026-06-13T16:46:49Z
- **Tasks:** 3 (01-02.1, 01-02.2, 01-02.3 — no-op)
- **Files modified:** 2

## Accomplishments

1. **cache.go** — Injected `mux *regexServeMux` into cache struct, delegated `register()`/`unregister()` to mux, added `registerOn()`/`registerRegexOn()` for safe reload, implemented safe reload pattern in `load()` with tempMux build + atomic swap.
2. **main.go** — Created mux in `Serve()`, injected into cache via `SetMux()`, set catchAll to `proxyQuery`, assigned mux as `server.Handler`, updated `Shutdown()` to call `mux.clear()`.
3. **cache_test.go** — Already clean (no redundant SetMux call needed).

## Task Commits

Each task was committed atomically:

1. **Task 01-02.1: Modify cache.go** — `3cf214f` (refactor)
   - Added mux field, SetMux, registerRegex, registerOn, registerRegexOn methods
   - Replaced dns.HandleFunc/dns.HandleRemove with c.mux delegation
   - Implemented safe reload in load() with tempMux + atomic swap
2. **Task 01-02.2: Modify main.go** — `8cb16c1` (refactor)
   - Created mux in Serve(), injected into cache, set catchAll, assigned to server.Handler
   - Replaced dns.HandleFunc(".", proxyQuery) with Handler: mux
   - Updated Shutdown() to call _cache.mux.clear()
3. **Task 01-02.3: Update cache_test.go** — No changes needed (SetMux already removed)
4. **Task 01-02.4: Wave 2 verification** — All 30 tests pass, race detector clean

**Plan metadata:** Wave 2 complete

## Files Created/Modified

- `internal/dns/cache.go` — Added mux field, SetMux, registerRegex, registerOn, registerRegexOn; replaced global handler calls with mux delegation; implemented safe reload
- `internal/dns/main.go` — Created mux in Serve(), injected into cache, set catchAll, assigned to server.Handler; updated Shutdown() to clear mux

## Decisions Made

- **Safe reload generation ordering:** Increment generation BEFORE loading entries (not after swap). This ensures new entries are tagged with the current generation and won't be evicted by the old-generation sweep. The original plan's "swap then increment" pattern would cause new entries to be evicted.
- **registerOn/registerRegexOn with targetMux:** Split registration into generic methods that accept a target mux, enabling safe reload to build on tempMux while production register/unregister use c.mux.
- **doLookup=false during safe reload:** Skip redundant DNS lookups during domainlist reload since the cache already has entries from the previous load.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed safe reload generation ordering**
- **Found during:** Task 01-02.1 implementation
- **Issue:** Plan specified increment generation AFTER swap (`c.mux = tempMux; _ = c.increaseGeneration()`). This would tag new entries with the old generation number, causing `evictByGeneration(oldGen)` to evict them immediately.
- **Fix:** Moved `_ = c.increaseGeneration()` BEFORE loading entries. New entries get the current generation and are preserved during eviction.
- **Files modified:** internal/dns/cache.go
- **Verification:** All 6 cache tests pass, generation increase test confirms correct behavior

**2. [Rule 1 - Bug] Fixed registerRegexOn to always trim prefix**
- **Found during:** Task 01-02.1 implementation
- **Issue:** Plan's registerRegexOn checked `strings.HasPrefix(fqdn, "regex:")` before trimming — if called with a pattern that already had the prefix stripped, it would error.
- **Fix:** Always trim prefix unconditionally: `pattern := strings.TrimPrefix(fqdn, "regex:")` — idempotent whether or not prefix is present.
- **Files modified:** internal/dns/cache.go
- **Verification:** Build passes, registerRegexOn works correctly

**3. [Rule 2 - Missing Critical] load() passes full line to registerRegexOn**
- **Found during:** Task 01-02.1 implementation
- **Issue:** Plan showed `c.registerRegexOn(line, tempMux)` where `line` includes "regex:" prefix. registerRegexOn correctly handles this by trimming the prefix internally.
- **Fix:** No change needed — registerRegexOn already does `strings.TrimPrefix(fqdn, "regex:")`.
- **Files modified:** (none — verified correct)

**Total deviations:** 3 auto-fixed (2 bugs, 1 verification)
**Impact on plan:** All deviations were necessary for correctness. The generation ordering fix was critical — without it, safe reload would evict newly loaded entries immediately.

## Issues Encountered

- None — plan executed as specified (with the 3 auto-fixes above for correctness).

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./internal/dns/` | PASS |
| `go test ./internal/dns/ -v -count=1` (30 tests) | PASS |
| `go test ./internal/dns/ -race -count=1` | PASS |
| `dns.HandleFunc`/`dns.HandleRemove` in cache.go | 0 matches |
| `dns.HandleFunc`/`dns.HandleRemove` in main.go | 0 matches |
| Mux wiring calls in main.go | 5 matches |

## Next Phase Readiness

- Plan 01-03 (Wave 3) is ready — it depends on 01-02 for the mux integration.
- Safe reload pattern is in place, preventing handler loss during domainlist file changes.
- All 5 requirements (REGEX-01 through REGEX-05) are complete.

---
*Phase: 01-regex-domainlist*
*Completed: 2026-06-13*
