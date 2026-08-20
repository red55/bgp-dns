# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-08-20
**Phases:** 5 | **Plans:** 17 | **Requirements:** 22/22

### What Was Built
- Regex domainlist support (exact > wildcard > regex > catch-all, load-time compile, atomic-swap reload)
- Full test coverage of critical paths: BGP ref-counting, resolver-ring failover, cache eviction, gRPC lifecycle, DNS→cache→BGP E2E — 82 test functions, green under -race
- Dependency injection across every subsystem; panic-on-init replaced with returned errors
- Reliability fixes: configurable DNS query timeout, O(n) set difference, cancellable daemon context for BGP, structured op-failure logging
- Security hardening: unix socket 0600, unified gRPC validation gate + audit log, listed-domain qtype filter (A/AAAA/HTTPS else REFUSED), opt-in per-peer BGP TCP-MD5 never logged

### What Worked
- Spike-first for the riskiest unknown (regexServeMux validated in spike 001 before the roadmap existed) — Phase 1 executed with zero design surprises
- Test-net-before-refactor ordering (Phase 2 safety net → Phase 3 DI) made the biggest structural change low-risk; behavioral equivalence held
- Machine-proven prohibitions in verification reports — negative claims ("no new goroutines", "credential never logged", "prefetch list untouched") verified against `git diff <phase-range>` and grep, not attested
- Wave-0 test scaffolding baked into plans meant verification harnesses already existed when truths were checked
- Parallel execution of Phases 4 and 5 after their shared Phase 3 dependency shortened wall-clock time

### What Was Inefficient
- Phase 5's execute-phase never ran its verify step → UAT started against a phase with no VERIFICATION.md and stalled on the completion predicate; regenerated mid-session
- The api-coverage verify:pre gate over-fired on first-party terminology ("surface… api"); resolving it took a reasoned `COVERAGE.md` no-integration declaration
- Plan-checker reports (`*-PLAN-CHECK.md`) matched the plan scanner's loose filename fallback and inflated plan counts in Phases 2 AND 3 — silently breaking phase-completion projection until manually diagnosed at milestone close
- Bookkeeping drift accumulated between sessions: two stale requirement checkboxes (TEST-03/04), mixed CRLF/LF report encodings, a stale STATE focus line — all found only during closeout

### Patterns Established
- Verification reports enumerate must-haves (truths / artifacts / key links / prohibitions) and tie every truth to a named passing test; negative claims carry command evidence
- Prohibition proof uses the phase's exact commit range as the diff base
- Shell-less executors self-check statically (re-grep) and leave build/vet/test gates to the orchestrator, who runs them once per wave
- Closeout hygiene checklist: plan-count sanity, checkbox parity vs verification reports, line-ending consistency, STATE/ROADMAP freshness

### Key Lessons
1. Non-executable artifacts that share a naming space with plans will break count-based completion detection — give such docs a `status: superseded` frontmatter marker (or distinct names) from day one; this recurred twice (phases 2, 3)
2. Run the verify step *within* the execute-phase session, not after — a missing VERIFICATION.md blocks the entire UAT→transition→closeout chain
3. Prove negative assertions with the phase commit range diff; it is cheap, conclusive, and keeps verification reproducible
4. Bookkeeping (checklist boxes, frontmatter, encoding) must be a close-out check item — it degrades silently between sessions

### Cost Observations
- Model mix: opus (planning/checking), sonnet (execution), haiku (trivial steps) — balanced profile throughout
- Timeline: 2026-06-13 → 2026-08-20 (~9 calendar weeks, work concentrated in June and mid/late August around the Phase 3–5 push)
- Notable: zero new third-party dependencies across all five phases; ~6.9K Go insertions net

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | multiple (June + Aug 2026) | 5 | Brownfield bootstrap: spike-first, DI-after-test-net, machine-proven prohibitions |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|--------------------|
| v1.0 | 82 test functions, full suite -race green | not instrumented | 0 |

### Top Lessons (Verified Across Milestones)

1. (pending second milestone — see v1.0 lesson 1 on artifact-naming collisions)
