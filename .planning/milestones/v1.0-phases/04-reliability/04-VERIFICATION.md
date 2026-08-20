---
phase: 04-reliability
verified: 2026-08-18T12:00:00Z
status: passed
score: "4/4 success criteria verified, 4/4 RELIAB requirements met, build+vet clean, full -race regression suite green incl. 15x concurrent-package stress"
behavior_unverified:
  - Live daemon SIGINT shutdown with a connected BGP peer (unit-level cancellation verified; end-to-end signal path needs networked peering, out of sandbox scope)
overrides_applied: 0
re_verification:
  previous_status: n/a
  previous_score: ""
  gaps_closed: []
  gaps_remaining: []
  regressions: []
gap_items: []
gaps: []
deferred:
  - DNS rate limiting / DNSSEC (explicitly out of Phase 04 scope per CONTEXT guardrail)
human_verification: {}
---

# Phase 04: Reliability — Verification Report

**Phase:** 04 — Reliability (RELIAB-01–04)
**Verified:** 2026-08-18
**Status:** PASSED
**Commits under test:** e14c8d2 (plan 01), fcaada0 (plan 02), 90abf46 (regression-gate race fixes) on top of baseline 2cb67c8

## Goal Achievement

**Goal:** Fix the four reliability gaps — DNS query timeout, O(n²) set difference, BGP context propagation, silent BGP error handling — with internal hardening only and no external behavior change.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Flat `Dns.Timeout` config field exists (yaml/json `Timeout`) | ✓ VERIFIED | internal/config/dns.go:22 `Timeout time.Duration` |
| 2 | Absent/misparsed value defaults to 5s; out-of-range rejected with named error | ✓ VERIFIED | Init() post-Unmarshal switch in internal/config/main.go (~:66-73): `<=0 → 5s`, `<3s \|\| >30s → fmt.Errorf("dns: timeout %v out of range (allowed: 3s-30s)")`; decodeHook `time.ParseDuration` branch at :55 makes YAML `"5s"` literals decode correctly |
| 3 | One shared cached `*dns.Client` per resolver replaces package-level `dns.Exchange` | ✓ VERIFIED | resolvers.go:26 `client *dns.Client` field; :45-55 `newResolver(a, timeout)` builds `&dns.Client{Net:"udp", Timeout}`; sole exchange site :100 `srv.client.Exchange(q, srv.addr.String())`; grep for package-level `dns.Exchange(` in non-test source = 0 |
| 4 | Cumulative `Timeout` (no Dial/Read/Write split) caps queries empirically | ✓ VERIFIED | `TestResolver_Timeout`: silent UDP listener + 3s config → query returns error with elapsed < 6s; fast fake server control case still succeeds (~3s added runtime) |
| 5 | `utils.Difference` is map-based O(n) with contract-pinned asymmetry | ✓ VERIFIED | internal/utils/main.go:12 — map membership per pass, single linear scan per half, dedup, nil-safe; doc comment states `slice1 \ slice2` contract explicitly |
| 6 | Asymmetric contract pinned by fresh unit tests (first coverage ever for utils) | ✓ VERIFIED | internal/utils/main_test.go `TestDifference`: 9 table cases (identical→nil, first-empty→nil **fresh-entry case**, second-empty→all, order preservation, duplicate collapse, overlap) + hand-picked 20/16-element pair cross-checked against an independent naive reference implementation |
| 7 | Cache gone/arrived semantics survive the rewrite (fresh entry ⇒ gone=nil, arrived=all IPs) | ✓ VERIFIED | `TestE2E_DnsQueryToCacheToBgp` ref-counter assertion (count 2 for A+HTTPS) and `TestE2E_MultiDomainIPSharing` pass under -race — the exact path that the wrong symmetric variant broke |
| 8 | `NewBgp` takes daemon ctx first; derives cancellable child stored on the server | ✓ VERIFIED | internal/bgp/main.go:46 `func NewBgp(ctx context.Context, cfg, l, logger)`; :57-58 `cctx, cancel := context.WithCancel(ctx); s.ctx = cctx` |
| 9 | All long-lived BGP calls + op loop run on the child ctx | ✓ VERIFIED | :61 `StartBgp(cctx,…)`, :95 `AddPeer(cctx,…)` per peer, :135 `go s.loop(cctx)` |
| 10 | add/find/remove use `s.ctx`; zero `context.Background()` left in bgp.go; //TODO removed | ✓ VERIFIED | `grep -c 'context.Background()' internal/bgp/bgp.go` = **0**; three call sites (former :47/:71/:106) now `s.ctx`; unused `context` import removed (build-clean) |
| 11 | Daemon wires its own ctx through; legacy Serve shim forwards | ✓ VERIFIED | cmd/bgp-dnsd/main.go:60 `bgp.NewBgp(ctx, cfg, …)` (daemon ctx built at :53-55); Serve shim body forwards its ctx param into NewBgp |
| 12 | Cancellation observably exits the op loop | ✓ VERIFIED | `TestBgpSrv_CancelExitsLoop`: bare srv, loop goroutine, cancel(), done signal within 5s deadline (done sourced from `loop()` returning — no wg contention); `TestBgpSrv_ContextStored` guards NewBgpSrvForTest storing srv.ctx |
| 13 | Failed Advance emits exactly one structured Warn (op/ip/router_id/peers+err), then aborts | ✓ VERIFIED | internal/bgp/main.go:194-202: `Warn().Str("op","advance").Str("ip",…).Str("router_id",…).Strs("peers",_bgp.peers).Err(e).Msg("BGP advance failed")` inside the Operation closure, `return` on named-return `e` (abort-on-first-error preserved) |
| 14 | Failed Withdraw mirrors the Warn; bare `Error().Err` emitter gone | ✓ VERIFIED | main.go:262-270 mirror shape `op=withdraw` / Msg "BGP withdraw failed"; `grep -rn '_bgp\.L()\.Error()' internal/bgp/` = **0** |
| 15 | Eviction-path withdrawal demoted Error→Warn (message unchanged); gcache store failure stays Error | ✓ VERIFIED | internal/dns/cache.go:58 `Warn().Err(e).Msgf("Failed to withdraw IPs for %s", ck)`; cache.go:117-119 `entries.Set` error remains `Error()` (classified out-of-scope at plan-check Defect 3) |
| 16 | No duplicate/no-op log churn at call sites | ✓ VERIFIED | cache.go:114-115 `_ = bgp.Advance(arrived)` / `_ = bgp.Withdraw(gone)` byte-identical to baseline; exactly-one-Warn invariant holds (emitters live in the closures, not at callers) |

**Score: 16/16 observable truths verified**

## Success Criteria → Requirements Map

| Criterion (ROADMAP §Phase 4) | Requirement | Verdict |
|---|---|---|
| SC-1: `dns.Client{Timeout}` configurable via YAML, default 5s | RELIAB-01 | PASS — truths 1-4 (naming: flat `Dns.Timeout` per locked D-04, supersedes ROADMAP's `Dns.Timeouts.Query`) |
| SC-2: O(n) map-based set difference in internal/utils/main.go | RELIAB-02 | PASS — truths 5-7 |
| SC-3: BGP ops use daemon-lifecycle cancellable context | RELIAB-03 | PASS — truths 8-12 |
| SC-4: Advance/Withdraw errors logged at Warn with peer-IP context | RELIAB-04 | PASS — truths 13-16 (`peers` field carries configured peer IPs) |

## Regression Gate

Per execute-phase step `regression-gate.md`: prior-phase floors extracted from 01/02/03-VERIFICATION.md — `internal/bgp/bgp_test.go`, `internal/dns/{resolvers,cache_eviction,cache,dns_e2e}_test.go`, `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` (all present; additional same-package files included by whole-package runs).

| Check | Result |
|---|---|
| Pre-execution baseline @ 2cb67c8: `go build ./... && go vet ./... && go test -race ./...` | GREEN (all suites, deps downloaded fresh) |
| Post-plan-01 full gate | GREEN — new tests `TestInit_DnsTimeout`, `TestResolver_Timeout`, `TestDifference` green |
| Post-plan-02 full gate | GREEN — new tests `TestBgpSrv_ContextStored`, `TestBgpSrv_CancelExitsLoop` green; grep invariants clean (0× `context.Background()` in bgp.go, 0× `_bgp.L().Error()`) |
| Concurrent 3-package stress (15× `go test -race ./internal/bgp/ ./internal/dns/ ./cmd/bgp-dnsd/cli/`) | 15/15 race-free **after** 90abf46 |
| Final full suite `go test -race -count=1 ./...` | GREEN — commands/cli/bgp/config/debugdetect/dns/utils all `ok` |

### Races caught and fixed by the gate (commit 90abf46)

1. **Pre-existing latent race** in `setResolvers` (internal/dns/resolvers.go): `iter.ForEach` (sourcegraph/conc) spawns one goroutine **per element**, and each concurrently mutated the shared `ring.Ring` pointer (`Value=`/`Next()`) — the mutex only serializes against other methods, not between ForEach workers. Present since Phase 2; surfaced under CPU contention when the three regression binaries ran concurrently (reproduced ~1-in-3). Fixed by replacing with a sequential `for _, a := range c` loop; `sourcegraph/conc` import removed (zero remaining users in internal/). Behavior-identical: ring population was never observed externally mid-build (constructors return after building).
2. **Race in new test** `TestBgpSrv_CancelExitsLoop` (exposed once #1 stopped failing the sibling binary): done signal raced `wg.Wait()` ahead of the loop's `wg.Add(1)` (unsynchronized sync.WaitGroup misuse). Fixed by sourcing done from `loop()` returning via `defer close(done)`.

Both fixes are within this phase's diff; no behavioral change (construction-time mutation only; test scaffolding only).

## Required Artifacts

- `.planning/phases/04-reliability/04-CONTEXT.md` — decisions D-04/D-05/D-10..D-13 locked, D-06 resolved during plan
- `04-RESEARCH.md` — orchestrator-inline research (subagent boot limitation), ends RESEARCH COMPLETE
- `04-01-PLAN.md` / `04-02-PLAN.md` — both executed; disjoint file sets
- `04-CHECK.md` — plan-checker BLOCK→PASS-after-revision (gate scoping, cache.go:118 classification)
- `04-01-SUMMARY.md` / `04-02-SUMMARY.md` — status: complete; deviations recorded (asymmetric Difference correction vs plan text; decodeHook duration branch; zerolog `Strs` slice form; no-shell executor sessions → orchestrator gates)
- `04-REVIEW.md` — advisory code review (execute:post hook)
- `04-VERIFICATION.md` — this document

## Deviations Carried From Summaries (cross-referenced)

| Deviation | Where | Impact |
|---|---|---|
| Difference implemented asymmetric (slice1\slice2) instead of planned "two-pass symmetric" | 04-01-SUMMARY §Deviations #1 | Contract-correcting: old loop ran once; symmetric variant broke E2E. Locked as intended contract with pinning tests |
| decodeHook gains string→time.Duration branch | 04-01-SUMMARY §Deviations #2 | Enables YAML `"5s"` literals; without it every duration string failed Unmarshal |
| `Strs("peers", _bgp.peers)` slice form (not variadic splat) | 04-02-SUMMARY §Deviations #1 | Compile requirement of zerolog v1.34; nil-slice panic-free retained |
| Two data-race fixes at regression gate | this report, above | Reliability in scope; see commit 90abf46 |

## Behavior Equivalence Notes

- Announce/withdraw semantics unchanged: ref-count gates (`c==1` / `c<1`) untouched; duplicate-collapse in Difference is a no-op delta because duplicates within one call were always ignored by the gating.
- Current production effective timeout moves from the library-implicit 2s (bare `dns.Exchange`) to the configured 5s default — a relaxation, documented at decision D-04.
- Shutdown ordering protects ctx plumbing: StopBgp → cancel → Stop → wg.Wait (unchanged).

**Verdict: PASSED — all four success criteria independently evidenced; regression floor intact with strengthened race coverage.**
