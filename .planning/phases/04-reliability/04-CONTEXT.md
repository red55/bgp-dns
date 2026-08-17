# Phase 04: Reliability - Context

**Gathered:** 2026-08-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix four known reliability gaps (RELIAB-01 through RELIAB-04): DNS query timeout, O(n²) set-difference complexity, BGP operations using `context.Background()`, and silently swallowed BGP Advance/Withdraw errors. These are low-risk internal hardening changes — **no external behavior change** is expected; the Phase 2 test suite (all 37 tests, `-race` clean) is the regression safety net that must keep passing before and after.

</domain>

<decisions>
## Implementation Decisions

### Set Difference (RELIAB-02) — DISCUSSED & LOCKED this session

- **D-10:** Keep `Difference()` as a pure, general-purpose utility in `internal/utils/main.go`. Do not move it or inline the logic at its call site. CONCERNS.md's map-based suggestion applies to this function, not to `cache.go`.
- **D-11:** Rewrite internals from nested loops to a map-based lookup (O(n) total), but preserve the function's exact two-pass symmetric return shape: first all of `slice1` not present in `slice2`, then all of `slice2` not present in `slice1`, appended into the same result slice. No observable change to existing callers or tests given duplicate-free input.
- **D-12:** Collapse duplicate entries within each half of the output (membership check before append). This is a deliberate, documented improvement over today's loop-through-every-matching-occurrence behavior — the two call sites (`arrived`/`gone`) feed set-membership BGP operations that do not care about multiplicity. — **Reversibility:** costly — reverting D-12's dedup requires re-introducing occurrence-counting logic and updating any future test that asserts on degenerate duplicate inputs.
- **D-13:** No special-case guards needed for empty/nil input slices. First-ever cache entry (empty `prevIps`) and zero-IP resolution results are normal cases the algorithm already handles correctly; do not add redundant nil checks in `Difference()` itself.

### Carried Forward From Prior In-Session Conversation (NOT re-verified against current code this session)

The following were resolved in an earlier conversation recorded in `.continue-here.md` BEFORE this discussion ran. They are restated here so the planner has them in one file, but per the anti-pattern note below ("Re-confirm GA 1 & 2" was offered but NOT selected), **planner/researcher should treat these as candidate decisions, not verified facts** — confirm current code state (`resolvers.go` exchange calls, `bgp` package context usage) before building plans on top of them.

- **D-04 (RELIAB-01, DNS timeout):** Single global `Dns.Timeout` config value (proposed: flat field, not a per-resolver map), default 5s, accepted range 3–30s, one cached `*dns.Client` per resolver instance (not a fresh client per query). *Unresolved naming conflict: ROADMAP success criterion #1 reads `Dns.Timeouts.Query` — flat vs nested key name was offered as a discussable gray area but skipped; downstream agent implementing this must pick one and keep it consistent between the config struct tag and any docs/comments. Flagged as open.*
- **D-05 (RELIAB-03, BGP context propagation):** Store the daemon lifecycle context on `bgpSrv.ctx` (not recreate `context.Background()` per call); `NewBgp(ctx, cfg, loop, logger)` signature change threaded through from `cmd/bgp-dnsd/main.go`; shutdown path calls `s.cancel()` which propagates cancellation to in-flight GoBGP API calls. Test helper `NewBgpSrvForTest` also takes a context parameter.
- **D-06 (RELIAB-04, BGP error logging severity/context):** Not discussed this session or the prior one. ROADMAP success criterion #4 and REQUIREMENTS.md both point to Warn level with peer-IP context, but no explicit user decision was captured on exact log format, structured fields, or whether `Advance`/`Withdraw` failures versus lower-level GoBGP API errors get different treatment. Planner should decide within those stated constraints unless user raises a preference when seeing the plan.

### Scope Guardrail Note

Phase 4 does not change domain-list loading, BGP route selection semantics, gRPC interface surface, or anything in Phases 1–3 scope. If research surfaces a tempting adjacent fix (e.g., rate limiting, DNSSEC — mentioned in CONCERNS.md under security), defer it rather than folding it in.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Roadmap & Requirements
- `.planning/ROADMAP.md` §"Phase 4: Reliability" — goal, requirements list (RELIAB-01..04), the four numbered success criteria
- `.planning/REQUIREMENTS.md` §"Reliability" + traceability table — requirement definitions

### Prior-Phase Handoff (supersedes nothing above, fills gaps)
- `.planning/phases/04-reliability/.continue-here.md` — prior-session gray-area resolutions (GA 1/2) and explicit "not yet discussed" list (GA 3/4, now updated by this CONTEXT.md); includes Required Reading order and blocking-constraints recap inherited from Phase 3

### Codebase Intelligence
- `.planning/codebase/CONCERNS.md` §"O(n²) Set Difference" — the specific fix approach (map-based) this phase implements, plus related-but-out-of-scope security notes for awareness only

### Existing Code Files (read during research/planning, per .continue-here.md's required reading order)
- `internal/dns/resolvers.go` — where `dns.Exchange()` currently runs without timeout config; target of RELIAB-01 changes
- `internal/bgp/bgp.go` — `add`/`remove`/`find` methods currently on `context.Background()`; target of RELIAB-03
- `internal/bgp/main.go` — `bgpSrv` struct, `NewBgp` constructor signature, `cancel` field (RELIAB-03 wiring point); also home for RELIAB-04 log-site additions
- `cmd/bgp-dnsd/main.go` — daemon lifecycle context creation / cancellation point (RELIAB-03 thread-through)
- `internal/utils/main.go` — `Difference()` implementation being rewritten (RELIAB-02), plus its only real call sites:
- `internal/dns/cache.go:111-112` — the double-call pattern `Difference(prevIps, ips)` / `Difference(ips, prevIps)` feeding `bgp.Advance`/`bgp.Withdraw`

### Regression Baseline
- `.planning/phases/02-test-suite/02-VERIFICATION.md` — Phase 2's 5/5 verification result and the five test files named there (`internal/bgp/bgp_test.go`, `internal/dns/resolvers_test.go`, `internal/dns/cache_eviction_test.go`, `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go`, `internal/dns/dns_e2e_test.go`) — every one of these must pass unchanged-by-design through Phase 4
- `.planning/phases/03-dependency-injection/03-VERIFICATION.md` — Phase 3 baseline (build clean, full suite green post nil-guard fix) — the "before" state Phase 4 preserves

No external ADR/spec documents govern this phase; requirements fully captured in the decisions above + ROADMAP/REQUIREMENTS entries linked here.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `utils.Difference()` itself — stays a public, reusable utility per D-10/D-11; only its internals change.
- `loop.Loop` (from Phase 3 DI work, `internal/loop/main.go`) — already injects logger/config through constructors; no new pattern needed, just more instances consuming it consistently (consistent with Phase 3's established constructor pattern).

### Established Patterns
- Constructor + explicit dependency injection (Phase 3 outcome): `NewX(cfg, l, logger) (*X, error)` returning errors, never panicking. Any Phase 4 signature change (e.g. adding `ctx` param to `NewBgp`, threading a timeout into `newResolvers`/resolver construction) should follow the same "return `(svc, error)`, no panic" convention introduced in Phase 3, not reintroduce old global-state shortcuts.
- Backward-compat shim pattern (`Serve`/global-var wrappers kept alongside instance methods in Phase 3) — if D-05's `NewBgp` signature change breaks any Phase 2 test helper, extend that same shim pattern rather than removing the old entrypoint in-band with behavior-preserving work.

### Integration Points
- `cmd/bgp-dnsd/main.go:60,83,101` — the three service constructor call sites that will need the daemon context passed down once RELIAB-03 lands.
- `internal/dns/cache.go:111-112` — sole production caller of `utils.Difference()`; the behavioral contract that must hold exactly (modulo the documented dedup improvement in D-12) when the rewrite lands.
- `internal/config/config.go` — where the new `Timeout` field (exact name pending, see D-04 conflict note) must be added to whichever struct owns DNS settings, alongside existing validation/defaults logic.

</code_context>

<specifics>
## Specific Ideas

- The handoff file (`.continue-here.md`) carried two concrete pre-decided design sketches for GA 1 (DNS timeout) and GA 2 (BGP context) from a prior conversation — quoted verbatim there under `<decisions_made>` — so the researcher/planner can start from those instead of re-deriving them, subject to the "not re-verified" caveat recorded in D-04/D-05 above.
- No other product-specific references or examples were raised during this session's discussion of the set-difference area.

</specifics>

<deferred>
## Deferred Ideas

Carried-forward items explicitly offered for discussion this session but NOT addressed — leaving them unblocked for the planning stage rather than guessing at answers now:

1. **GA 3 — BGP error logging (RELIAB-04)** severity/structured-field specifics beyond what ROADMAP already fixes (Warn level, peer IP context). Open question left to planner within those stated bounds.
2. **Config key naming conflict** — `Dns.Timeout` (prior session) vs `Dns.Timeouts.Query` (ROADMAP SC-1 wording). Not settled by discussion; needs one consistent choice applied across config struct + docs when implementation happens.
3. **Verification status** of the prior-session GA 1/GA 2 decisions (D-04, D-05 above) against the actual current source tree, since that conversation happened before further commits landed in Phase 3's later waves.

None of these expand scope — they're refinements of decisions already inside Phase 4's four locked requirements, just not yet nailed down past the roadmapiel level.

</deferred>

---

*Phase: 04-Reliability*
*Context gathered: 2026-08-17*
