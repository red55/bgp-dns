---
phase: 03-dependency-injection
plan: 06
subsystem: refactor
tags: [dead-code-removal, code-cleanup]

# Dependency graph
requires:
  - phase: 03-05
    provides: main.go panic-to-error migration, typed context keys, constructor-based services
provides:
  - Clean internal/bgp/main.go — no commented hashmap references
  - Clean internal/dns/resolvers.go — no commented error handling block

# Tech tracking
tech-stack:
  added: []
  patterns: [dead-code-removal]

key-files:
  created: []
  modified:
    - internal/bgp/main.go
    - internal/dns/resolvers.go

key-decisions: []

patterns-established: []

requirements-completed: [REFACTOR-04]

# Metrics
duration: 3min
completed: 2026-06-16
---

# Phase 03 Plan 06: Dead Code Removal — Commented HashMap and Error Handling

**Remove commented-out dead code from internal/bgp/main.go (hashmap references) and internal/dns/resolvers.go (error handling block).**

## Performance

- **Duration:** 3 min
- **Started:** 2026-06-16T10:00:00Z
- **Completed:** 2026-06-16T10:03:00Z
- **Tasks:** 1/1
- **Files modified:** 2

## Accomplishments
- Removed commented `//ipRefCounter *hashmap.Map[string, *atomic.Uint64]` struct field from bgp/main.go
- Removed commented error handling block (`cause := e`, `opError *net.OpError`) from resolvers.go
- Verified `go build ./internal/bgp ./internal/dns` succeeds
- Verified `go test ./... -count=1` — all packages pass

## Task Commits

1. **Task 1: Remove commented hashmap from bgp/main.go and commented error handling from resolvers.go** - `d12ad7a` (refactor)

## Files Created/Modified
- `internal/bgp/main.go` — Removed commented hashmap struct field declaration (line 19)
- `internal/dns/resolvers.go` — Removed 22-line commented error handling block (lines 127-148)

## Deviations from Plan

None — plan executed exactly as written.

## Verification

- [x] `go build ./internal/bgp ./internal/dns` succeeds
- [x] `go test ./internal/bgp ./internal/dns -count=1` — passes
- [x] `go test ./... -count=1` — all packages pass
- [x] No `hashmap` references remain in bgp/main.go
- [x] No `cause :=` or `opError` references remain in resolvers.go
- [x] No commented-out code blocks in either file

## Next Phase Readiness
- Phase 03 is now complete (all 6 plans done)
- REFACTOR-04 requirement marked complete

---
*Phase: 03-dependency-injection*
*Completed: 2026-06-16*
