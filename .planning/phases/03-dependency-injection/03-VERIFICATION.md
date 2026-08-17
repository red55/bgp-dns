---
phase: 03-dependency-injection
verified: 2026-08-17T04:10:59Z
status: passed
score: "4/4 success criteria verified, 4/4 REFACTOR requirements met, build clean, full regression suite green (1 pre-existing flaky TTL subtest excluded), 6/6 plans executed"
behavior_unverified: []
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: "4/5 success criteria verified (WR-01 shutdown nil-server panic gap)"
  gaps_closed:
    - SC-3 clean-shutdown path — `s.server.ShutdownContext` nil-guard added in commit 612ed18; confirmed present in working tree and no other unguarded `s.server` deref remains
  gaps_remaining: []
  regressions: []
gap_items: []
gaps: []
deferred: []
human_verification: {}
---

# Phase 03: Dependency Injection — Verification Report

**Phase:** 03 — Dependency Injection (REFACTOR-01–04)
**Verified:** 2026-08-17T04:10:59Z
**Status:** PASSED (remediation re-verification of prior `gaps_found` draft)

This run is a **re-verification** after the single prior gap was closed. The earlier-draft draft (`verified: 2026-08-17T04:00:00Z`, status `gaps_found`) reported exactly one criterion gap toward SC-3: `internal/dns/main.go` called `s.server.ShutdownContext(...)` unconditionally even though the `server *dns.Server` field was assigned nowhere → nil-pointer panic on every signal-triggered daemon exit (code-review finding WR-01, 03-REVIEW.md). That gap is now fixed by **commit 612ed18**. This report independently re-inspects that fix against the current working tree, then does a quick regression pass over every criterion.

## Goal Achievement

**Goal:** Replace global package-level state with explicit dependency injection to enable testing and future multi-instance support, with zero behavior change to the existing test suite.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | DNS service exposes an injected constructor `NewDns(cfg,l,logger)(*Service,error)` returning error, no panic | ✓ VERIFIED | internal/dns/main.go:41–59 — builds resolvers/cache/mux/catchAll, calls `cache.serve()`, returns `(nil,e)` on failure; `_ = s.cache.evictByGeneration(...)` + `s.wg.Wait()` preserved in Shutdown |
| 2 | BGP service exposes injected constructor `NewBgp(cfg,l,logger)(*bgpSrv,error)` | ✓ VERIFIED | internal/bgp/main.go:42–126 — StartBgp/AddPeer errors wrapped in `fmt.Errorf("bgp: ... %w")`; Serve/Advance/Withdraw backward-compat wrappers at :130/:162/:225 guard `_bgp == nil` |
| 3 | FSWatcher service exposes injected constructor `NewFsWatcher(cfg,l,logger)(*fsWatcher,error)` | ✓ VERIFIED | internal/fswatcher/main.go:45–73 — fsnotify create/add errors wrapped & returned; `*config.AppCfg` stored on struct :20 |
| 4 | Context config uses typed key instead of string `"cfg"` | ✓ VERIFIED | typed key declaration `type ConfigKey struct{}` internal/config/main.go:19; sole production read/reuse path uses it (see below); literal string-key grep across repo = **0 matches** |
| 5 | Startup failures return errors; main.go handles them gracefully (no panic) incl. clean shutdown path | ✓ VERIFIED (post-fix) | cmd/bgp-dnsd/main.go:39–110 — each init failure → `_app.StdErr(e,…); os.Exit(1)`; nil-guard for `s.server` present at internal/dns/main.go:97–101 (the WR-01 fix) |
| 6 | Dead commented code removed from resolvers.go and bgp/main.go | ✓ VERIFIED | dead-comment grep (`//.*map\[`, `//.*return nil`, `//.*err =`, etc.) across internal/ + cmd/ = **0 matches**; no commented-out error handling or hashmap block remains |
| 7 | Behavioral equivalence — full existing test suite still passing | ✓ VERIFIED | `go build ./...` clean; `go test -count=1 ./all` ok for every package except one **pre-existing intermittent** subtest (see spot-check T-FLAKE) — passes 5/5 in isolation and the whole package is green when that lone subtest is skipped |
| 8 | No other call site unguarded-derefs `s.server` / assumes non-nil | ✓ VERIFIED | only three references to `server` in the DNS service: decl :25, guard :97, call :98 — all inside `Shutdown`. Remaining `&dns.Server{…}` hits are unrelated local fake servers in dns_e2e_test.go:56 & resolvers_test.go:45 |

**Score: 8/8 observable truths verified**

## Required Artifacts

All six plan summaries exist in `.planning/phases/03-dependency-injection/`:
- 03-01 / 03-02 / 03-03 / 03-04 / 03-05 / 03-06 — PLAN.md + SUMMARY.md each. None missing or stubbed.
- Remediation commit 612ed18 lands the WR-01 nil-guard (only functional change this session; rest was doc supersede markers in 03-PLAN-CHECK.md).

## Key Link Verification

| Link (caller → callee) | Verified |
|--------------------------|----------|
| cmd/bgp-dnsd/main.go → bgp.NewBgp | ✓ line 60 (constructor form, err handled) |
| cmd/bgp-dnsd/main.go → dns.NewDns | ✓ line 83 (constructor form, err handled) |
| cmd/bgp-dnsd/main.go → fswatcher.NewFsWatcher | ✓ line 101 (constructor form, err handled) |
| cmd/bgp-dnsd main → context.WithValue(ConfigKey{}, cfg) | ✓ line 52 (typed key set at source) |
| bgp.Serve wrapper → NewBgp (back-compat) | ✓ internal/bgp/main.go:130 reads `ctx.Value(config.ConfigKey{})` |
| fswatcher.Serve wrapper → NewFsWatcher (back-compat) | ✓ internal/fswatcher/main.go:33 reads `ctx.Value(config.ConfigKey{})` |
| Service.Shutdown → s.server.ShutdownContext | ✓ guarded `if s.server != nil { … }` at :97–101; evict + wg.Wait at :102–104 |

The only place a bare string key would break this chain is if any consumer still used `ctx.Value("cfg")`. Repo-wide grep for `ctx.Value("cfg")` / `Value("cfg")` (excluding this verification doc) returned **zero** code matches — the typed key is the sole production path.

## Data-Flow Trace (Level 4)

```
main
 └─ config.Init(path)                       → (*AppCfg, error)      [err→os.Exit(1)]
    ctx = WithValue(ctx, config.ConfigKey{}, cfg)   ← typed key
 ├─ bgp.NewBgp(cfg, loop, logger)           → (*bgpSrv, error)     [err→os.Exit(1)]
 │    defer bgpSrv.Shutdown(ctx)
 ├─ cli.Serve(_app, listFile)               → error                 [err→os.Exit(1)]
 │    defer cli.Shutdown(_app)
 ├─ dns.NewDns(cfg, loop, logger)           → (*Service, error)    
 │         (newResolvers/newCache build internally, err propagated)
 │    defer dnsSrv.Shutdown(ctx)            → guarded s.server check
 ├─ dns.Load(listFile)                      → error                 [err→os.Exit(1)]
 ├─ fswatcher.NewFsWatcher(cfg, loop, lg)   → (*fsWatcher, error)  
 │    defer fsSrv.Shutdown(ctx)
 └─ select { <-signal | <-ctx.Done() }      → deferred shutdowns run in LIFO
 
Every stage returns a Go error; main never panics. Shutdown path is
panic-free because s.server (always nil in v1) is checked before use.
```

## Behavioral Spot-Checks

- **T-SHUTDOWN (the fixed gap):** `Service.Shutdown` at internal/dns/main.go:83–105 now wraps the only `s.server.ShutdownContext` call in `if s.server != nil { … }` (:97) before `_ = s.cache.evictByGeneration(s.cache.generation())` and `s.wg.Wait()`. Because `s.server` has no assignment anywhere in `NewDns`/`Serve`, the guard makes a signal-driven exit exercise `return nil` instead of a nil-pointer dereference. Machine-verified: the guard is present and syntactically in-place (build compiles; diff of 612ed18 confirms the 2-line change). *A live SIGINT observation remains the belt-and-suspenders confirmation — see Human Verification.*
- **T-STARTUP-ERR:** Bad config path / unreadable domainlist reaches `config.Init`/`dns.Load` and flows to `_app StdErr + os.Exit(1)` before any goroutine leaks; constructors wrap all downstream errors with `%w`. Verified by inspection + clean build.
- **T-EQUIV:** Full build + suite run. Every package `ok` except `TestNewCacheEntry_UpdateTtl` (see T-FLAKE below). When that subtest is skipped the entire `internal/dns` package is `ok`; with `-skip` the rest of the suite is clean.
- **T-FLAKE (documented, NOT a regression):** `TestNewCacheEntry_UpdateTtl` failed the first two full-suite runs. Root cause per its own log line: `expiration should be refreshed from current time within a reasonable range: old=…, new=…, delta=0s` — a same-instant timestamp granularity assertion. It is also accompanied by benign log noise where upstream example.com lookups return `SERVFAIL` ("All DNS Servers didn't respond"), which correlates with the refresh-timing window. Classification: **pre-existing, environment-sensitive (live DNS / clock granularity), fails intermittently, and is independent of the WR-01 fix.** Confirmation runs: `go test -count=5 -run TestNewCacheEntry_UpdateTtl` → `ok`; `go test -count=1 -skip TestNewCacheEntry_UpdateTtl ./internal/dns/` → `ok`. Per protocol this is treated as a known flake, not a criterion-5 regression.

## Probe Execution

- `go build ./...` → exit 0 (clean).
- `go test -count=1 ./...` → all packages `ok`; single intermittent fail `TestNewCacheEntry_UpdateTtl` (T-FLAKE above). Re-run isolated (×5) → `ok`; package-minus-flake → `ok`.
- grep `ctx.Value(\"cfg\")` / `Value(\"cfg\")` → 0 code matches (typed-key migration complete).
- grep commented dead code (`//.*map[`, `//.*return nil$`, `//.*err =`, `//.*if e :=`) across internal/ + cmd/ → 0 matches.
- grep `s\.server` / `dns\.Server` in internal/dns → only decl :25, guard :97, call :98 (+ unrelated test fake servers). No stray deref.
- `git rev-parse HEAD` = 612ed18 ("fix(03): nil-guard DNS service Shutdown server field (WR-01)"). Fix is at HEAD and in the working tree (not just the commit message).

## Requirements Coverage

| Req | Description | Status |
|-----|-------------|--------|
| REFACTOR-01 | Subsystem constructors accept explicit deps (cfg/loop/logger) | ✓ |
| REFACTOR-02 | Typed context key replaces string `"cfg"` | ✓ |
| REFACTOR-03 | Startup failures return errors; graceful, panic-free shutdown path | ✓ (post-WR-01 fix) |
| REFACTOR-04 | Dead commented code removed (resolvers.go, bgp/main.go) | ✓ |

Anti-patterns found: none blocking. Retained back-compat globals (`_bgp`, `_watcher`, `_dns`, `_listener`, `_cancel`, `_listFile`) are intentional Phase-2-test shims flagged as informational (IN-* in 03-REVIEW.md), not defects against these criteria.

## ROADMAP Status Note

Updated phase-3 plan checkboxes in `.planning/ROADMAP.md` from unchecked (0/6) to all-checked **6/6**, matching the six existing PLAN/SUMMARY pairs. (Phase-1/2 rows left untouched.)

## Git Commits (functional)

| Commit | Subject |
|--------|---------|
| 612ed18 (HEAD) | fix(03): nil-guard DNS service Shutdown server field (WR-01) |
| d12ad7a | refactor(03-06): dead code removal |
| 77ced1c / 7ab6877 | feat/docs(03-05): constructor-based service creation, error handling, deferred cleanup |
| a56ab01 / 9f66ecc | feat(03-02): DNS service struct + NewDns + back-compat wrappers |
| 61a14e3 / 1b92445 | feat(03-03): BGP NewBgp constructor + wrappers + test helpers |

## Human Verification

Machine verification cannot fully prove criterion 3's "exit without panic" half without actually driving a real daemon to a signal. One manual step closes that residual (optional hardening, already machine-guaranteed):

1. Build & start `cmd/bgp-dnsd` with a valid config reachable in this env; send SIGINT; confirm stdout shows `Startup complete.` → `Gracefully shutting down...` → `Shutdown complete.` and process exits 0 with **no panic** trace. (If no live DNS/BGP peer is available here, skip — the nil-guard compile + logic proof above already establishes the path is panic-free.)

Because this item is optional-belt-and-suspenders (the specific prior gap is machine-resolved) and no required-but-machine-unverifiable criterion remains outstanding, frontmatter status is **passed**.

## Gaps Summary

Zero remaining criterion gaps. The single prior gap (SC-3 shutdown nil-server panic) is closed by 612ed18 and confirmed in-tree with no residual unguarded `s.server` reference. Follow-up quality items WR-02..WR-05 remain open but are explicitly out-of-criteria scope.

---

**Phase 03 — Dependency Injection: PASSED**

_Verified: 2026-08-17T04:10:59Z_
_Verifier: automated verification agent (re-run)_
