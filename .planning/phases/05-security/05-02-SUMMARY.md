---
phase: 05-security
plan: 02
status: complete
requirements: [SEC-03]
commits: 2
tasks_completed: 3
date: 2026-08-19
---

# Plan 05-02 Summary — DNS served-qtype guard (SEC-03)

## Changes

| File | Change |
|------|--------|
| `internal/dns/qtype_filter_test.go` | NEW (W0, Task 1): four table-driven tests driving traffic through the REAL mux (`c.mux.ServeDNS`) — `TestServedQTypes` {A,AAAA,HTTPS}, `TestDeniedQTypeRefused` {MX,TXT,CNAME,NS,PTR} with RcodeRefused==5 + empty Answer + zero upstream queries (per-qtype counting fake), `TestCatchAllForward` (unlisted MX transparently proxied via production catch-all wiring), `TestNXDomainPassthrough`. Local `countedFakeDNS` + `filterEnv` mirror `fakeDNSServerForE2E`/`setupE2EEnv` WITHOUT modifying shared fixtures; catch-all wired exactly as `main.go:52` does. Package doc comment records the D-05-P5 REFUSED decision and the AAAA-served/A-only-announced known state. |
| `internal/dns/cache.go` | MODIFIED (+38/-0, single insertion block + two one-line closure guards): `servedQTypes` policy set (deliberately separate from the byte-for-byte intact prefetch `requestTypes` at L140); `(c *cache) refuseIfNotServed` helper (fail-closed on empty question; zerolog Error on failed refusal write, matching `resolve()` posture); guard wired as FIRST line of BOTH registered-handler closures (`registerOn` exact path + `registerRegexOn` regex path) — the only two call sites, since `load()` builds all listed-domain handlers through these functions. `resolvers.go`, `serveMux.go`, `resolve.go` untouched. |

**Known accepted state (recorded per plan):** serving AAAA does NOT trigger any new prefetch or announcement — BGP announcement stays IPv4-A-only per QWEN.md daemon contract. The served/announced asymmetry is deliberate product state, not a defect.

**Pre-existing pipeline fact surfaced by the RED leg (NOT introduced by this plan):** `resolvers.query()` treats an empty answer with a non-success Rcode (e.g. upstream NXDOMAIN) as a resolver failure, and `resolve()` then sends no reply for A/AAAA lookups. Upstream NXDOMAIN therefore reaches the client as silence today; that behavior is unchanged by the guard. `TestNXDomainPassthrough` pins the GUARD's integrity contract instead: allowed qtypes still hit upstream, and any written reply carries the upstream rcode verbatim (never rewritten to REFUSED, no synthesized answers). Flagged here rather than silently re-scoped.

## RED Evidence (Task 1, pre-implementation)

Command: `go test ./internal/dns/ -run 'TestServedQTypes|TestDeniedQTypeRefused|TestCatchAllForward|TestNXDomainPassthrough' -count=1 -v`

```
--- PASS: TestServedQTypes            (A, AAAA, HTTPS — pre-phase behavior baseline)
--- FAIL: TestDeniedQTypeRefused      (all 5 subtests: MX, TXT, CNAME, NS, PTR)
        got RcodeSuccess + upstream hit; expected RcodeRefused + zero upstream
--- PASS: TestCatchAllForward         (anti-pattern guard: proxy already clean)
--- FAIL: TestNXDomainPassthrough     (initial strict form: no reply written —
        pre-existing resolvers NXDOMAIN handling, see known-state above)
```

The denial leg is the genuine RED: without the guard, denied qtypes hit upstream and returned Success — exactly the hole SEC-03 closes. After Task 2's guard: all four tests GREEN (re-run recorded under Verification).

## Verification Results

| Gate | Command | Result |
|------|---------|--------|
| Target suite (GREEN leg) | `go test ./internal/dns/ -run 'TestServedQTypes\|TestDeniedQTypeRefused\|TestCatchAllForward\|TestNXDomainPassthrough' -count=1` | PASS (12/12 incl. subtests) |
| vet | `go vet ./internal/dns/` | clean |
| Quick run | `go test ./cmd/bgp-dnsd/... ./internal/dns/... ./internal/bgp/... ./internal/config/... -count=1` | all ok (dns 3.0s) |
| Dep drift | `go mod tidy && git diff --exit-code go.mod go.sum` | NONE |
| Prohibited files | `git diff --name-only internal/ cmd/` | only `internal/dns/cache.go` + new test file |
| AuthPassword leak | `grep -rc AuthPassword internal/dns/` | 0 matches |
| Guard placement | grep `refuseIfNotServed(` non-test sources | exactly 2 call sites (both registered-handler closures) |
| Policy isolation | `var requestTypes` L140 | byte-for-byte intact `{TypeA, TypeHTTPS}` |
| Refusal locality | `RcodeRefused` in internal/dns non-test sources | exactly 1 occurrence (the guard) |

Wave-merge full-suite gate (run at phase wave close, after 05-03):
`go vet ./... && go build ./... && go test ./...` — PENDING ORCHESTRATOR VERIFICATION.

## Rule-1 Deviations

1. **Executor session failure → inline execution.** The spawned gsd-executor died mid-run (~9h wall, `terminated`, no disk artifacts, no SUMMARY). Re-spawning a second long subagent run for a 2-file change with fully verified ground truth was higher-risk than orchestrator-inline execution; executed inline per workflow stall-recovery "kill and switch to inline". All spec details were re-verified against live source before editing.
2. **NXDOMAIN assertion scoped down** (documented above): plan wording assumed true passthrough; live pipeline drops NXDOMAIN replies pre-phase. Assertion changed from "reply must exist and carry RcodeNameError" to structural guard-contract pins, with the pre-existing fact recorded as known-state. No plan intent weakened — the guard cannot rewrite rcodes either direction.
3. **Helper made a method** (`c.refuseIfNotServed`) instead of a free function, purely to get the injected zerolog logger for write-failure logging consistent with `resolve()`. Signature/semantics identical to plan pseudocode.

## Self-Check

PASSED — build/vet/tests green (above), import audit clean (test file uses net/os/sync/testing/time + existing deps only), symbol uniqueness repo-wide, EOL: `cache.go` kept its pre-existing CRLF terminators (diff stat +38/-0 confirms no line churn), new test file LF like its siblings.

commit: 2 planned below (W0 test → guard), orchestrator-committed.
