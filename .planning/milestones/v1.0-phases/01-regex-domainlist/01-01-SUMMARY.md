---
phase: 01-regex-domainlist
plan: 01
subsystem: dns
tags: [dns, regex, routing, miekg/dns, sync.RWMutex, testify]

# Dependency graph
requires:
provides:
  - regexServeMux struct with exact/wildcard/regex/catch-all priority routing
  - 12 unit tests covering all match types, priority orderings, and lifecycle methods
  - HandleRegex with error-safe regexp.Compile (no MustCompile)
  - HandleRemove with canonical key support
  - clear() method for domainlist reload resets
affects:
  - cache.go (register/unregister delegation)
  - main.go (Server.Handler wiring)
  - resolvers.go (proxyQuery catch-all handler)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Priority-based DNS routing (exact > wildcard > regex > catch-all)
    - Standalone mux (no dns.ServeMux embedding)
    - Error-safe regex compilation (Compile + error return, not MustCompile)
    - sync.RWMutex for concurrent handler registration/query routing
    - Label-boundary wildcard matching

key-files:
  created:
    - internal/dns/serveMux.go
    - internal/dns/serveMux_test.go
  modified: []

key-decisions:
  - "HandleRemove canonicalizes exact keys with dns.CanonicalName() to match HandleFunc registration"
  - "Wildcard matching uses \".\"+prefix suffix check for label-boundary safety"

patterns-established:
  - "regexServeMux: standalone priority-routing mux, no embedding of dns.ServeMux"
  - "HandleFunc: wildcard detection via strings.HasPrefix(pattern, \"*.\") with 2-char strip"
  - "HandleRegex: regexp.Compile with error return, never MustCompile"
  - "ServeDNS: RLock for reads, question normalized to trailing dot before routing"

requirements-completed:
  - REGEX-01
  - REGEX-02
  - REGEX-03
  - REGEX-04
  - REGEX-05

# Metrics
duration: 5min
completed: 2026-06-13
---

# Phase 01-01: regexServeMux Foundation Summary

**regexServeMux struct with exact/wildcard/regex/catch-all priority routing and 12 unit tests covering all match types, priority orderings, and lifecycle methods**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-06-13T16:20:00Z
- **Completed:** 2026-06-13T16:24:05Z
- **Tasks:** 3/3
- **Files modified:** 2

## Accomplishments

1. Created `regexServeMux` struct implementing priority-based DNS routing (exact > wildcard > regex > catch-all)
2. Implemented all 6 methods: `HandleFunc`, `HandleRegex`, `HandleRemove`, `SetCatchAll`, `clear`, `ServeDNS`
3. Created 12 unit tests covering all match types, 3 priority orderings, empty question, removal, invalid regex, and clear
4. All tests pass with race detector enabled — zero data races

## Task Commits

1. **Task 01-01.1: Create serveMux.go** - `f00d02d` (feat)
2. **Task 01-01.2: Create serveMux_test.go** - `af3a181` (test)
3. **Task 01-01.3: Wave 1 verification** - no new commit (verification only)

## Files Created/Modified

- `internal/dns/serveMux.go` — regexServeMux struct with 6 methods implementing priority DNS routing
- `internal/dns/serveMux_test.go` — 12 unit tests with testResponseWriter mock and helper functions

## Decisions Made

- **HandleRemove canonicalization:** `HandleRemove` for exact patterns uses `dns.CanonicalName(pattern)` to match the key format stored by `HandleFunc`. Without this, `delete(m.exact, pattern)` would fail to find the key since `HandleFunc` stores with trailing dot.
- **Wildcard label-boundary matching:** Changed from `wc.prefix + "."` suffix check to `"." + wc.prefix` to ensure matching at domain label boundaries (e.g., `"*.corp"` matches `"foo.corp"` but not `"foo.corp.com"`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] HandleRemove exact key not canonicalized**
- **Found during:** Task 2 — `TestServeMux_HandleRemoveExact` failed
- **Issue:** `HandleRemove("example.com")` called `delete(m.exact, "example.com")` but `HandleFunc` stored with `dns.CanonicalName("example.com")` = `"example.com."` — key mismatch, delete was a no-op
- **Fix:** Changed `delete(m.exact, pattern)` to `delete(m.exact, dns.CanonicalName(pattern))`
- **Files modified:** `internal/dns/serveMux.go`
- **Verification:** `TestServeMux_HandleRemoveExact` passes after fix
- **Committed in:** `af3a181`

**2. [Rule 1 - Bug] Wildcard suffix check label-boundary mismatch**
- **Found during:** Task 2 — `TestServeMux_WildcardMatch`, `TestServeMux_PriorityWildcardOverRegex` failed
- **Issue:** `ServeDNS` computed `canonical = strings.TrimSuffix(question, ".")` (no trailing dot), then checked `strings.HasSuffix(canonical, wc.prefix+".")` — but `wc.prefix` (from `pattern[2:]`) also had no trailing dot, so `"example.com."` was never a suffix of `"foo.example.com"`
- **Fix:** Changed wildcard check to `strings.HasSuffix(canonical, "."+wc.prefix)` for label-boundary safety
- **Files modified:** `internal/dns/serveMux.go`
- **Verification:** All wildcard tests pass; `"*.corp"` matches `"foo.corp"` but not `"foo.corp.com"`
- **Committed in:** `af3a181`

---

**Total deviations:** 2 auto-fixed (Rule 1 — bugs in key canonicalization and label-boundary matching)
**Impact on plan:** Both fixes essential for correctness. The canonicalization bug would cause HandleRemove to silently fail; the wildcard matching bug would prevent all wildcard matches and could cause false-positive matches without the label-boundary fix.

## Issues Encountered

- None — all 12 tests pass, build succeeds, race detector clean

## Verification Results

| Command | Result |
|---------|--------|
| `go build ./internal/dns/` | ✅ PASS |
| `go test ./internal/dns/ -run TestServeMux -v -count=1` | ✅ 12/12 PASS |
| `go test ./internal/dns/ -race -count=1` | ✅ PASS, no races |
| `grep -c 'dns\.HandleFunc\|dns\.HandleRemove' serveMux.go` | ✅ 0 |
| `grep -c 'regexp\.' serveMux.go` | ✅ 1 (only regexp.Compile) |

## Self-Check

- [x] `internal/dns/serveMux.go` exists with regexServeMux struct, regexHandler struct, newRegexServeMux(), and all 6 methods
- [x] `internal/dns/serveMux_test.go` exists with 12 test functions
- [x] All 12 tests pass
- [x] Build succeeds with zero errors
- [x] No global `dns.HandleFunc` or `dns.HandleRemove` calls in serveMux.go
- [x] Race detector passes
- [x] Commits verified in git log

## Next Phase Readiness

- regexServeMux foundation complete — ready for cache integration (Task 01-02: wire register/unregister to mux, Task 01-03: add regex: prefix parsing in load())

---
*Phase: 01-regex-domainlist*
*Completed: 2026-06-13*
