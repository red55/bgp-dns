# Phase 04: Reliability - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-17
**Phase:** 04-reliability
**Areas discussed:** O(n) set difference (RELIAB-02)

---

## Gray Area Selection

Presented four candidate gray areas for this phase (carried into this session from `.continue-here.md`'s "not yet discussed" list plus two newly identified during setup):

| Option | Description | Selected |
|--------|-------------|----------|
| BGP error logging severity/context (GA 3, RELIAB-04) | Warn vs Error, structured fields, peer IP context specifics | |
| O(n) set difference implementation location & shape (GA 4, RELIAB-02) | Where map-based rewrite lives + what contract it keeps | ✓ |
| Re-confirm GA 1 & 2 against current code | Verify prior-session DNS-timeout/BGP-context decisions still hold given commits that landed afterward | |
| Config key naming (Dns.Timeout vs Dns.Timeouts.Query) | ROADMAP SC-1 wording vs prior-session handoff decision disagree on the YAML key name | |

**User's choice:** O(n) set difference only. The other three were explicitly skipped and left as planner-stage items rather than forced into this discussion round; CONTEXT.md records exactly which of them were offered-and-skipped (D-04 conflict note, deferred ideas 1–3).

---

## O(n) Set Difference

### Q1: Where should the O(n) implementation live?

| Option | Description | Selected |
|--------|-------------|----------|
| Keep `Difference()` a pure utility in `internal/utils/main.go`, internals go map-based, same public API, call site unchanged | CONCERNS.md's suggested fix; minimal diff; zero behavior change to callers | ✓ |
| Also add an A/arrived+gone split helper so `cache.go` calls once instead of twice with swapped args | More churn; would need test updates in the same change | |
| Drop `utils.Difference` entirely, inline the map logic at `cache.go:111-120` | Loses the general-purpose named helper | |

**User's choice:** Keep pure utility (Recommended).
**Notes:** Rationale surfaced — CONCERNS.md already recommends this exact approach; keeping the function signature stable means no caller changes ride along with an algorithm swap.

### Q2: Preserve Difference()'s two-pass symmetric return shape exactly?

| Option | Description | Selected |
|--------|-------------|----------|
| Preserve current return shape exactly: first pass appends `slice1∖slice2-members`, second pass appends `slice2∖slice1-members`, same backing slice throughout | Zero observable change to any existing caller/test given duplicate-free input; future optimization can stop paying for the unused half later without another behavior-change risk | ✓ |
| Collapse to one direction only (`A ∖ B`) | Conceptually cleaner single-purpose contract, but changes observable output for corner cases today and forces a coordinated update to the `cache.go` double-call pattern at the same moment | |

**User's choice:** Preserve current return shape exactly (Recommended).
**Notes:** Acknowledged that today's two call sites each only consume one side of the result and throw away the other — accepted as wasted-but-safe work this phase is not allowed to remove (no-behavior-change constraint), flagged for potential follow-up if performance profiling ever cares.

### Q3: How should repeated identical entries in an input list be treated in the output?

| Option | Description | Selected |
|--------|-------------|----------|
| Let duplicates pass through as-is to match current O(n²) behavior exactly (occurrence-counting append per matching occurrence) | Maximally conservative byte-for-byte identical output even on degenerate inputs | |
| Collapse duplicates within each half of the output via membership check before appending | Slightly different output than today specifically when an input slice contains repeats of the same value; argued more correct since `arrived`/`gone` feed set-membership operations (BGP Advance/Withdraw) that don't care about multiplicity | ✓ |

**User's choice:** Collapse duplicates in output.
**Notes:** This is the only point where the new implementation intentionally differs from the old one's degenerate-input behavior; documented in CONTEXT.md D-12 with a reversibility rating (costly) because reverting it later would require re-introducing occurrence counting and revisiting any test asserting on duplicate-heavy edge cases.

### Q4: Any special handling wanted for empty/nil input slices?

| Option | Description | Selected |
|--------|-------------|----------|
| Treat as normal case — no special handling or top-level guard needed; algorithm already produces correct results for nil/empty | Less noise in the function body matches how the rest of the codebase handles trivially-safe inputs | ✓ |
| Add an explicit early-return guard at the very top even though the loop/map approach wouldn't crash on nil anyway | Defensive redundancy at the cost of a slightly noisier reading path | |

**User's choice:** Treat as normal case — no special handling.
**Notes:** Cited concrete real-world trigger (first-ever cache entry has empty `prevIps`; a domain resolving to zero IPs yields an empty `ips` list) to show these are ordinary, expected conditions already exercised indirectly by the existing test suite, not rare edge cases needing belt-and-suspenders guards.

### Follow-up question (post-initial-batch check-in)

| Option | Description | Selected |
|--------|-------------|----------|
| Next area (wrap up set-difference) | | |
| More questions | Ask additional edge-case questions before wrapping the area | ✓ |

**User's choice:** More questions → led to Q3 (duplicate collapse) and Q4 (empty/nil inputs) above, both recorded as their own numbered rows for completeness of the audit trail order.

Final area-closure check: "Set difference area is settled... Anything else on it?"
| Option | Description | Selected |
|--------|-------------|----------|
| Next area | | ✓ |
| More questions | | |

**User's choice:** Next area.

---

## Agent's Discretion

No full "you decide" deferrals occurred during this session's active questions — every option above had an explicit user selection. However, three gray areas offered during the initial multi-select were deliberately NOT selected by the user (BGP error logging details, config key name conflict, re-verification of prior-session decisions) and are handed to the planner stage as noted open items rather than quietly guessed at; see CONTEXT.md `<decisions>` ("Carried Forward") section and `<deferred>` section for the precise boundaries of what was resolved versus what remains planner-discretion within bounds already fixed by ROADMAP.

## Deferred Ideas

None new this session — the three carried-forward planner-stage items (GA 3 logging specifics, `Dns.Timeout` vs `Dns.Timeouts.Query` key naming, re-verification of prior GA 1/GA 2 code state) are refinements of decisions already inside Phase 4's locked roadmap requirements, not net-new scope for any phase.
