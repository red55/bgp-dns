# Phase 1 Plan Review — 01-regex-domainlist

**Review Date:** 2026-06-13
**Plans Reviewed:** 01-01-PLAN.md, 01-02-PLAN.md, 01-03-PLAN.md
**Reviewer:** Plan Checker (Revision Gate)

---

## Overall Verdict: FAIL (with path to fix)

**Issues found:** 3 BLOCKERs, 4 WARNINGs, 2 INFO

The plans are structurally sound and cover all 5 requirements with appropriate task decomposition. However, 3 blockers must be fixed before execution:
1. Wave-complete tasks have verify commands that don't validate their done criteria
2. Integration tests are missing behavioral assertions (only checking generation counts)
3. `load()` clears all mux handlers before re-registering — a silent risk during file watcher reload

---

## Dimension 1: Requirement Coverage

| Requirement | Plan | Task(s) | Status |
|-------------|------|---------|--------|
| REGEX-01 (regex: prefix syntax) | 01-02 | Task 1 (load regex: detection) | Covered |
| REGEX-01 (regex: prefix syntax) | 01-03 | Task 1 (LoadRegexPattern integration test) | Covered |
| REGEX-02 (priority: exact > wildcard > regex > catch-all) | 01-01 | Task 1 (ServeDNS routing) | Covered |
| REGEX-02 | 01-01 | Task 2 (PriorityExactOverWildcard, PriorityWildcardOverRegex, PriorityRegexOverCatchAll) | Covered |
| REGEX-02 | 01-03 | Task 1 (PriorityExactOverRegex integration test) | Covered |
| REGEX-03 (compile at load time) | 01-01 | Task 1 (HandleRegex uses regexp.Compile) | Covered |
| REGEX-03 | 01-03 | Task 1 (LoadRegexPattern verifies compilation) | Covered |
| REGEX-04 (invalid regex → warning, skip) | 01-02 | Task 1 (load skips invalid regex with Warn) | Covered |
| REGEX-04 | 01-01 | Task 2 (HandleRegexInvalid test) | Covered |
| REGEX-04 | 01-03 | Task 1 (InvalidRegexSkippedWithWarning test) | Covered |
| REGEX-05 (exact-match still works) | 01-02 | Task 1 (register delegates to mux.HandleFunc) | Covered |
| REGEX-05 | 01-01 | Task 2 (ExactMatch test) | Covered |
| REGEX-05 | 01-03 | Task 1 (ExactMatchStillWorks test) | Covered |

**Result:** All 5 requirements covered by at least 2 tasks across plans. ✅ PASS

---

## Dimension 2: Context Compliance

### Locked Decisions (from CONTEXT.md / DISCUSSION-LOG.md)

| Decision | Implemented In | Status |
|----------|---------------|--------|
| D-01: `regex:` prefix syntax | Plan 01-02 Task 1 (`strings.HasPrefix(line, "regex:")`) | ✅ |
| D-02: Wildcard support in Phase 1 | Plan 01-01 Task 1 (HandleFunc wildcard branch) | ✅ |
| D-03: Load-time validation only | Plan 01-01 Task 1 (HandleRegex uses regexp.Compile) | ✅ |
| D-04: Skip invalid regex + Warn | Plan 01-02 Task 1 (`continue` on registerRegex error) | ✅ |
| D-05: Work within global state | Plan 01-02 Task 2 (uses _cache.SetMux, no DI) | ✅ |

### Deferred Ideas (excluded from plans)
| Deferred Idea | Present in Plans? | Status |
|---------------|-------------------|--------|
| `/pattern/` syntax | No | ✅ Correctly excluded |
| Runtime limits (compile timeout, pattern length) | No | ✅ Correctly excluded |
| DI refactoring | No | ✅ Correctly excluded |
| Fail-to-start on invalid regex | No | ✅ Correctly excluded |

### Scope Reduction Detection
No scope reduction language found. All plans implement full decisions as written. ✅ PASS

---

## Dimension 3: Task Completeness

### Plan 01-01 (Foundation)

| Task | Files | Action | Verify | Done | Specificity | Status |
|------|-------|--------|--------|------|-------------|--------|
| 01-01.1 (serveMux.go) | ✅ | ✅ (detailed method-by-method) | ⚠️ `go build` only | ✅ | High | WARNING |
| 01-01.2 (serveMux_test.go) | ✅ | ✅ (12 test specs) | ⚠️ `go build` only | ✅ | High | WARNING |
| 01-01.3 (Wave 1 complete) | ✅ | ✅ (4 verification commands) | ⚠️ `go build` only | ✅ | Medium | WARNING |

**Issue:** All three tasks have `<verify>` blocks that only contain `go build ./internal/dns/`. This command cannot distinguish pass from fail of the actual acceptance criteria:
- Task 1: `go build` succeeds but doesn't verify 6 methods exist
- Task 2: `go build` succeeds but doesn't verify 12 tests pass
- Task 3: `go build` succeeds but doesn't verify 12/12 tests PASS

The `<done>` criteria for Task 2 requires "All 12 tests pass" but `<verify>` only runs `go build`. These are mismatched.

### Plan 01-02 (Integration)

| Task | Files | Action | Verify | Done | Specificity | Status |
|------|-------|--------|--------|------|-------------|--------|
| 01-02.1 (cache.go) | ✅ | ✅ (line-by-line modifications) | ⚠️ `go build` only | ✅ | Very High | WARNING |
| 01-02.2 (main.go) | ✅ | ✅ (line-by-line modifications) | ⚠️ `go build` only | ✅ | Very High | WARNING |
| 01-02.3 (cache_test.go) | ✅ | ✅ (SetMux injection) | ⚠️ `go build` only | ✅ | Medium | WARNING |
| 01-02.4 (Wave 2 complete) | ✅ | ✅ (5 verification commands) | ⚠️ `go build` only | ✅ | Medium | WARNING |

### Plan 01-03 (Integration Tests)

| Task | Files | Action | Verify | Done | Specificity | Status |
|------|-------|--------|--------|------|-------------|--------|
| 01-03.1 (integration tests) | ✅ | ✅ (7 test specs) | ⚠️ `go build` only | ✅ | Medium | WARNING |
| 01-03.2 (full suite verify) | ✅ | ✅ (3 verification commands) | ⚠️ `go build` only | ✅ | Medium | WARNING |

**Blocker:** Plan 01-03 Task 2 `<done>` states "All 25 tests pass" and "go test -race passes with no data races", but `<verify>` only contains `go build ./internal/dns/`. This is a critical gap — the verify command cannot confirm the done criteria.

---

## Dimension 4: Dependency Correctness

```
Plan 01-01 (Wave 1): depends_on: [] → Wave 1
Plan 01-02 (Wave 2): depends_on: [01-01] → Wave 2
Plan 01-03 (Wave 3): depends_on: [01-02] → Wave 3
```

Within Plan 01-01:
- Task 1 → Task 2 (serveMux.go must exist before tests) ✅
- Task 2 → Task 3 (wave complete depends on tests) ✅

Within Plan 01-02:
- Task 1 → Task 2 (cache.go must exist before main.go wiring) ✅
- Task 1 → Task 3 (cache_test.go depends on cache changes) ✅
- Task 3 → Task 4 (wave complete depends on all tasks) ✅

No cycles, no missing references, no forward references. ✅ PASS

---

## Dimension 5: Key Links Planned

| Key Link | Source Task | Wiring Verified | Status |
|----------|------------|-----------------|--------|
| cache.register() → c.mux.HandleFunc() | 01-02 Task 1 | ✅ Action specifies exact replacement | ✅ |
| cache.load() → c.mux.clear() + c.registerRegex() | 01-02 Task 1 | ✅ Action specifies clear() before scan | ✅ |
| main.go Serve() → mux creation + injection | 01-02 Task 2 | ✅ Action specifies SetMux + SetCatchAll | ✅ |
| main.go Shutdown() → mux.clear() | 01-02 Task 2 | ✅ Replaces dns.HandleRemove | ✅ |
| mux.ServeDNS() → c.resolve() | 01-03 Task 1 | ✅ Integration test exercises full path | ✅ |

All artifacts are wired. ✅ PASS

---

## Dimension 6: Scope Sanity

| Plan | Tasks | Files | Wave | Status |
|------|-------|-------|------|--------|
| 01-01 | 3 | 2 (serveMux.go, serveMux_test.go) | 1 | ✅ (3 tasks, 2 files) |
| 01-02 | 4 | 3 (cache.go, main.go, cache_test.go) | 2 | ✅ (4 tasks, 3 files) |
| 01-03 | 2 | 1 (regex_integration_test.go) | 3 | ✅ (2 tasks, 1 file) |

No plan exceeds 4 tasks. No plan exceeds 5 files. Total phase scope: 9 tasks, 6 files. ✅ PASS

---

## Dimension 7: Verification Derivation

All `must_haves.truths` are user-observable (not implementation details):
- "User can add regex:... lines and queries resolve correctly" ✅
- "Exact domain entries still resolve with same behavior" ✅
- "Wildcard entries match subdomains" ✅
- "DNS query priority is exact > wildcard > regex > catch-all" ✅
- "Invalid regex patterns produce a warning and do not block loading" ✅

Artifacts map to truths:
- serveMux.go → all 5 truths (routing engine)
- serveMux_test.go → all 5 truths (priority, regex, invalid)
- cache.go + main.go → all 5 truths (integration)
- regex_integration_test.go → all 5 truths (end-to-end)

Key links connect artifacts to functionality. ✅ PASS

---

## Dimension 8: Nyquist Compliance

### Check 8e — VALIDATION.md Existence
No VALIDATION.md found for Phase 1. This is expected for a plan-checker review (VALIDATION.md is created during execution). ✅ SKIPPED

### Check 8a — Automated Verify Presence
All tasks have `<automated>` verify commands. However, all use `go build ./internal/dns/` which is insufficient for test-related verification.

### Check 8c — Sampling Continuity
- Wave 1: Tasks 1, 2, 3 all have `<automated>` verify commands (build only)
- Wave 2: Tasks 1, 2, 3, 4 all have `<automated>` verify commands (build only)
- Wave 3: Tasks 1, 2 all have `<automated>` verify commands (build only)

**Blocker:** The automated verify commands are insufficient. `go build` cannot verify test pass/fail. The verify commands should include `go test` invocations.

### Check 8d — Wave 0 Completeness
RESEARCH.md identifies 5 Wave 0 gaps (serveMux_test.go tests). Plan 01-01 Task 2 creates serveMux_test.go with 12 tests — this addresses the Wave 0 gaps. ✅ PASS

---

## Dimension 9: Architectural Tier Compliance

From RESEARCH.md Architectural Responsibility Map:

| Capability | Expected Tier | Plan Assignment | Status |
|------------|--------------|-----------------|--------|
| Domainlist parsing | DNS Cache (cache.load) | Plan 01-02 Task 1 (cache.go) | ✅ |
| Regex compilation | DNS Cache (registerRegex) | Plan 01-01 Task 1 (HandleRegex) | ✅ |
| DNS query routing | Custom mux (ServeDNS) | Plan 01-01 Task 1 (serveMux.go) | ✅ |
| DNS interception | DNS Cache (cache.resolve) | Plan 01-02 Task 1 (unchanged) | ✅ |
| Upstream proxy | Resolver ring (proxyQuery) | Plan 01-02 Task 2 (SetCatchAll) | ✅ |
| BGP advertisement | BGP Server | Unchanged | ✅ |

All tiers match. ✅ PASS

---

## Dimension 10: Cross-Plan Data Contracts

| Entity | Plan A | Plan B | Compatibility |
|--------|--------|--------|---------------|
| `*regexServeMux` | Plan 01-01: defines struct | Plan 01-02: uses in cache struct | ✅ Compatible |
| `HandleFunc(pattern, handler)` | Plan 01-01: registers exact/wildcard | Plan 01-02: calls via c.mux.HandleFunc() | ✅ Compatible |
| `HandleRegex(pattern, handler)` | Plan 01-01: compiles + registers | Plan 01-02: calls via c.mux.HandleRegex() | ✅ Compatible |
| `HandleRemove(pattern)` | Plan 01-01: removes exact/wildcard | Plan 01-02: calls via c.mux.HandleRemove() | ✅ Compatible |
| `SetCatchAll(handler)` | Plan 01-01: sets catchAll field | Plan 01-02: calls in main.go Serve() | ✅ Compatible |
| `clear()` | Plan 01-01: resets all handlers | Plan 01-02: calls in load() and Shutdown() | ✅ Compatible |
| `SetMux(*regexServeMux)` | Plan 01-01: not defined | Plan 01-02: defines on cache | ✅ Compatible |

No conflicting transformations. ✅ PASS

---

## Dimension 11: AGENTS.md Compliance

AGENTS.md specifies:
- GSD workflow with planning docs in `.planning/` ✅ (plans are in correct location)
- Spike findings skill should be used ✅ (Plan 01-01 reads regex-domainlist.md)
- Phase 1 is "Regex Domainlist" ✅ (matches roadmap)

No violations. ✅ PASS

---

## Dimension 12: Spike Blueprint Compliance

The spike blueprint (regex-domainlist.md) provides:
1. ✅ regexServeMux struct definition — Plan 01-01 Task 1 matches exactly
2. ✅ HandleRegex method — Plan 01-01 Task 1 implements it
3. ✅ ServeDNS priority routing — Plan 01-01 Task 1 implements it
4. ✅ Load-time regex detection — Plan 01-02 Task 1 implements it
5. ✅ registerRegex method — Plan 01-02 Task 1 implements it
6. ✅ main.go mux wiring — Plan 01-02 Task 2 implements it

Plans faithfully implement the spike blueprint. ✅ PASS

---

## Identified Issues

### BLOCKER 1: Verification commands don't validate done criteria

**Plans:** 01-01 (all 3 tasks), 01-02 (all 4 tasks), 01-03 (both tasks)

**Description:** Every task's `<verify>` block contains only `go build ./internal/dns/`. This command cannot verify the `<done>` acceptance criteria:
- Task 01-01.2 done criteria: "All 12 tests pass" — but verify only runs `go build`
- Task 01-01.3 done criteria: "12/12 PASS" — but verify only runs `go build`
- Task 01-02.4 done criteria: "All 6 existing cache tests pass" — but verify only runs `go build`
- Task 01-03.2 done criteria: "All 25 tests pass, race detector clean" — but verify only runs `go build`

The executor will run the verify command, see a successful build, and mark the task as verified — even if tests fail to compile or pass.

**Fix:** Replace `go build` in `<verify>` blocks with appropriate test commands:
- For test-creation tasks: `<automated>go test ./internal/dns/ -run TestServeMux -v -count=1</automated>`
- For wave-complete tasks: `<automated>go test ./internal/dns/ -v -count=1 && go test ./internal/dns/ -race -count=1</automated>`

---

### BLOCKER 2: Integration tests lack behavioral assertions

**Plans:** 01-03 Task 1 (Tests 1, 3, 5, 6, 7)

**Description:** Several integration tests only verify generation counter increments but don't assert actual handler behavior:

| Test | What it verifies | Missing |
|------|-----------------|---------|
| LoadRegexPattern | Generation > 0 | Regex pattern actually registered in mux |
| InvalidRegexSkippedWithWarning | Generation > 0 | exact domains still registered after invalid regex |
| WildcardInDomainlist | Generation > 0 | Wildcard handler actually matches queries |
| LoadClearsPreviousState | Generation > 0 | Old handlers removed, new handlers present |
| ExactMatchStillWorks | Generation > 0 | Exact domains actually match queries |

These tests would pass even if the implementation was completely broken (as long as generation increments).

**Fix:** Add behavioral assertions to each integration test:
- Test 1: Query the mux for a regex-matched domain after load
- Test 3: Query the mux for example.com and test.com to verify they registered
- Test 5: Query the mux for a wildcard-matched domain
- Test 6: Query the mux for both old and new domains
- Test 7: Query the mux for cloudflare.com and google.com

---

### BLOCKER 3: `load()` clears handlers before re-registering — silent risk during file watcher reload

**Plans:** 01-02 Task 1 (load modification)

**Description:** The plan modifies `load()` to call `c.mux.clear()` before scanning the new file. This means:
1. All handlers are removed
2. File is scanned line by line
3. Handlers are re-registered as each line is processed

If the file is large or the scan fails partway through, there is a window where:
- All registered domain handlers are gone
- DNS queries fall through to catch-all (proxyQuery)
- BGP announcements are not made for any registered domains

The existing file watcher (fswatcher/loop.go) calls `dns.Load()` on file change, which calls `cache.load()`. If `load()` fails mid-way (e.g., I/O error), the handlers are cleared but not restored.

RESEARCH.md Open Question #2 explicitly flags this: "whether load() needs to clear the old mux state before registering new patterns." The plan answers "yes, always clear" but doesn't address the failure scenario.

**Severity:** BLOCKER because this is a reliability issue that affects the daemon during runtime file watcher reloads.

**Fix options:**
1. **Safe reload (recommended):** Build new handler list first, then atomically swap. Use `clear()` only after all new handlers are registered successfully.
2. **Dual-mux approach:** Keep old mux active while building new one, swap on success.
3. **Document the risk:** If safe reload is too complex for Phase 1, document the failure mode and add a TODO for Phase 3 (DI refactoring).

---

### WARNING 1: `HandleFunc` wildcard prefix extraction strips wrong character count

**Plan:** 01-01 Task 1 (HandleFunc implementation)

**Description:** The HandleFunc implementation checks `strings.HasPrefix(pattern, "*.")` (2-char prefix) and strips it with `pattern[2:]`. However, HandleRemove checks `strings.HasPrefix(pattern, "*.")` and strips only 1 character with `pattern[1:]`.

This means:
- Registering `"*.example.com"` stores prefix `"example.com"` (stripped `"*."`)
- Removing `"example.com."` (with trailing dot) strips `pattern[1:]` = `"xample.com."` — doesn't match `"example.com"`

The plan's HandleRemove implementation strips only 1 character (`pattern[1:]`), which is inconsistent with HandleFunc's 2-character strip (`pattern[2:]`).

**Fix:** Ensure HandleRemove strips the same prefix length as HandleFunc:
```go
// HandleRemove should use:
prefix := pattern[2:]  // not pattern[1:]
```

---

### WARNING 2: Plan 01-02 Task 3 (cache_test.go update) has redundant SetMux call

**Plan:** 01-02 Task 3

**Description:** The plan updates `newTestCache()` to call `c.SetMux(newRegexServeMux())`. However, `newCache()` already creates the mux internally (`r.mux = newRegexServeMux()` per Task 1). The SetMux call in Task 3 replaces the already-initialized mux with a new one.

This is not a correctness issue (the test passes either way), but it's redundant code that adds confusion.

**Fix:** Either remove the SetMux call from newTestCache() (since newCache already initializes the mux), or remove the mux initialization from newCache() and require SetMux. The plan does both, which is inconsistent.

---

### WARNING 3: serveMux_test.go wildcard test uses trailing dot in HandleFunc pattern

**Plan:** 01-01 Task 2 (TestServeMux_WildcardMatch)

**Description:** The test calls `mux.HandleFunc("*.example.com.", ...)` with a trailing dot. In HandleFunc, the wildcard check `strings.HasPrefix("*.example.com.", "*.")` succeeds, and prefix is stored as `"example.com."` (with trailing dot). In ServeDNS, the wildcard match checks `strings.HasSuffix("foo.example.com.", "example.com.")` which passes.

This works, but it's inconsistent with how domainlist entries would be registered (no trailing dot). The test should use `"*.example.com"` (no trailing dot) to match real usage.

**Fix:** Change test patterns to not include trailing dots, matching real domainlist file entries.

---

### INFO 1: Test count discrepancy in Plan 01-03 Task 2

**Plan:** 01-03 Task 2

**Description:** The plan states "25 tests total (6 cache + 12 mux + 7 integration)" but Plan 01-02 Task 3 adds a new test (for SetMux), making cache tests = 7, total = 26.

**Fix:** Update the expected test count to 26.

---

### INFO 2: Test helper duplication between serveMux_test.go and regex_integration_test.go

**Plan:** 01-01 Task 2 and 01-03 Task 1

**Description:** Both test files define `testResponseWriter`. Since they're in the same package (`package dns`), the test helper should be defined once (in serveMux_test.go) and shared.

**Fix:** Remove the testResponseWriter definition from regex_integration_test.go and reference the one in serveMux_test.go.

---

## Summary

| Dimension | Status | Notes |
|-----------|--------|-------|
| Requirement Coverage | ✅ PASS | All 5 requirements covered |
| Context Compliance | ✅ PASS | All decisions honored, no scope creep |
| Task Completeness | ⚠️ WARN | Verify/done mismatch on all tasks |
| Dependency Correctness | ✅ PASS | Valid DAG, no cycles |
| Key Links Planned | ✅ PASS | All artifacts wired |
| Scope Sanity | ✅ PASS | Within budget |
| Verification Derivation | ✅ PASS | User-observable truths |
| Nyquist Compliance | ❌ FAIL | Verify commands insufficient |
| Architectural Tier | ✅ PASS | Correct tier assignments |
| Data Contracts | ✅ PASS | Compatible across plans |
| AGENTS.md | ✅ PASS | No violations |
| Spike Blueprint | ✅ PASS | Faithful implementation |

**Blocking Issues: 3**
1. Verify commands can't validate test pass/fail
2. Integration tests lack behavioral assertions
3. load() handler-clearing risk during file watcher reload

**Recommendation:** Return to planner with the 3 blockers. The plans are well-structured and comprehensive, but the verification gaps and the load() safety issue must be addressed before execution.
