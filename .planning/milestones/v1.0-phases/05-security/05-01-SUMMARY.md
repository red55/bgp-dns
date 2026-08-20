---
phase: 05-security
plan: "01"
subsystem: security
tags: [grpc, unix-socket, permissions, interceptor, validation, zerolog, audit, redaction]
requirements: [SEC-01, SEC-02]
commit: NONE (orchestrator commits) — shell-less executor, changes left UNCOMMITTED
commits: 0
requires: []
provides:
  - Unix admin socket forced to owner-only 0600 before the gRPC server accepts any connection (SEC-01)
  - Unified gRPC server-side interceptor gate: ONE shared audit closure chained for unary + stream — the hook point the roadmap's "enhanced checks" lives behind (SEC-02)
  - Test harness newTestApp() + serveUnixAndAssert0600() that drives real cli.Serve() against t.TempDir() unix targets
  - Nil-rejection coverage for ClearCache + ListCacheEntries (direct calls), completing the 3-RPC set alongside pre-existing TestGRPC_ReloadList_NilRequest
affects:
  - Any future RPC added to BgpDnsService inherits the audit gate automatically; nil-request rejection must remain in the handler (the gate does not inspect message shape)
  - Observability: new per-RPC zerolog Debug events (rpc=<FullMethod>, elapsed=<dur>)
actuals:
  tokens: ~1600   # chars/4 over realized diff (est.; static, shell-less session)
  tasks: 3
  commits: 0      # NONE — see commit field above
tech-stack:
  added: []       # no new deps — grpc v1.73.0, zerolog v1.34.0, testify v1.10.0 already in go.mod
  patterns:
    - "explicit os.Chmod post-Listen for filesystem permission boundaries (never rely on process umask)"
    - "ChainUnaryInterceptor + ChainStreamInterceptor over ONE shared audit closure"
    - "drive the real production entrypoint (cli.Serve) in-package against t.TempDir() to pin OS-level side effects"
key-files:
  created:
    - cmd/bgp-dnsd/cli/socket_test.go
  modified:
    - cmd/bgp-dnsd/cli/cache.go
    - cmd/bgp-dnsd/cli/grpc_lifecycle_test.go
key-decisions:
  - "os.Chmod(target, 0o600) inserted BETWEEN net.Listen success and the `go grpcServer.Serve(l)` start, guarded by proto==\"unix\" only (tcp branch untouched); fatal-on-error mirrors the file's established `Fatal().Err(e).Msgf` + `return e` style"
  - "ONE shared audit closure (auditRPC) adapted into both ChainUnaryInterceptor and ChainStreamInterceptor; emits exactly one zerolog Debug event (rpc=info.FullMethod, elapsed=time.Since) per RPC and forwards untouched — NO request-shape decisions at the gate (a serialized nil arrives as an empty message and is indistinguishable there — Pitfall 5)"
  - "Per-handler nil checks (ListCacheEntries stream==nil, ClearCache req==nil, ReloadList req==nil) left byte-for-byte authoritative; the interceptor is a uniform audit hook, NOT the rejector"
  - "grpc v1.73.0 exposes info.FullMethod as a string FIELD, not a method — RESEARCH pseudocode `info.FullMethod()` corrected to field access `info.FullMethod` (verified against module cache interceptor.go)"
  - "Non-nil-passes-gate pinned to codes.FailedPrecondition (not merely NotEqual InvalidArgument) because the test server leaves DNS uninitialized — proves the request reached handler business logic"
  - "Stale-socket variant pre-creates a 0644 regular file at the socket path (umask-independent) to pin the Stat→Remove→Listen→Chmod path"
  - "Serve-based test saves/restores package globals (_cancel/_listener/_listFile) in t.Cleanup, defensively closes the captured _listener after Shutdown, and never calls t.Parallel (global-state invariant)"
patterns-established:
  - "Permission-boundary hardening: normalize mode with os.Chmod immediately after the syscall that creates the object, before it becomes reachable"
  - "A single server-side interceptor as the one structured-audit point for every RPC"
  - "Serve-based tests are sequential, save/restore package globals in t.Cleanup, and use t.TempDir() exclusively (never /run paths)"
coverage:
  - id: D1
    description: "cli.Serve() on a unix:// target yields a socket whose stat perm bits are exactly 0600 (fresh bind + stale-removal variants)"
    requirement: SEC-01
    verification:
      - kind: unit
        ref: "cmd/bgp-dnsd/cli/socket_test.go#TestServe_UnixSocketMode0600"
        status: pending-orchestrator
    human_judgment: false
  - id: D2
    description: "Direct method calls with nil request/stream on all three RPCs (ClearCache, ReloadList, ListCacheEntries) return codes.InvalidArgument"
    requirement: SEC-02
    verification:
      - kind: unit
        ref: "cmd/bgp-dnsd/cli/grpc_lifecycle_test.go#TestGRPC_ClearCache_NilRequest / #TestGRPC_ListCacheEntries_NilStream / #TestGRPC_ReloadList_NilRequest"
        status: pending-orchestrator
    human_judgment: false
  - id: D3
    description: "Valid empty-but-non-nil requests traverse the new interceptor chain and reach business logic (FailedPrecondition, never InvalidArgument)"
    requirement: SEC-02
    verification:
      - kind: unit
        ref: "cmd/bgp-dnsd/cli/grpc_lifecycle_test.go#TestGRPC_ServeShutdown (non-nil passes gate assertions)"
        status: pending-orchestrator
    human_judgment: false
duration: ~35min
completed: 2026-08-19
status: complete
---

# Phase 5 Plan 01 Summary

**The daemon's gRPC admin channel is hardened end-to-end: the Unix control socket is forced to owner-only 0600 *before* the server accepts any connection (SEC-01), and every RPC now passes through one unified server-side interceptor gate that emits a single structured zerolog audit event per call — while the existing per-handler nil checks remain the authoritative rejectors (SEC-02).**

This is the Phase 5 tracer: the thinnest vertical cut through the most load-bearing requirement (socket permissions ARE the entire authentication mechanism for the CLI control plane), delivered production-quality through the real `Serve()` path, with the adjacent SEC-02 validator wired into the same function.

## Performance

- **Duration:** ~35 min (code-complete, static self-check only — this executor session has NO shell, so no build/vet/test ran here)
- **Tasks:** 3/3 implemented, all pending orchestrator execution of their `<verify>` gates
- **Commits:** 0 — changes left UNCOMMITTED in the working tree; the orchestrator performs the atomic commit after its green gate

## Accomplishments

- **SEC-01 (socket 0600):** In `cmd/bgp-dnsd/cli/cache.go` `Serve()`, immediately after the `net.Listen(proto, target)` success block, a `proto == "unix"`-guarded `os.Chmod(target, 0o600)` runs *before* the `go grpcServer.Serve(l)` goroutine starts — so no connection can arrive on a world-accessible socket. On error it follows the file's established fatal style (`_app.L().Fatal().Err(e).Msgf("CLI Service: Failed to set mode 0600 on %s", target)` then `return e`). The tcp branch is untouched; the commented-out `MkdirAll` block is untouched; no `syscall.Umask` introduced.
- **SEC-02 (unified validation gate):** The bare `grpc.NewServer()` is replaced by `grpc.NewServer(grpc.ChainUnaryInterceptor(...), grpc.ChainStreamInterceptor(...))`. Both chains share ONE small closure `auditRPC(method, start)` that emits a single zerolog event (`_app.L().Debug().Str("rpc", method).Dur("elapsed", time.Since(start)).Msg("CLI Service: gRPC call")`) and forwards untouched. The gate makes NO request-shape decisions (a nil serializes to an empty message, indistinguishable here — Pitfall 5); nil-rejection authority stays in the three per-handler checks, which are byte-for-byte intact (stream==nil, req==nil, req==nil).
- **Tests:**
  - NEW `cmd/bgp-dnsd/cli/socket_test.go` — `TestServe_UnixSocketMode0600` (table-driven: fresh + stale subtests) drives real `Serve()` against `t.TempDir()` unix targets and asserts `fi.Mode().Has(os.ModeSocket)` and `fi.Mode().Perm() == 0o600` exactly; the stale variant pre-creates a 0644 file to pin the removal path. Helper `newTestApp()` builds a hand-wired `*app.Application` with a `zerolog.Nop()` logger. Globals saved/restored; no `t.Parallel()`.
  - `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` — added `TestGRPC_ClearCache_NilRequest` and `TestGRPC_ListCacheEntries_NilStream` (direct impl calls, per Pitfall 5), completing the 3-RPC nil leg with pre-existing `TestGRPC_ReloadList_NilRequest`. Extended `TestGRPC_ServeShutdown` with explicit non-nil-passes-gate assertions: a valid `&api.ClearCacheRequest{}` returns exactly `FailedPrecondition` (NOT InvalidArgument), proving the request reached business logic.
- **Static self-check (no shell):** repo-wide re-grep proves zero stale call sites and that all five new symbols are unique; no `Umask(` anywhere in `cmd/` or `internal/`; no logger call references `AuthPassword`; import usage audited (all imports in touched files still used — removing none broke `go build` preconditions); grpc API signatures verified against the local module cache.

## TDD Cycle (RED → GREEN) — recorded, not executed

Per the plan's documented single cross-file RED→GREEN cycle (Task 1 = tracer implementation; Task 2 = the W0 test file landing alongside it):

- **RED expectation (static; cannot execute here — orchestrator confirms):** With the Task-1 `os.Chmod` removed, `go test ./cmd/bgp-dnsd/cli/ -run TestServe_UnixSocketMode0600 -count=1` fails the `fresh` subtest. `net.Listen("unix", …)` creates the socket honoring the process umask; under a typical umask 0022 the result is `srwxr-xr-x` (perm `0o755`), so the assertion `assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())` reports expected `0o600` (384) vs got `0o755` (493). This proves the test is **not** self-fulfilling — it genuinely exercises the chmod, and stays correct independent of the runner's umask (RESEARCH Pitfall 1).
- **GREEN (pending orchestrator):** With Task 1's `os.Chmod` present, the same command reaches exactly `0o600` for both subtests.

## Task Results

| # | Type | Name | Result |
|---|------|------|--------|
| 1 | tracer | End-to-end "0600-controlled admin socket" — bind, harden, validate, serve (chmod + unified interceptor in `cache.go`) | Implemented — pending orchestrator verify |
| 2 | auto/tdd | Wave 0 — socket permission test file `socket_test.go` (fresh + stale) | Implemented — RED expectation recorded; pending orchestrator GREEN |
| 3 | auto/tdd | Nil-rejection for all three RPCs + non-nil-passes-gate assertions in `grpc_lifecycle_test.go` | Implemented — pending orchestrator verify |

## PENDING ORCHESTRATOR VERIFICATION

No shell in this session — the following exact commands (from each task's `<verify>` block plus the VALIDATION.md wave-merge + cross-cut gates) must be run by the orchestrator and are the acceptance gate for this plan. Run from the repo root:

```bash
# Task 1 verify (tracer)
go test ./cmd/bgp-dnsd/... -count=1 && go mod tidy && git diff --exit-code go.mod go.sum

# Task 2 verify (Wave 0 socket test — RED→GREEN companion to Task 1)
go test ./cmd/bgp-dnsd/cli/ -run TestServe_UnixSocketMode0600 -count=1

# Task 3 verify (nil-rejection legs + gate passthrough)
go test ./cmd/bgp-dnsd/cli/ -run 'TestGRPC_(ClearCache|ReloadList|ListCacheEntries)_Nil|TestGRPC_ServeShutdown' -count=1

# Cross-cut: dependency drift must be zero
go mod tidy && git diff --exit-code go.mod go.sum

# VALIDATION.md quick run (after every task commit)
go test ./cmd/bgp-dnsd/... ./internal/dns/... ./internal/bgp/... ./internal/config/...

# VALIDATION.md wave merge gate / full suite
go vet ./... && go build ./... && go test ./...
```

Expected outcomes: all green; `go.mod`/`go.sum` unchanged by `go mod tidy` (no new dependencies were added). Regression floor includes pre-existing `TestGRPC_ReloadList_NilRequest` and the full `cmd/bgp-dnsd/cli` suite.

## Never-Log-Secrets Compliance

The unified interceptor logs **only** `info.FullMethod` (a method-name string such as `/api.BgpDnsService/ClearCache`) and elapsed time — never the request body, reply, or any peer-config/credential content. There is no `AuthPassword` (or other secret) in the CLI surface at all. Verified by grep: no logger call references a credential field, and no `Umask(` appears anywhere in `cmd/`/`internal/`. This satisfies the plan prohibition "Never log AuthPassword or peer-config credential content; mask as set/unset if logging peer config" (vacuously — nothing of the sort is logged).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - correctness] `info.FullMethod()` corrected to field access `info.FullMethod`**
- **Found during:** Task 1 (writing the interceptor)
- **Issue:** RESEARCH.md Pattern 2 pseudo-code wrote `info.FullMethod()` as a method call. Against grpc v1.73.0 (module-cache `interceptor.go:71,93`), `FullMethod` is a `string` **field** on both `UnaryServerInfo` and `StreamServerInfo`; invoking it as a method would not compile.
- **Fix:** Use `info.FullMethod` (field) in `auditRPC(method string, …)`; the closure receives the string and both adapters pass `info.FullMethod`.
- **Files modified:** `cmd/bgp-dnsd/cli/cache.go`
- **Verification:** static — signature checked against `grpc@v1.73.0/interceptor.go`; orchestrator `go build ./...` to confirm.
- **Impact on plan:** none on intent — method name + elapsed still emitted per RPC; no behavioral change.

### Design constraints honored (NOT deviations)
- Per-handler nil checks preserved byte-for-byte and remain the authoritative rejectors (interceptor does not shadow them).
- Commented-out `MkdirAll` block in `Serve()` left untouched, as mandated.
- `tcp` listeners receive no chmod behavior change (guard is `proto == "unix"` only).
- No new goroutines: interceptors run inline in gRPC handlers; the only goroutine is the pre-existing serve goroutine.
- Existing test bodies untouched except the additive non-nil assertions appended to `TestGRPC_ServeShutdown`.

**Total deviations:** 1 auto-fixed (compile-blocking, invisible to the plan author because no compiler ran at plan/research time). Impact on plan: none on intent.

## Issues Encountered

- **No shell in this executor session:** `go build/vet/test` could not be run here. All confidence comes from exhaustive static re-greps (stale call sites, symbol uniqueness, import usage, grpc API signatures against the module cache) and direct reads of the edited regions. The orchestrator MUST run the full gate in **PENDING ORCHESTRATOR VERIFICATION** before committing.
- Tooling note: anchored line-range regexes on PLAN.md did not match via `grep_search`; all needed spec was recovered through content-pattern greps (no plan detail lost).

## User Setup Required

None — no external service configuration required. (Rollout note carried from research: already-running daemon instances pick up 0600 only at next restart.)

## Next Phase Readiness

Code-complete for SEC-01 + SEC-02, pending the orchestrator's green `go vet/build/test ./...` + zero-diff `go mod tidy` gate. The SEC-02 interceptor is the designated hook point for the roadmap's later "enhanced checks"; expansion plans build outward from this hardened admin channel. Files touched are confined to `cmd/bgp-dnsd/cli/` (disjoint from the DNS qtype-filter / BGP auth work in later plans).

## Self-Check (static)

- [x] `cmd/bgp-dnsd/cli/socket_test.go` exists with `TestServe_UnixSocketMode0600` (+ helpers `newTestApp`, `serveUnixAndAssert0600`) — confirmed by symbol re-grep
- [x] `TestGRPC_ClearCache_NilRequest` + `TestGRPC_ListCacheEntries_NilStream` present in `grpc_lifecycle_test.go`; `TestGRPC_ServeShutdown` extended with non-nil-passes-gate assertions
- [x] `cache.go` contains the `proto == "unix"` chmod guard positioned between `net.Listen` and `go Serve`, and the `ChainUnaryInterceptor`+`ChainStreamInterceptor` wiring over one shared `auditRPC`
- [x] Per-handler nil checks (L112/L137/L153) byte-for-byte intact; commented `MkdirAll` block untouched
- [x] No `Umask(` in `cmd/` or `internal/`; no logger references `AuthPassword`
- [x] All imports in touched files still used (no orphaned import that would break `go build`)
- [x] Only `cmd/bgp-dnsd/cli/{cache.go, socket_test.go, grpc_lifecycle_test.go}` changed; no edits to STATE.md / ROADMAP.md / other plans' files
- [ ] Dynamic build/vet/test — DEFERRED to orchestrator (no shell here); see PENDING ORCHESTRATOR VERIFICATION

Self-Check: PASSED (static; dynamic gates deferred to orchestrator)

*Phase: 05-security · Plan: 01 (tracer)*
