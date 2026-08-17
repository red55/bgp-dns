---
phase: 03-dependency-injection
reviewed: 2026-08-17T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - cmd/bgp-dnsd/cli/cache.go
  - cmd/bgp-dnsd/cli/grpc_lifecycle_test.go
  - cmd/bgp-dnsd/main.go
  - internal/bgp/bgp.go
  - internal/bgp/bgp_test.go
  - internal/bgp/main.go
  - internal/config/main.go
  - internal/config/test.go
  - internal/dns/cache.go
  - internal/dns/cacheEntry.go
  - internal/dns/cacheEntry_test.go
  - internal/dns/cache_eviction_test.go
  - internal/dns/cache_test.go
  - internal/dns/dns_e2e_test.go
  - internal/dns/loop.go
  - internal/dns/main.go
  - internal/dns/resolvers.go
  - internal/dns/resolvers_test.go
  - internal/fswatcher/loop.go
  - internal/fswatcher/main.go
  - internal/loop/main.go
findings:
  critical: 0
  warning: 5
  info: 4
  total: 9
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-08-17T00:00:00Z
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

Review covers the phase 03 dependency-injection refactor (constructor-based `bgp.NewBgp`, `dns.NewDns`, `fswatcher.NewFsWatcher` alongside legacy package-global `Serve` wrappers, typed config context key, panic-to-error constructor migration, and supporting test infrastructure). File scope came from SUMMARY.md extraction cross-checked against the git diff (base `883f5ff^`); the diff surfaced 8 additional changed files beyond the summaries (cli gRPC code, bgp path logic, cacheEntry, config test helper), which were added to scope. Non-source files (`.devcontainer/devcontainer-lock.json`, `.idea/bgp-dns.iml`) are in the diff but excluded as generated/tooling.

High-level assessment: the DI design itself is sound — constructors return `(svc, error)`, globals remain only as backward-compat shims for Phase 2 tests, and shutdown paths exist per instance. The main risks are leftover global state in the CLI gRPC service and two nil-reference crash paths on legitimate runtime/shutdown sequences. No security vulnerabilities or data-loss risks found.

## Warnings

### WR-01: Service.Shutdown will panic on `s.server` when no listener was attached

**File:** `internal/dns/main.go:87`
**Issue:** `Service.server` (field declared at internal/dns/main.go:22) is never assigned anywhere in the package, yet `Shutdown` unconditionally calls `s.server.ShutdownContext(shutdownCtx)` at line 87 without a nil check. Every path through `NewDns`/`Serve` leaves `server == nil`, so calling `dnsSrv.Shutdown` (which `cmd/bgp-dnsd/main.go` defers unconditionally in main) nil-dereferences and panics at exit.
**Fix:**
```go
if s.server != nil {
    if e := s.server.ShutdownContext(shutdownCtx); e != nil && !errors.Is(e, context.Canceled) {
        return e
    }
}
```

### WR-02: CLI Serve() can race / bind over an existing domainlist socket and leaves no recovery from Fatal

**File:** `cmd/bgp-dnsd/cli/cache.go:53-62`
**Issue:** For unix-socket targets, `os.Stat(target)` result is interpreted with `!os.IsNotExist(e)` — when `Stat` fails for reasons other than ENOENT (permission errors, races where the file vanished between Stat and Remove), `os.Remove(target)` runs and its error is logged Fatal. Also, `Fatal` exits the process from inside a setup function while `Serve` still returns `e` to a caller (`main.go`) that would also `os.Exit(1)`; double-exit is benign but hides cleanup of `_listener` opened before the fatal path in edge ordering. More importantly, removing an existing socket file means a second instance silently steals/replaces the first instance's endpoint rather than failing loudly — operator-visible data-plane confusion.
**Fix:** Check `statErr == nil && !info.Mode()&fs.ModeSocket == false` to confirm it is actually our own stale socket before removal; otherwise return an error instead of Fatal, and let the single exit path in main handle it:
```go
if _, statErr := os.Stat(target); statErr == nil {
    _app.L().Error().Msgf("CLI Service: socket %s already exists", target)
    return fmt.Errorf("target %s exists", target)
} else if !os.IsNotExist(statErr) {
    return statErr
}
```

### WR-03: resolvers.query mutates shared ring head under RLock across concurrent queries

**File:** `internal/dns/resolvers.go:90-130`
**Issue:** `query()` takes `rs.m.RLock()` but mutates `rs.rs = rs.rs.Next()` while iterating failover candidates (line ~124). Multiple concurrent DNS queries each advance the same shared `ring.Ring` cursor independently under read lock, so rotation state is racy: one query's "all servers failed" termination check (`head == rs.rs`) can be skipped or hit early because another goroutine moved the ring between comparisons. This can cause spurious `All DNS Servers didn't respond` errors or a resolver marked failed even though a later probe in the same loop succeeded.
**Fix:** Make the ring per-call (copy the ordered list under RLock into a local slice) or take a full Lock for the failover walk. Minimal fix:
```go
rs.m.RLock()
n := rs.rs.Len()
cands := make([]*resolver, 0, n)
for i, cur := 0, rs.rs; i < n; i, cur = i+1, cur.Next() {
    cands = append(cands, cur.Value.(*resolver))
}
rs.m.RUnlock()
```
then iterate `cands` locally (with wrap-around back to index 0 for the all-failed check).

### WR-04: fswatcher loop swallows dns.Load failure detail and can hot-loop on transient failures

**File:** `internal/fswatcher/loop.go:25-29`
**Issue:** On Create/Write events, `dns.Load(w.cfg.Dns.List.File)` errors are logged without message context and the event is simply dropped. Worse, editors commonly write by rename/recreate (tmp file + move), which produces a sequence of Write events for the *new* inode while the watch is still on the old path — every such event re-attempts loading a possibly half-written or already-replaced file, logging repeated errors, and never re-adds the watch. Since watcher restart-on-rotation was not part of this phase, at minimum the load error should include the path and the handler should tolerate ENOENT/invalid-partial-file states explicitly.
**Fix:**
```go
if e := dns.Load(w.cfg.Dns.List.File); e != nil {
    w.L().Error().Err(e).Msgf("failed to reload domainlist %s", w.cfg.Dns.List.File)
}
```
(file already has the path field available.) A follow-up (not required this phase): track watched inode and re-establish watch on rotation.

### WR-05: cache.load() partial-registration failure leaves mux swapped inconsistently with eviction

**File:** `internal/dns/cache.go:215-237`
**Issue:** In `load()`, plain (non-regex) line registration errors abort the whole function with `return e` — but this happens *after* new generations' entries for earlier lines have already been registered on `tempMux`, which is then discarded, meaning previously-valid domains stay in the live mux (correct) but the generation counter has *already* been incremented. Subsequent reloads therefore evict keys whose gen <= the bumped value even though nothing matching was loaded — a silent inconsistency between generation bookkeeping and actual registrations when a malformed domainline coexists with valid ones mid-file. Regex errors correctly skip-and-continue; plain-domain errors should match that behavior for consistency (or validate the whole file before swapping).
**Fix:** mirror the regex branch:
```go
if e = c.registerOn(line, tempMux, false); e != nil {
    c.L().Warn().Msgf("Skipping invalid line %q: %v", line, e)
    continue
}
```

## Info

### IN-01: Global variables retained intentionally but documented only in comments

**File:** `cmd/bgp-dnsd/cli/cache.go:36-39`, `internal/dns/main.go:31`, `internal/bgp/main.go:36`
**Issue:** Package-level globals (`_cancel`, `_listener`, `_listFile`, `_dns`, `_bgp`, `_watcher`) are kept as Phase-2 backward-compat shims. They defeat the DI goal for anything reached through them (`dns.Load`, `dns.DumpCache`, `bgp.Advance/Withdraw`) and make concurrent-service testing fragile (tests must serialize on `_bgp`). Acceptable as a transitional measure, but the retirement path isn't tracked.
**Fix:** Add a follow-up work item to route `cache.onEntryEvicted`/`upsert` calls through the injected service reference instead of `bgp.Withdraw`/`bgp.Advance` package functions, then delete the globals.

### IN-02: Duplicate logger parameter naming (`l` vs `l2`) in newCache signature

**File:** `internal/dns/cache.go:44`
**Issue:** `newCache(max int, minTtl time.Duration, rs *resolvers, l *zerolog.Logger, cfg *config.AppCfg, l2 loop.Loop)` uses `l` for the zerolog logger and `l2` for the loop — easy to transpose at call sites; there are multiple callers (main.go, tests).
**Fix:** Rename params to `logger` and `l` (loop), consistent with sibling constructors `NewDns(cfg, l, logger)`.

### IN-03: Test helper exported API surface growing in production packages

**File:** `internal/bgp/main.go:206-230`
**Issue:** `SetBgpForTest`, `NewBgpSrvForTest`, `GetBgpRefCounter` plus `internal/config/test.go` (`TestConfig`) add test-only export surface to non-_test source files. Workable, but consider moving to an `internal/bgp/testutil` subpackage (importable only by tests via Go tooling conventions) or build-tagged file.
**Fix:** Optional; track as tech debt.

### IN-04: main.go startup order starts BGP/CLI/DNS/fswatcher before first domainlist Load; Load errors after services started exit cleanly but leave partial state window

**File:** `cmd/bgp-dnsd/main.go:88-97`
**Issue:** `dns.Load(cfg.Dns.List.File)` runs after all four services are constructed and deferred shutdowns registered. Failure here exits before "Startup complete", which is fine, but between `NewDns` succeeding and `Load` failing, the DNS server (if wired) could proxy queries with an empty cache. Purely ordering/cosmetic given deferred cleanups work.
**Fix:** Optionally move `dns.Load` ahead of `NewFsWatcher` construction so the watcher only watches a successfully-loaded file; or start the fswatcher last as currently intended.

---

_Reviewed: 2026-08-17T00:00:00Z_
_Reviewer: opencode (gsd-code-review role, executed inline)_
_Depth: standard_
