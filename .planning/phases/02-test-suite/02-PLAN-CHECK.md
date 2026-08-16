---
phase: 02
checker: plan-checker
date: 2026-06-14
verdict: ISSUES_FOUND
plans_checked: 3
issues: 3
---

# Phase 02 — Plan Check Report

**Phase:** 02-test-suite
**Plans verified:** 3 (02-01, 02-02, 02-03)
**Status:** ISSUES FOUND — 1 BLOCKER, 2 WARNING

## Dimension 1: Requirement Coverage

| Requirement | Plan | Tasks | Status |
|-------------|------|-------|--------|
| TEST-01: BGP reference counting | 02-01 | Task 1 | ✅ COVERED |
| TEST-02: DNS resolver failover | 02-01 | Task 2 | ✅ COVERED |
| TEST-03: Cache eviction | 02-02 | Task 1 | ✅ COVERED |
| TEST-04: gRPC CLI lifecycle | 02-02 | Task 2 | ✅ COVERED |
| TEST-05: E2E DNS→cache→BGP | 02-03 | Task 1 | ✅ COVERED |

All 5 requirements mapped. Each requirement appears in at least one plan's `requirements` frontmatter.

## Dimension 2: Task Completeness

| Plan | Task | Files | Action | Verify | Done | Status |
|------|------|-------|--------|--------|------|--------|
| 02-01 | 1 (BGP) | ✅ | ✅ | ✅ | ✅ | Complete |
| 02-01 | 2 (Resolver) | ✅ | ✅ | ✅ | ✅ | Complete |
| 02-02 | 1 (Cache) | ✅ | ✅ | ✅ | ✅ | Complete |
| 02-02 | 2 (gRPC) | ✅ | ✅ | ✅ | ✅ | Complete |
| 02-03 | 1 (E2E) | ✅ | ✅ | ✅ | ✅ | Complete |

All tasks have `<files>`, `<action>`, `<verify>`, and `<done>` elements. All tasks are type `auto` with `tdd=true`.

## Dimension 3: Dependency Correctness

```
Plan 01 (Wave 1) — depends_on: []
Plan 02 (Wave 2) — depends_on: [01]
Plan 03 (Wave 3) — depends_on: [02]
```

- No cycles, no missing references, no future references
- Wave assignments consistent with dependencies
- ✅ PASS

## Dimension 4: Key Links Planned

| Key Link | From | To | Via | Status |
|----------|------|-----|-----|--------|
| BGP ref counting | bgp_test.go | main.go | Operation() spy | ✅ |
| BGP counter inspect | bgp_test.go | main.go | ipRefCounter map | ✅ |
| Resolver failover | resolvers_test.go | resolvers.go | fake DNS server | ✅ |
| Cache eviction | cache_eviction_test.go | cache.go | load() → evictByGeneration() | ✅ |
| gRPC lifecycle | grpc_lifecycle_test.go | cli/cache.go | ephemeral port + gRPC server | ✅ |
| E2E flow | e2e_test.go | cache.go | cache.resolve() | ✅ |
| E2E → BGP | e2e_test.go | bgp/main.go | bgp.Advance() | ⚠️ MISSING |

**Issue:** Plan 03 must_haves.key_links only links `e2e_test.go → cache.go`. The E2E test must also verify `bgp.Advance()` was called (per D-11), but there is no key_link from `e2e_test.go` to `bgp/main.go`.

## Dimension 5: Scope Sanity

| Plan | Tasks | Files | Wave | Status |
|------|-------|-------|------|--------|
| 02-01 | 2 | 2 (bgp_test.go + resolvers_test.go) | 1 | ✅ OK |
| 02-02 | 2 | 2 (cache_eviction_test.go + grpc_lifecycle_test.go) | 2 | ✅ OK |
| 02-03 | 1 | 1 (e2e_test.go) | 3 | ✅ OK |

All plans within 2-3 tasks. No scope warnings.

## Dimension 6: Verification Derivation

### Plan 01 must_haves truths:
1. "BGP reference counting correctly tracks first-reference (add) and last-reference (remove) transitions" — ✅ user-observable
2. "Multi-domain IP sharing increments/decrements counter without calling add/remove between first and last reference" — ✅ user-observable
3. "DNS resolver ring rotates to next resolver on failure and recovers when it succeeds again" — ✅ user-observable
4. "All-resolvers-failure returns a meaningful error" — ✅ user-observable

### Plan 02 must_haves truths:
1. "Cache entries from old generation are removed when a new domainlist file is loaded" — ✅ user-observable
2. "gRPC server starts on a port, accepts RPCs, and shuts down gracefully" — ✅ user-observable

### Plan 03 must_haves truths:
1. "A DNS query for a registered domain triggers cache resolution and BGP advertisement" — ✅ user-observable
2. "Multi-domain IP sharing correctly increments/decrements reference count" — ✅ user-observable

All truths are user-observable behaviors, not implementation details. ✅ PASS

## Dimension 7: Context Compliance

### Locked Decisions (D-01 through D-15):

| Decision | Implementing Task | Status |
|----------|-------------------|--------|
| D-01: Spy on loop.Operation() for BGP | Plan 01 Task 1 | ✅ |
| D-02: Operation(f, true) sync waits | Plan 01 Task 1 | ✅ |
| D-03: 5 BGP scenarios | Plan 01 Task 1 (5 test functions) | ✅ |
| D-04: Verify counter AND add/remove timing | Plan 01 Task 1 | ✅ |
| D-05: Same-package tests in bgp_test.go | Plan 01 Task 1 | ✅ |
| D-06: Fake DNS server for resolver tests | Plan 01 Task 2 | ✅ |
| D-07: 4 resolver scenarios | Plan 01 Task 2 (4 test functions) | ✅ |
| D-08: Test ring position | Plan 01 Task 2 | ✅ |
| D-09: Only test query(), skip proxyQuery() | Plan 01 Task 2 | ✅ |
| D-10: Chain-of-responsibility, skip DNS server | Plan 03 Task 1 | ✅ |
| D-11: Verify Advance() called, skip path attrs | Plan 03 Task 1 | ✅ |
| D-12: Shared test server for E2E | Plan 03 Task 1 | ✅ |
| D-13: Embedded strings for test data | Plan 02 Task 1 | ✅ |
| D-14: Race detection on BGP/loop only | Plan 01 Task 1 | ✅ |
| D-15: Full gRPC lifecycle tests | Plan 02 Task 2 | ✅ |

All 15 locked decisions have implementing tasks. ✅ PASS

### Deferred Ideas:
- Test coverage metrics, GoBGP test harness, property-based testing, CI integration — all correctly excluded from plans. ✅ PASS

### Scope Reduction Detection:
No scope reduction language detected in any plan. All decisions are implemented at full scope. ✅ PASS

## Dimension 8: Nyquist Compliance

### Check 8e — VALIDATION.md Existence:
✅ `02-VALIDATION.md` exists.

### Check 8a — Automated Verify Presence:

| Task | Plan | Wave | Automated Command | Status |
|------|------|------|-------------------|--------|
| 02-01-01 | 01 | 1 | `go test -race -run "TestBgp" ./internal/bgp/` | ✅ |
| 02-01-02 | 01 | 1 | `go test -run "TestResolver" ./internal/dns/` | ✅ |
| 02-02-01 | 02 | 2 | `go test -run "TestCache_GenerationEviction\|..." ./internal/dns/` | ✅ |
| 02-02-02 | 02 | 2 | `go test -run "TestGRPC" ./cmd/bgp-dnsd/cli/` | ✅ |
| 02-03-01 | 03 | 3 | `go test -run "TestE2E" ./internal/dns/` | ✅ |

All tasks have `<automated>` verify commands. ✅ PASS

### Check 8b — Feedback Latency:
- All commands target specific packages (not full suite) — fast execution
- Max estimated: ~15 seconds per package
- No `--watchAll` flags
- ✅ PASS

### Check 8c — Sampling Continuity:
- Wave 1: Tasks 1, 2 — both have automated → 2/2 ✅
- Wave 2: Tasks 1, 2 — both have automated → 2/2 ✅
- Wave 3: Task 1 — has automated → 1/1 ✅
- No 3 consecutive tasks without automated verify
- ✅ PASS

### Check 8d — Wave 0 Completeness:
VALIDATION.md Wave 0 lists 5 files. All are created by the plans:
- `internal/bgp/bgp_test.go` → Plan 01 Task 1 ✅
- `internal/dns/resolvers_test.go` → Plan 01 Task 2 ✅
- `internal/dns/cache_eviction_test.go` → Plan 02 Task 1 ✅
- `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` → Plan 02 Task 2 ✅
- `internal/dns_e2e_test.go` → Plan 03 Task 1 (⚠️ path mismatch, see BLOCKER below)

⚠️ VALIDATION.md Wave 0 references `internal/dns_e2e_test.go` but Plan 03 action text says `internal/dns/e2e_test.go`. The `files_modified` frontmatter has the correct path.

## Dimension 9: Cross-Plan Data Contracts

| Data Entity | Plan A | Plan B | Compatibility | Status |
|-------------|--------|--------|---------------|--------|
| `_bgp` global | Plan 01 (nil-safe add/remove) | Plan 03 (E2E sets _bgp) | Compatible — Plan 01 makes bgp.go nil-safe so Plan 03's E2E tests can set `_bgp` without GoBGP dependency | ✅ |
| `_cache` global | Plan 02 (cache eviction tests) | Plan 03 (E2E uses local cache) | Compatible — Plan 03 creates local cache instances, doesn't modify global `_cache` | ✅ |
| `_resolvers` global | Plan 01 (resolver tests create fresh) | Plan 03 (E2E creates local resolvers) | Compatible — both create fresh instances | ✅ |

No conflicting transformations. ✅ PASS

## Dimension 10: AGENTS.md Compliance

AGENTS.md specifies:
- GSD workflow with planning docs in `.planning/`
- Spike findings skill for bgp-dns

Plans:
- All docs are in `.planning/phases/02-test-suite/` ✅
- Plans reference spike findings via CONTEXT.md canonical refs ✅
- No forbidden patterns introduced ✅

✅ PASS

## Dimension 11: Research Resolution

RESEARCH.md has `## Open Questions` section:
1. "How to handle bgpSrv.bgp (the GoBGP server) in tests?" → RESOLVED: Test ref counting only, skip GoBGP API calls, make add()/remove() nil-safe
2. "How to test bgp.Advance()/bgp.Withdraw() package-level functions?" → RESOLVED: newTestBgpSrv() helper, Operation() spy pattern
3. "Should resolver tests use dns.Exchange() or dns.Client{}.Exchange()?" → RESOLVED: Test query() directly (D-09)

All open questions have resolution recommendations. ✅ PASS

## Dimension 12: Pattern Compliance

PATTERNS.md maps all 5 new test files to exact analogs:
- `bgp_test.go` → `cache_test.go` (newTestXxx helper pattern) ✅
- `resolvers_test.go` → `serveMux_test.go` (testResponseWriter mock) ✅
- `cache_eviction_test.go` → `cache_test.go` (newTestCache pattern) ✅
- `grpc_lifecycle_test.go` → `cache_test.go` in cli/ (existing test) ✅
- `dns_e2e_test.go` → `regex_integration_test.go` (integration pattern) ✅

✅ PASS

---

## Issues Found

### BLOCKER #1: E2E Test File Path Mismatch (Plan 03)

```yaml
issue:
  plan: "02-03"
  dimension: task_completeness
  severity: blocker
  description: |
    Plan 03 action text says "Create `internal/dns/e2e_test.go` with package `dns`" but
    RESEARCH.md Section "Recommended Project Structure", PATTERNS.md, and VALIDATION.md Wave 0
    all specify `internal/dns_e2e_test.go` (underscore, in `internal` package, not `internal/dns/`).
    
    This is a critical path mismatch. The underscore placement determines the package:
    - `internal/dns_e2e_test.go` → package `internal` (can access internal/dns AND internal/bgp internals)
    - `internal/dns/e2e_test.go` → package `dns` (can only access internal/dns internals)
    
    The E2E test must verify `ipRefCounter` from the bgp package (per D-11: "Just verify bgp.Advance()
    was called with correct IPs"). This requires access to `internal/bgp` private fields, which is only
    possible from package `internal`, not package `dns`.
    
    The `files_modified` frontmatter correctly lists `internal/dns_e2e_test.go`, but the action text
    and must_haves.artifacts.path say `internal/dns/e2e_test.go` — these must be unified.
  task: 1
  fix_hint: |
    Change action text from "Create `internal/dns/e2e_test.go` with package `dns`" to
    "Create `internal/dns_e2e_test.go` with package `internal`". Also update must_haves.artifacts
    path from `internal/dns/e2e_test.go` to `internal/dns_e2e_test.go`.
```

### WARNING #1: Plan 03 Key Links Incomplete

```yaml
issue:
  plan: "02-03"
  dimension: key_links_planned
  severity: warning
  description: |
    Plan 03 must_haves.key_links only includes the link from e2e_test.go to cache.go
    (via cache.resolve()). The second critical wiring — from e2e_test.go to bgp/main.go
    (via bgp.Advance() call verification) — is missing.
    
    Per D-11: "Just verify bgp.Advance() was called with correct IPs." The E2E test
    MUST check that bgp.Advance() was invoked, which means verifying ipRefCounter
    state in the bgp package. This is a key link that should be documented.
  fix_hint: |
    Add a key_link entry:
    - from: internal/dns_e2e_test.go
      to: internal/bgp/main.go
      via: "bgp.Advance() called from cache.upsert() with resolved IPs"
      pattern: "bgp\\.Advance\\("
```

### WARNING #2: Plan 02 Must-Haves Missing TEST-03 Truth

```yaml
issue:
  plan: "02-02"
  dimension: verification_derivation
  severity: warning
  description: |
    Plan 02 requirements frontmatter lists both TEST-03 (cache eviction) and TEST-04 (gRPC lifecycle),
    but must_haves.truths only includes the gRPC lifecycle truth:
    "gRPC server starts on a port, accepts RPCs, and shuts down gracefully"
    
    The cache eviction truth ("Cache entries from old generation are removed when a new domainlist
    file is loaded") is present but the test also covers TTL expiration and capacity limits,
    which are not reflected in the must_haves.
  fix_hint: |
    Add cache-specific truths to must_haves:
    - "Cache entries expire after their TTL duration"
    - "Cache respects maximum entry capacity via LFU eviction"
```

### INFO #1: Plan 02 Task 1 Behavior Mentions FakeClock

```yaml
issue:
  plan: "02-02"
  dimension: task_completeness
  severity: info
  description: |
    Task 1 behavior section says "advance time past TTL" but RESEARCH.md Assumption A2
    warns that gcache v0.0.2 FakeClock availability was uncertain. The action text correctly
    falls back to time.Sleep(), but the behavior description is slightly misleading.
  fix_hint: |
    Update behavior to say "entries evicted after TTL duration (using time.Sleep)" to match
    the actual action implementation, avoiding confusion about FakeClock availability.
```

---

## Issues Found — RESOLVED

### BLOCKER #1: E2E Test File Path Mismatch — ✅ RESOLVED

Fixed in commit `366c479`:
- Changed all occurrences of `internal/dns/e2e_test.go` to `internal/dns_e2e_test.go`
- Changed package from `dns` to `internal`
- Updated files_modified frontmatter, must_haves.artifacts.path, key_links.from, task files, action text, success criteria, Artifacts table, and verify/done commands

### WARNING #1: Plan 03 Key Links Incomplete — ✅ RESOLVED

The key_link from e2e_test.go to bgp/main.go was already present in the original plan (lines 27-30). Only the file path needed correction, which was done in the BLOCKER fix.

### WARNING #2: Plan 02 Must-Haves Missing TEST-03 Truth — ✅ RESOLVED

Fixed in commit `366c479`: Added two cache-specific truths to Plan 02 must_haves:
- "Cache entries expire after their TTL duration"
- "Cache respects maximum entry capacity via LFU eviction"

### INFO #1: Plan 02 Task 1 Behavior Mentions FakeClock — ✅ RESOLVED

Fixed in commit `366c479`: Updated behavior description to "entries evicted after TTL duration (using time.Sleep)" and action text to clarify gcache FakeClock availability is uncertain.

---

## Summary

| Dimension | Status |
|-----------|--------|
| 1. Requirement Coverage | ✅ PASS |
| 2. Task Completeness | ✅ PASS |
| 3. Dependency Correctness | ✅ PASS |
| 4. Key Links Planned | ✅ PASS |
| 5. Scope Sanity | ✅ PASS |
| 6. Verification Derivation | ✅ PASS |
| 7. Context Compliance | ✅ PASS |
| 8. Nyquist Compliance | ✅ PASS |
| 9. Cross-Plan Data Contracts | ✅ PASS |
| 10. AGENTS.md Compliance | ✅ PASS |
| 11. Research Resolution | ✅ PASS |
| 12. Pattern Compliance | ✅ PASS |

**Verdict: PASS** — All issues resolved in commit `366c479`. Plans are ready for execution.
