# Phase 1 Revision Summary

**Revision Date:** 2026-06-13
**Trigger:** Plan checker review (01-CHECK.md) — 3 BLOCKERs, 4 WARNINGs, 2 INFO
**Result:** All issues addressed

---

## BLOCKER 1: Verification commands don't validate done criteria

**Status:** FIXED across all 3 plans

**Changes:**

| Plan | Task | Old verify | New verify |
|------|------|-----------|------------|
| 01-01 | Task 1 | `go build ./internal/dns/` | `go build ./internal/dns/` (unchanged — implementation task, no tests) |
| 01-01 | Task 2 | `go build ./internal/dns/` | `go test ./internal/dns/ -run TestServeMux -v -count=1` |
| 01-01 | Task 3 | `go build ./internal/dns/` | `go test ./internal/dns/ -v -count=1 && go test ./internal/dns/ -race -count=1` |
| 01-02 | Task 1 | `go build ./internal/dns/` | `go build ./internal/dns/` (unchanged — implementation task) |
| 01-02 | Task 2 | `go build ./internal/dns/` | `go build ./internal/dns/` (unchanged — implementation task) |
| 01-02 | Task 3 | `go build ./internal/dns/` | `go test ./internal/dns/ -run TestCache -v -count=1` |
| 01-02 | Task 4 | `go build ./internal/dns/` | `go test ./internal/dns/ -v -count=1 && go test ./internal/dns/ -race -count=1` |
| 01-03 | Task 1 | `go build ./internal/dns/` | `go test ./internal/dns/ -run TestRegexIntegration -v -count=1` |
| 01-03 | Task 2 | `go build ./internal/dns/` | `go test ./internal/dns/ -v -count=1 && go test ./internal/dns/ -race -count=1` |

---

## BLOCKER 2: Integration tests lack behavioral assertions

**Status:** FIXED in 01-03-PLAN.md Task 1

**Changes to each integration test:**

| Test | Before | After |
|------|--------|-------|
| LoadRegexPattern | Only checked `generation() > 0` | Also queries mux for "foo.internal.corp." and asserts handler was called |
| InvalidRegexSkippedWithWarning | Only checked `generation() > 0` | Also queries mux for "example.com." and "test.com." to verify they registered |
| WildcardInDomainlist | Only checked `generation() > 0` | Also queries mux for "foo.example.com." to verify wildcard handler matched |
| LoadClearsPreviousState | Only checked `generation() > 0` | Also queries mux for "example.com." before/after reload to verify old handler removed, and "test.com." after to verify new handler present |
| ExactMatchStillWorks | Only checked `generation() > 0` | Also queries mux for "cloudflare.com." and "google.com." to verify exact matches work |

---

## BLOCKER 3: load() clears handlers before re-registering — silent failure risk

**Status:** FIXED in 01-02-PLAN.md Task 1

**Safe reload pattern implemented:**

1. `load()` creates `tempMux := newRegexServeMux()` before scanning the file
2. All entries are registered on `tempMux` (not `c.mux`) via new `registerOn()` and `registerRegexOn()` methods
3. Only after ALL entries are successfully registered: `c.mux = tempMux` (atomic swap)
4. If scan fails mid-way (e.g., invalid FQDN), `c.mux` retains old handlers

**New methods added to cache.go:**
- `registerOn(fqdn string, targetMux *regexServeMux, doLookup bool)` — register handler on specified mux without DNS lookup when doLookup=false
- `registerRegexOn(fqdn string, targetMux *regexServeMux)` — register regex handler on specified mux

**Threat T-01-04 added to threat model:** Documents safe reload mitigation.

---

## WARNING 1: HandleRemove prefix stripping bug

**Status:** FIXED in 01-01-PLAN.md Task 1

**Change:** In HandleRemove, wildcard prefix extraction changed from `pattern[1:]` to `pattern[2:]` to match HandleFunc's 2-character strip of "*.".

**Verification:** TestServeMux_HandleRemoveWildcard now verifies that `HandleRemove("*.example.com")` correctly removes the wildcard registered with `HandleFunc("*.example.com")`.

---

## WARNING 2: Redundant SetMux in newTestCache

**Status:** FIXED in 01-02-PLAN.md Task 3

**Change:** Removed `c.SetMux(newRegexServeMux())` from `newTestCache()` since `newCache()` already initializes `r.mux = newRegexServeMux()`.

---

## WARNING 3: serveMux_test.go wildcard test uses trailing dot

**Status:** FIXED in 01-01-PLAN.md Task 2

**Change:** All test patterns use domain names without trailing dots (e.g., `"example.com"` instead of `"example.com."`), matching real domainlist file entries. The wildcard test uses `"*.example.com"` (no trailing dot).

---

## INFO 1: Test count discrepancy

**Status:** CORRECTED in 01-03-PLAN.md Task 2

**Change:** Test count stated as 25 (6 cache + 12 mux + 7 integration). The checker incorrectly claimed Task 3 adds a test — Task 3 only removes a redundant SetMux call, not adding a new test.

---

## INFO 2: testResponseWriter duplication

**Status:** FIXED in 01-03-PLAN.md Task 1

**Change:** Removed duplicate `testResponseWriter` definition from `regex_integration_test.go`. The helper is already defined in `serveMux_test.go` (same package `dns`), so it's automatically shared.

---

## Files Modified

| File | Changes |
|------|---------|
| `01-01-PLAN.md` | BLOCKER 1 (verify commands), WARNING 1 (HandleRemove fix), WARNING 3 (trailing dot fix) |
| `01-02-PLAN.md` | BLOCKER 1 (verify commands), BLOCKER 3 (safe reload pattern), WARNING 2 (redundant SetMux) |
| `01-03-PLAN.md` | BLOCKER 1 (verify commands), BLOCKER 2 (behavioral assertions), INFO 1 (test count), INFO 2 (testResponseWriter dedup) |
