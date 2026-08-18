# Phase 04: Reliability - Context

**Gathered:** 2026-08-17
**Updated:** 2026-08-18 (D-04/D-05 re-verified against code and LOCKED; config naming conflict resolved)
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

### Verified Against Current Code (2026-08-18, post Phase 3)

The two prior-session decisions were re-confirmed against the live source tree. They are now **LOCKED** (previously "candidate, not verified").

- **D-04 (RELIAB-01, DNS timeout) — LOCKED & VERIFIED:**
  - Single global `Dns.Timeout` — **flat field on `dnsCfg`** (`Timeout time.Duration \`yaml:"Timeout"\``). **Naming conflict resolved in favor of flat**: pre-existing config style is flat sub-fields (`Cache.MinTtl`, `Cache.MaxEntries`) and the flat name was the earlier explicit decision; ROADMAP SC-1 wording `Dns.Timeouts.Query` is superseded. Apply consistently across config struct tag, docs/comments, and tests.
  - Default 5s, validated range 3–30s (enforce in config init alongside existing validation).
  - Per-resolver cached `*dns.Client`: add `client *dns.Client` to the `resolver` struct, built in `newResolver` from the configured duration (`Timeout` + `ReadTimeout`). `dns.Exchange` calls become `r.client.Exchange(q, r.addr.String())` — same behavior, bounded latency.
  - Threading: `newResolvers(addrs, timeout, logger)` gains a `timeout` param; sole production caller is `internal/dns/main.go:48`; in-package test helpers pass `0` (→ default) or an explicit short value where timing is under test.
  - Verification evidence: query path is bare `dns.Exchange(q, srv.addr.String())` with no client; `dnsCfg` has no timeout field today.

- **D-05 (RELIAB-03, BGP context propagation) — LOCKED & VERIFIED:**
  - `NewBgp(ctx, cfg, l, logger)` — takes the daemon lifecycle context (Go convention: ctx first). Internally derives a child: `cctx, cancel := context.WithCancel(ctx)`; stores `s.ctx = cctx`, `s.cancel = cancel`. Keeps `Shutdown()` owning cancellation while the daemon's parent cancel propagates on signal.
  - `bgpSrv` gains a `ctx context.Context` field beside the existing `cancel context.CancelFunc` (no other struct change).
  - All three `context.Background()` call sites in `bgp.go` (`add`, `find`, `remove`) switch to `s.ctx`; the `//TODO: pass context` marker disappears.
  - `cmd/bgp-dnsd/main.go:83` passes the daemon ctx (already in scope, currently unused at that call site).
  - Legacy shim `bgp.Serve(ctx)` already accepts ctx and discards it — after the change it forwards ctx to `NewBgp` (no test churn).
  - `NewBgpSrvForTest(t)` needs **no signature change**: it builds `bgpSrv` directly, so it creates its own `WithCancel(Background())` and stores it in `s.ctx` (the returned `cancel` already covers cleanup).
  - Verification evidence: `bgp.go` carries `//TODO: pass context`; `find`/`remove` use `context.Background()`; `NewBgp` today creates its own `WithCancel(Background())`, so the daemon lifecycle is not propagated.
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

Resolved since last revision (no longer open):
- ~~Config key naming conflict (`Dns.Timeout` vs `Dns.Timeouts.Query`)~~ — **RESOLVED 2026-08-18**: flat `Dns.Timeout`, see D-04 above.
- ~~Verification of prior-session GA 1/GA 2 decisions against current code~~ — **DONE 2026-08-18**: both re-verified and LOCKED, see "Verified Against Current Code" section above.

None of these expand scope — they're refinements of decisions already inside Phase 4's four locked requirements, just not yet nailed down past the roadmap-level.

</deferred>

---

*Phase: 04-Reliability*
*Context gathered: 2026-08-17*
