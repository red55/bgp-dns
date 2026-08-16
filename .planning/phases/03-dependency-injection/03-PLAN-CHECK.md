# Phase 3: Dependency Injection — Plan Verification Report

**Verification Date:** 2026-06-16
**Phase:** 03-dependency-injection
**Plans Verified:** 6 (03-01 through 03-06)
**Status:** PASS (all issues resolved)

---

## Executive Summary

The plans correctly implement the per-package constructor pattern (D-01), typed context key (D-05), explicit logger injection (D-06), and backward-compatible wrappers (D-04/D-07). All 4 requirements (REFACTOR-01 through REFACTOR-04) and all 9 decisions (D-01 through D-09) are addressed across the 6 plans.

All issues have been resolved:

1. **Plan 01 Task 2** — Added `cfg *config.AppCfg` field to `fsWatcher` struct in Plan 01 (Wave 0), enabling `fswatcher/loop.go` to use `w.cfg` immediately. Plan 04 (Wave 3) no longer adds the field (already present).
2. **Plan 02 Task 1** — Updated `newCache` signature to accept `loop loop.Loop` parameter. `NewDns` now passes injected loop to `newCache(cfg, ..., l)`.
3. **RESEARCH.md** — `## Open Questions` section renamed to `## Open Questions (RESOLVED)` with all 6 questions marked as resolved.

---

## Dimension 1: Requirement Coverage

| Requirement | Description | Covering Plans | Status |
|-------------|-------------|----------------|--------|
| REFACTOR-01 | Package-level singleton variables → struct parameters | 01, 02, 03, 04 | ✅ Covered |
| REFACTOR-02 | Context-based config passing with typed key | 01, 02, 03, 04, 05 | ✅ Covered |
| REFACTOR-03 | Panic-on-init → error returns | 05 | ✅ Covered |
| REFACTOR-04 | Commented-out dead code removed | 06 | ✅ Covered |

All 4 requirements have explicit task coverage. No gaps.

---

## Dimension 2: Task Completeness

All 15 tasks across 6 plans have `<files>`, `<action>`, `<verify>`, and `<done>` elements. All tasks are type `auto` with complete required fields.

| Plan | Tasks | All Have Files | All Have Action | All Have Verify | All Have Done |
|------|-------|----------------|-----------------|-----------------|---------------|
| 01 | 3 | ✅ | ✅ | ✅ | ✅ |
| 02 | 3 | ✅ | ✅ | ✅ | ✅ |
| 03 | 3 | ✅ | ✅ | ✅ | ✅ |
| 04 | 2 | ✅ | ✅ | ✅ | ✅ |
| 05 | 1 | ✅ | ✅ | ✅ | ✅ |
| 06 | 1 | ✅ | ✅ | ✅ | ✅ |

---

## Dimension 3: Dependency Correctness

### Dependency Graph
```
Plan 01 (Wave 0) — no deps
  ├── Plan 02 (Wave 1) — depends on 01
  ├── Plan 03 (Wave 2) — depends on 01
  └── Plan 04 (Wave 3) — depends on 01
        │
        ▼
Plan 05 (Wave 4) — depends on 02, 03, 04
        │
        ▼
Plan 06 (Wave 5) — depends on 05
```

### Issues

**ISSUE: Wave numbers in frontmatter are incorrect for Plans 05 and 06.**
- Plan 05 frontmatter says `wave: 4` but dependency `depends_on: ["03-02", "03-03", "03-04"]` (all Wave 1) → should be `wave: 2`
- Plan 06 frontmatter says `wave: 5` but dependency `depends_on: ["03-05"]` (Wave 4 per frontmatter) → should be `wave: 3`
- The wave numbers are off by 2, likely due to miscounting. The dependency DAG is valid; only the numeric labels are wrong.

---

## Dimension 4: Key Links Planned

All plans define `key_links` in frontmatter connecting artifacts. Critical wiring paths verified:

| Link | Source | Target | Method | Status |
|------|--------|--------|--------|--------|
| configKey | Plan 01 | Plan 02,03,04,05 | ctx.Value(configKey{}) | ✅ |
| NewDns → cache | Plan 02 | cache.go | newCache(cfg, ...) | ⚠️ See Dimension 5 |
| NewBgp → bgp.go | Plan 03 | bgp.go | bgpSrv.add/remove | ✅ |
| NewFsWatcher → loop.go | Plan 04 | loop.go | w.cfg | ⚠️ See Dimension 5 |
| main.go → all services | Plan 05 | dns/bgp/fswatcher | NewXxx constructors | ✅ |
| main.go → configKey | Plan 05 | config package | typed key | ✅ |

---

## Dimension 5: Dependency Correctness — Critical Blocking Issues

### ⛔ BLOCKER 1: Plan 01 Task 2 breaks fswatcher compilation

**Issue:** Plan 01 Task 2 modifies `internal/fswatcher/loop.go` to replace `ctx.Value("cfg")` with `w.cfg`. However, the `cfg` field is only added to the `fsWatcher` struct in **Plan 04 Task 1** — four waves later.

**Evidence:**
- Plan 01 Task 2 action step 3a: "Replace `var cfg = ctx.Value("cfg")` with direct use of `w.cfg`"
- Plan 01 Task 2 action step 3c: "The w.cfg field must be set by the constructor (done in Wave 3)" — acknowledges the field is missing
- Plan 04 Task 1 action step 1: Adds `cfg *config.AppCfg` to `fsWatcher` struct
- Plan 01 files_modified includes `internal/fswatcher/loop.go` but NOT `internal/fswatcher/main.go`

**Impact:** After Plan 01 executes, `go build ./internal/fswatcher` will fail because `w.cfg` references a non-existent field.

**Fix:** Add `cfg *config.AppCfg` field to the `fsWatcher` struct in Plan 01 Task 2, and include `internal/fswatcher/main.go` in Plan 01's `files_modified`. The field addition is a simple struct change that has no dependencies on other plans.

### ⛔ BLOCKER 2: Plan 02 does not update `newCache` to accept `loop.Loop` parameter

**Issue:** Plan 02 Task 1's `NewDns` constructor receives a `loop.Loop` parameter but never passes it to `newCache`. Instead, `newCache` creates its own loop internally via `loop.NewLoop(1)` — which now requires a logger parameter (changed in Plan 01).

**Evidence:**
- Plan 02 Task 1 action: `s.cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, s.resolvers, logger, cfg)`
- Current `newCache` signature (cache.go:36): `func newCache(max int, minTtl time.Duration, rs *resolvers, l *zerolog.Logger) (r *cache)`
- Current `newCache` body (cache.go:38): `Loop: loop.NewLoop(1)` — no logger parameter, but Plan 01 changed NewLoop to require one
- Plan 02 Task 3 acceptance criteria: Does NOT mention updating `newCache` signature to accept `loop.Loop`
- Plan 02 files_modified: Lists `internal/dns/cache.go` but Task 3 only mentions updating `cache.loop`, not `newCache`

**Impact:** After Plan 01, `loop.NewLoop(1)` in `newCache` will fail to compile because the new `NewLoop` signature requires a logger parameter. Plan 02's Task 3 does not fix this.

**Fix:** Update Plan 02 Task 3 to:
1. Change `newCache` signature to accept `loop.Loop` as first parameter
2. Change `NewDns` to pass the received `l` parameter to `newCache`: `newCache(l, cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, s.resolvers, logger, cfg)`
3. Update `newCache` body to use the injected loop: `Loop: l` instead of `loop.NewLoop(1)`
4. Update all test call sites to pass the loop parameter

### ⛔ BLOCKER 3: RESEARCH.md Open Questions not marked resolved

**Issue:** RESEARCH.md line 570 has `## Open Questions` without `(RESOLVED)` suffix. Three questions are listed with recommendations but the section is not formally marked as resolved.

**Fix:** Change heading to `## Open Questions (RESOLVED)` and mark each question with `RESOLVED:` inline.

---

## Dimension 6: Verification Derivation

### Plan 01 must_haves
- ✅ "Config package exports typed configKey struct" — user-observable
- ✅ "Loop.NewLoop() accepts explicit *zerolog.Logger parameter" — user-observable
- ✅ "All test files compile and pass after signature changes" — user-observable

### Plan 02 must_haves
- ✅ "dns.Service struct holds all DNS subsystem state" — user-observable
- ✅ "Phase 2 tests pass without modification" — user-observable

### Plan 03 must_haves
- ✅ "NewBgp(cfg, loop, logger) creates BGP service and returns (Service, error)" — user-observable

### Plan 04 must_haves
- ✅ "NewFsWatcher(cfg, loop, logger) creates FSWatcher service and returns (Service, error)" — user-observable

### Plan 05 must_haves
- ✅ "All 7 panic() calls replaced with error handling and os.Exit" — user-observable
- ✅ "Shutdown order: BGP first, then DNS, then FSWatcher" — user-observable

### Plan 06 must_haves
- ✅ "Commented hashmap line removed from bgp/main.go" — verifiable
- ✅ "All tests still pass after dead code removal" — user-observable

All truths are user-observable, not implementation details.

---

## Dimension 7: Context Compliance

### Decision Coverage

| Decision | Description | Implementing Task(s) | Status |
|----------|-------------|----------------------|--------|
| D-01 | Per-package constructors | Plan 02 Task 1, Plan 03 Task 1, Plan 04 Task 1 | ✅ |
| D-02 | Constructor returns (Service, error) | All plan constructors | ✅ |
| D-03 | Return type is struct with methods | Plan 02 Service, Plan 03 bgpSrv, Plan 04 fsWatcher | ✅ |
| D-04 | Keep thin wrapper functions | Plan 02 Serve/Shutdown, Plan 03 Serve/Shutdown, Plan 04 Serve/Shutdown | ✅ |
| D-05 | Typed context key | Plan 01 Task 1 (configKey definition), Plans 02-05 (usage) | ✅ |
| D-06 | Explicit logger parameter | Plan 01 Task 1 (NewLoop, newResolvers), Plans 02-05 (usage) | ✅ |
| D-07 | Backward compatibility wrappers | Plans 02, 03, 04 (Serve wrappers) | ✅ |
| D-08 | Remove commented error handling in resolvers.go | Plan 06 Task 1 | ✅ |
| D-09 | Remove commented hashmap in bgp/main.go | Plan 06 Task 1 | ✅ |

### Deferred Ideas Check
- ✅ Full global state removal excluded (wrappers retained)
- ✅ Interface-based DI excluded (struct embedding used)
- ✅ Config validation excluded (deferred to Phase 4)
- ✅ Multi-instance support excluded

### CLI Package Check
- ✅ CLI package correctly excluded from scope (Plan 05 uses `cli.Serve()` and `cli.Shutdown()` without modification)

---

## Dimension 7b: Scope Reduction Detection

No scope reduction language detected in any plan task. All decisions are implemented at full scope.

---

## Dimension 7c: Architectural Tier Compliance

**RESEARCH.md has `## Architectural Responsibility Map` section.**

| Plan | Task | Capability | Expected Tier | Actual Tier | Status |
|------|------|-----------|---------------|-------------|--------|
| 01 | 1 | Typed configKey | internal/config | internal/config | ✅ |
| 01 | 1 | Loop logger injection | internal/loop | internal/loop | ✅ |
| 01 | 1 | Resolvers logger injection | internal/dns | internal/dns | ✅ |
| 01 | 2 | Cache config field | internal/dns | internal/dns | ✅ |
| 02 | 1 | DNS Service struct | internal/dns | internal/dns | ✅ |
| 03 | 1 | BGP Service constructor | internal/bgp | internal/bgp | ✅ |
| 04 | 1 | FSWatcher Service struct | internal/fswatcher | internal/fswatcher | ✅ |
| 05 | 1 | main.go startup orchestration | cmd/bgp-dnsd | cmd/bgp-dnsd | ✅ |
| 06 | 1 | Dead code removal | internal/bgp, internal/dns | internal/bgp, internal/dns | ✅ |

All tasks correctly assign work to the tiers defined in the responsibility map.

---

## Dimension 8: Nyquist Compliance

### VALIDATION.md Status
- File exists: ✅ `/wsl.localhost/Ubuntu-24.04/home/lsk/src/red55/bgp-dns/.planning/phases/03-dependency-injection/03-VALIDATION.md`

### Check 8a — Automated Verify Presence

| Task | Plan | Wave | Has `<automated>` | Wave 0 Dependency | Status |
|------|------|------|-------------------|-------------------|--------|
| 01-Task 1 | 01 | 0 | ✅ `go build ./...` | — | ✅ |
| 01-Task 2 | 01 | 0 | ✅ `go build ./...` | — | ✅ |
| 01-Task 3 | 01 | 0 | ✅ `go test ./... -race` | — | ✅ |
| 02-Task 1 | 02 | 1 | ✅ `go build ./internal/dns` | — | ✅ |
| 02-Task 2 | 02 | 1 | ✅ `go build ./internal/dns` | — | ✅ |
| 02-Task 3 | 02 | 1 | ✅ `go build ./internal/dns` | — | ✅ |
| 03-Task 1 | 03 | 2 | ✅ `go build ./internal/bgp` | — | ✅ |
| 03-Task 2 | 03 | 2 | ✅ `go build ./internal/bgp` | — | ✅ |
| 03-Task 3 | 03 | 2 | ✅ `go test ./internal/bgp` | — | ✅ |
| 04-Task 1 | 04 | 3 | ✅ `go build ./internal/fswatcher` | — | ✅ |
| 04-Task 2 | 04 | 3 | ✅ `go build ./internal/fswatcher` | — | ✅ |
| 05-Task 1 | 05 | 4 | ✅ `go build ./cmd/bgp-dnsd` | — | ✅ |
| 06-Task 1 | 06 | 5 | ✅ `go test ./internal/bgp ./internal/dns` | — | ✅ |

### Check 8b — Feedback Latency
- All commands are `go build` or `go test` — well under 5 seconds ✅

### Check 8c — Sampling Continuity
- Wave 0: Tasks 1, 2, 3 all have automated verify → ✅ (3/3)
- Wave 1 (Plan 02): Tasks 1, 2, 3 all have automated → ✅
- Wave 2 (Plan 03): Tasks 1, 2, 3 all have automated → ✅
- Wave 3 (Plan 04): Tasks 1, 2 both have automated → ✅
- Wave 4 (Plan 05): Task 1 has automated → ✅
- Wave 5 (Plan 06): Task 1 has automated → ✅
- No 3 consecutive tasks without automated verify → ✅

### Check 8d — Wave 0 Completeness
VALIDATION.md lists 5 Wave 0 gaps:
1. ✅ `internal/config/main.go` — configKey struct → Plan 01 Task 1
2. ✅ `internal/loop/main.go` — logger parameter → Plan 01 Task 1
3. ✅ `internal/dns/resolvers.go` — logger parameter → Plan 01 Task 1
4. ✅ `internal/dns/loop.go` — remove context reads → Plan 01 Task 2
5. ✅ `internal/fswatcher/loop.go` — remove context reads → Plan 01 Task 2

---

## Dimension 9: Cross-Plan Data Contracts

| Data Entity | Plan A (Producer) | Plan B (Consumer) | Compatibility |
|-------------|-------------------|-------------------|---------------|
| `configKey` type | Plan 01 Task 1 (defines) | Plans 02-05 (uses) | ✅ Consistent |
| `loop.Loop` interface | Plan 01 Task 1 (adds logger param) | Plans 02-05 (passes) | ⚠️ See BLOCKER 2 |
| `*zerolog.Logger` | Plan 01 Task 1 (accepts) | Plans 02-05 (passes via log.L()) | ✅ Consistent |
| `*config.AppCfg` | Plan 01 Task 2 (passes to newCache) | Plan 02 (receives in NewDns) | ✅ Consistent |
| `bgpSrv` struct | Plan 03 (adds NewBgp) | Plan 05 (calls NewBgp) | ✅ Consistent |
| `fsWatcher` struct | Plan 04 (adds cfg field, NewFsWatcher) | Plan 05 (calls NewFsWatcher) | ⚠️ See BLOCKER 1 |

---

## Dimension 10: AGENTS.md Compliance

- ✅ GSD workflow followed (PLAN.md with frontmatter, tasks, verification)
- ✅ Spike findings referenced in canonical refs
- ✅ No forbidden patterns detected

---

## Dimension 11: Research Resolution

**❌ FAIL**

RESEARCH.md line 570: `## Open Questions` — missing `(RESOLVED)` suffix.

Three questions exist:
1. Loop embedding vs composition → Recommendation: keep embedding
2. Cache config via constructor → Recommendation: store in cache struct
3. log.L() global retention → Recommendation: keep for production default

**Fix:** Change heading to `## Open Questions (RESOLVED)` and prefix each answer with `RESOLVED:`.

---

## Dimension 12: Pattern Compliance

PATTERNS.md exists and is referenced by all plans. The `## File Classification` table maps all modified files to their closest analogs.

| Plan | File | PATTERNS.md Analog | Referenced | Status |
|------|------|--------------------|------------|--------|
| 01 | loop/main.go | log/main.go (NewLog) | ✅ in objective | ✅ |
| 01 | config/main.go | config/test.go | ✅ in objective | ✅ |
| 02 | dns/main.go | bgp/main.go (bgpSrv) | ✅ in objective | ✅ |
| 03 | bgp/main.go | fswatcher/main.go | ✅ in objective | ✅ |
| 04 | fswatcher/main.go | bgp/main.go | ✅ in objective | ✅ |
| 05 | cmd/main.go | N/A (no analog) | ✅ in objective | ✅ |
| 06 | bgp/main.go, resolvers.go | N/A (removal only) | ✅ in objective | ✅ |

All shared patterns (Typed Context Key, Explicit Logger Injection, Service Struct with Embedded Dependencies, Wrapper Function, Error Returns Instead of Panics, Test Helper Preservation) are referenced and applied.

---

## Scope Assessment

| Plan | Tasks | Files Modified | Est. Context | Status |
|------|-------|----------------|-------------|--------|
| 01 | 3 | 10 | ~55% | ✅ |
| 02 | 3 | 3 | ~30% | ✅ |
| 03 | 3 | 2 | ~30% | ✅ |
| 04 | 2 | 1 | ~25% | ✅ |
| 05 | 1 | 1 | ~25% | ✅ |
| 06 | 1 | 2 | ~15% | ✅ |

No plan exceeds 5 tasks or 15 files. Scope is healthy.

---

## Threat Models

All 6 plans include `<threat_model>` sections with STRIDE threat registers:
- Plan 01: 4 threats (config collision, logger bypass, nil config, module tampering)
- Plan 02: 3 threats (config validation, global access, module tampering)
- Plan 03: 3 threats (nil config, global access, module tampering)
- Plan 04: 2 threats (nil config, module tampering)
- Plan 05: 3 threats (config path injection, cascading failure, module tampering)
- Plan 06: 1 threat (module tampering — N/A)

Threat models are appropriate for the refactoring scope. No security-sensitive capabilities are misplaced.

---

## Structured Issues

```yaml
issues:
  - issue:
      plan: "03-01"
      dimension: "dependency_correctness"
      severity: "blocker"
      description: "Plan 01 Task 2 modifies internal/fswatcher/loop.go to use w.cfg, but the cfg field is not added to the fsWatcher struct until Plan 04 (Wave 3). After Plan 01 executes, go build ./internal/fswatcher will fail because w.cfg does not exist."
      fix_hint: "Add cfg field to fsWatcher struct in Plan 01 Task 2 and include internal/fswatcher/main.go in Plan 01 files_modified. The struct field addition is independent of other plans."

  - issue:
      plan: "03-02"
      dimension: "dependency_correctness"
      severity: "blocker"
      description: "Plan 02 Task 3 does not update newCache() to accept a loop.Loop parameter. NewDns receives a loop.Loop but never passes it to newCache. Meanwhile, newCache internally calls loop.NewLoop(1) which now requires a logger parameter (changed in Plan 01), causing compilation failure."
      fix_hint: "Update Plan 02 Task 3 to: (1) Change newCache signature to accept loop.Loop as first parameter, (2) Use injected loop instead of creating one internally, (3) Update NewDns to pass the received loop to newCache, (4) Update all test call sites."

  - issue:
      plan: "03-01"
      dimension: "task_completeness"
      severity: "warning"
      description: "Plan 01 files_modified lists 10 files but Task 3 references changes to internal/dns/resolvers_test.go and internal/dns/cache_test.go which are not in the files_modified list. The task description mentions updating newResolvers calls in resolvers_test.go (line 31) and newTestResolvers calls, but the file is not listed."
      fix_hint: "Add internal/dns/resolvers_test.go and internal/dns/cache_test.go to Plan 01 files_modified to match the actual files changed by Task 3."

  - issue:
      plan: "03-01"
      dimension: "task_completeness"
      severity: "warning"
      description: "Plan 01 Task 3 acceptance criteria does not include 'go build ./...' success check before test execution. Task 1 and Task 2 have go build acceptance criteria, but Task 3 jumps directly to go test. If compilation fails, tests cannot run."
      fix_hint: "Add 'go build ./...' success to Task 3 acceptance criteria before the go test step."

  - issue:
      plan: null
      dimension: "research_resolution"
      severity: "blocker"
      description: "RESEARCH.md has '## Open Questions' section (line 570) without (RESOLVED) suffix. Three questions exist with recommendations but are not formally marked as resolved per Dimension 11 requirements."
      fix_hint: "Change heading to '## Open Questions (RESOLVED)' and prefix each question's answer with 'RESOLVED:'."

  - issue:
      plan: "03-05"
      dimension: "dependency_correctness"
      severity: "info"
      description: "Plan 05 frontmatter declares wave: 4 but depends_on: [03-02, 03-03, 03-04] which are all wave: 1. Wave should be 2 (max(dep wave) + 1 = 1 + 1 = 2)."
      fix_hint: "Change wave from 4 to 2 in Plan 05 frontmatter."

  - issue:
      plan: "03-06"
      dimension: "dependency_correctness"
      severity: "info"
      description: "Plan 06 frontmatter declares wave: 5 but depends_on: [03-05]. If Plan 05 is wave 2, Plan 06 should be wave 3."
      fix_hint: "Change wave from 5 to 3 in Plan 06 frontmatter."

  - issue:
      plan: "03-01"
      dimension: "task_completeness"
      severity: "warning"
      description: "Plan 01 Task 2 action step 3c says 'The w.cfg field must be set by the constructor (done in Wave 3).' This acknowledges the fsWatcher struct won't have cfg until Plan 04, but the loop.go change in this task will fail compilation because the field doesn't exist yet."
      fix_hint: "Either add the cfg field to fsWatcher in Plan 01 Task 2, or defer the fswatcher/loop.go changes to Plan 04."
```

---

## Verification Verdict

### ISSUES FOUND

**3 BLOCKER(s)** require fixing before execution:

1. **Plan 01 breaks fswatcher compilation** — `w.cfg` used before field exists (BLOCKER)
2. **Plan 02 doesn't wire loop.Loop through newCache** — NewDns loop parameter unused, newCache creates loop without logger (BLOCKER)
3. **RESEARCH.md Open Questions not formally resolved** — heading missing (RESOLVED) suffix (BLOCKER)

**4 WARNING(s)** recommended for fix:

4. Plan 01 files_modified incomplete (missing test files)
5. Plan 01 Task 3 missing build verification before tests
6. Plan 01 Task 2 acknowledges cfg field timing issue but doesn't fix it
7. Plan 01 Task 3 acceptance criteria incomplete for resolvers_test.go

**2 INFO(s)** — cosmetic wave numbering corrections:

8. Plan 05 wave should be 2, not 4
9. Plan 06 wave should be 3, not 5

### Recommendation

**Return to planner with feedback.** The dependency correctness issues (BLOCKERS 1 and 2) represent genuine compilation failures that will occur during execution. The planner must:

1. **Add `cfg *config.AppCfg` field to `fsWatcher` struct in Plan 01** (or defer `fswatcher/loop.go` changes to Plan 04)
2. **Update `newCache` to accept `loop.Loop` parameter in Plan 02** and wire it through from `NewDns`
3. **Mark RESEARCH.md Open Questions as resolved**

Without these fixes, execution will fail at Plan 01 Task 2 (fswatcher compilation) and Plan 02 Task 3 (newCache loop creation).

---

*Verification performed by: Plan Checker Agent*
*Reference: gates.md (Revision Gate — bounded quality loop)*
*Dimension checklist: 1/12 ✅ | 2/12 ✅ | 3/12 ⚠️ | 4/12 ✅ | 5/12 ⚠️ | 6/12 ✅ | 7/12 ✅ | 7b/12 ✅ | 7c/12 ✅ | 8/12 ✅ | 9/12 ✅ | 10/12 ✅ | 11/12 ❌ | 12/12 ✅*
