---
phase: 02-test-suite
verified: 2026-06-15T04:30:00Z
status: passed
score: 5/5 success criteria verified, 24/24 Phase 2 tests pass, race detector clean, build succeeds

gaps: []
---

# Phase 02: Test Suite Verification Report

**Phase Goal:** Add unit and integration tests for critical untested packages (BGP, DNS resolvers, cache, gRPC).

**Verified:** 2026-06-15T04:30:00Z
**Status:** PASSED — All 5 ROADMAP success criteria met, all 24 Phase 2 tests pass, race detector clean, build succeeds, all 5 requirements satisfied.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `internal/bgp/` has tests covering reference counting (advance/withdraw transitions, multi-domain IP sharing) | ✓ VERIFIED | `internal/bgp/bgp_test.go` — 6 test functions: `TestBgp_ReferenceCounting_SingleIP`, `MultiDomainSharing`, `ConcurrentAdvances`, `WithdrawNonExistent`, `MixedSequence`, `AddRemove_NilSafe`. All pass. Uses `Operation(f, true)` for sync execution (11 Operation calls verified). |
| 2 | `internal/dns/resolvers.go` has tests for ring failover, health tracking, round-robin selection | ✓ VERIFIED | `internal/dns/resolvers_test.go` — 4 test functions: `TestResolver_SingleSuccess`, `Failover`, `Recovery`, `AllFail`. Uses `newFakeDNSServer` helper to create controlled DNS responses. Ring rotation and health state verified via `rs.rs.Do()` iteration. |
| 3 | `internal/dns/cache.go` has tests for generation-based eviction, TTL expiration, capacity limits | ✓ VERIFIED | `internal/dns/cache_eviction_test.go` — 5 test functions: `TestCache_GenerationEviction`, `TTLExpiration`, `CapacityLimit`, `EvictionTriggersWithdraw`, `MultiDomainEviction`. Generation-based eviction verified through mux registration state. LFU capacity limit verified via `c.entries.GetALL(true)`. |
| 4 | `cmd/bgp-dnsd/cli/` has integration tests for gRPC Serve/Shutdown lifecycle | ✓ VERIFIED | `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` — 6 test functions: `TestGRPC_ServeShutdown`, `ListCacheEntries_Uninitialized`, `ClearCache_Uninitialized`, `ReloadList_NilRequest`, `ReloadList_Uninitialized`, `GracefulStop`. Error codes verified: `FailedPrecondition` for uninitialized cache, `InvalidArgument` for nil requests. |
| 5 | End-to-end test: DNS query → cache hit → BGP announcement verification | ✓ VERIFIED | `internal/dns/dns_e2e_test.go` — 3 test functions: `TestE2E_DnsQueryToCacheToBgp`, `MultiDomainIPSharing`, `CacheHitReturnsAnswer`. Uses `bgp.NewBgpSrvForTest()`, `bgp.SetBgpForTest()`, `bgp.GetBgpRefCounter()` helpers. Verifies `ipRefCounter` state after DNS queries trigger BGP advancement. |

**Score: 5/5 truths verified**

### Deferred Items

None. All success criteria are met.

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/bgp/bgp_test.go` | 6 BGP reference counting tests | ✓ VERIFIED | 353 lines, 6 test functions + `TestMain`. Uses `newTestBgpSrv()` helper, `testHooks` for add/remove recording, `Operation(f, true)` for sync execution. |
| `internal/dns/resolvers_test.go` | 4 DNS resolver failover tests | ✓ VERIFIED | 288 lines, 4 test functions + helpers (`newFakeDNSServer`, `startFakeServer`, `makeSuccessMsg`, `makeErrorResponse`). Fresh resolver instance per test. |
| `internal/dns/cache_eviction_test.go` | 5 cache eviction tests | ✓ VERIFIED | 186 lines, 5 test functions. Uses `newTestCache()` from `cache_test.go`. Generation-based eviction verified through mux state. |
| `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` | 6 gRPC lifecycle tests | ✓ VERIFIED | 165 lines, 6 test functions + `newTestGRPCLifecycle()` helper. Ephemeral port via `net.Listen("tcp", "127.0.0.1:0")`. gRPC error codes verified via `status.FromError()`. |
| `internal/dns/dns_e2e_test.go` | 3 E2E integration tests | ✓ VERIFIED | 248 lines, 3 test functions + helpers (`fakeDNSServerForE2E`, `setupE2EEnv`). Cross-package BGP state inspection via exported test helpers. |
| `internal/config/test.go` | TestConfig() helper | ✓ VERIFIED | 15 lines. Returns minimal `*config.AppCfg` for cache serve loop. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| `bgp_test.go` | `bgp/main.go` | `Operation(f, true)` spy | ✓ WIRED | 11 `Operation(func() error { ... }, true)` calls in `bgp_test.go`. Direct `bgpSrv` construction with `loop.NewLoop(1)`. |
| `bgp_test.go` | `bgp/main.go` | `ipRefCounter` map inspection | ✓ WIRED | Tests directly access `srv.ipRefCounter["10.0.0.1"]` (same-package test). Counter state verified after each Advance/Withdraw. |
| `resolvers_test.go` | `resolvers.go` | `dns.Exchange()` via fake server | ✓ WIRED | `newFakeDNSServer()` creates real DNS server on random UDP port. `rs.query()` calls `dns.Exchange()` which hits fake server. |
| `cache_eviction_test.go` | `cache.go` | `load()` → `evictByGeneration()` | ✓ WIRED | Tests call `c.load()` with temp files, verify `c.mux.exact` entries removed after generation increment. |
| `grpc_lifecycle_test.go` | `cli/cache.go` | Ephemeral port + gRPC server | ✓ WIRED | `net.Listen("tcp", "127.0.0.1:0")` → `grpc.NewServer()` → `RegisterBgpDnsServiceServer()`. Client connects via `grpc.Dial()`. |
| `dns_e2e_test.go` | `dns/cache.go` | `cache.resolve()` with mock writer | ✓ WIRED | Tests construct `dns.Msg` and call `c.resolve(w, msg, false)` directly. `testResponseWriter` captures response. |
| `dns_e2e_test.go` | `bgp/main.go` | `bgp.Advance()` via `_bgp` global | ✓ WIRED | `bgp.SetBgpForTest(srv)` sets global, `bgp.GetBgpRefCounter()` reads `ipRefCounter`. Reference count verified after `resolve()`. |
| `bgp.go` (source) | `add()`/`remove()` | Nil-safety guard | ✓ WIRED | `bgp.go:41` `if s.bgp == nil { return nil }` in `add()`, `bgp.go:98` same guard in `remove()`. |
| `cli/cache.go` (source) | `ListCacheEntries` | `FailedPrecondition` status | ✓ WIRED | `cli/cache.go:97` `status.Errorf(codes.FailedPrecondition, ...)` for `dns.ENotInitialized`. |
| `cache.go` (source) | `onEntryEvicted` | `cacheKey` cast fix | ✓ WIRED | `cache.go:52` `ck := k.(cacheKey)` — cast to struct, not string. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `bgp_test.go` | `srv.ipRefCounter` | `Operation()` closures | Real atomic counter state | ✓ FLOWING |
| `resolvers_test.go` | `fakeDNSServer` handler | `net.ListenPacket("udp", ...)` | Real DNS responses (success/error) | ✓ FLOWING |
| `cache_eviction_test.go` | `c.mux.exact` | `c.load()` → `c.registerOn()` | Real mux registration state | ✓ FLOWING |
| `grpc_lifecycle_test.go` | `grpc.Server` | `net.Listen("tcp", "127.0.0.1:0")` | Real gRPC server accepting connections | ✓ FLOWING |
| `dns_e2e_test.go` | `cache.resolve()` | `fakeDNSServerForE2E()` + `c.serve()` | Full DNS→cache→BGP data flow | ✓ FLOWING |
| `cacheEntry.go` | `ttl`, `expiration` | `updateTtl()` / atomic reads in `loop()` | Race-free concurrent access | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| All Phase 2 tests pass | `go test ./internal/bgp/ ./internal/dns/ ./cmd/bgp-dnsd/cli/ -count=1` | 63/63 PASS | ✓ PASS |
| Race detector clean (BGP) | `go test ./internal/bgp/ -race -count=1` | ok (1.036s) | ✓ PASS |
| Race detector clean (DNS) | `go test ./internal/dns/ -race -count=1` | ok (1.081s) × 5 runs | ✓ PASS |
| Race detector clean (CLI) | `go test ./cmd/bgp-dnsd/cli/ -race -count=1` | ok (1.072s) | ✓ PASS |
| Full workspace race detector | `go test ./... -race -count=1` | all ok | ✓ PASS |
| Full workspace build | `go build ./...` | clean | ✓ PASS |
| No t.Parallel() in Phase 2 tests | `grep -rn 't\.Parallel()' internal/bgp/bgp_test.go internal/dns/resolvers_test.go internal/dns/cache_eviction_test.go cmd/bgp-dnsd/cli/grpc_lifecycle_test.go internal/dns/dns_e2e_test.go` | 0 matches | ✓ PASS |
| No stub/placeholder code | `grep -rn 'TODO\|FIXME\|XXX\|placeholder\|coming soon' internal/bgp/bgp_test.go internal/dns/resolvers_test.go internal/dns/cache_eviction_test.go cmd/bgp-dnsd/cli/grpc_lifecycle_test.go internal/dns/dns_e2e_test.go` | 0 matches | ✓ PASS |
| No `MustCompile` usage | `grep -rn 'MustCompile' internal/bgp/bgp_test.go internal/dns/resolvers_test.go internal/dns/cache_eviction_test.go cmd/bgp-dnsd/cli/grpc_lifecycle_test.go internal/dns/dns_e2e_test.go` | 0 matches | ✓ PASS |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
| ----------- | ----------- | ------ | -------- |
| **TEST-01** | BGP reference counting unit tests | ✓ SATISFIED | `internal/bgp/bgp_test.go` — 6 tests covering SingleIP, MultiDomainSharing, ConcurrentAdvances, WithdrawNonExistent, MixedSequence, AddRemove_NilSafe. Nil-safety guards in `bgp.go:41,98`. |
| **TEST-02** | DNS resolver ring failover unit tests | ✓ SATISFIED | `internal/dns/resolvers_test.go` — 4 tests covering SingleSuccess, Failover, Recovery, AllFail. Ring position verified via `rs.rs.Do()` iteration. |
| **TEST-03** | Cache eviction on generation change | ✓ SATISFIED | `internal/dns/cache_eviction_test.go` — 5 tests covering GenerationEviction, TTLExpiration, CapacityLimit, EvictionTriggersWithdraw, MultiDomainEviction. |
| **TEST-04** | gRPC server lifecycle integration tests | ✓ SATISFIED | `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` — 6 tests covering ServeShutdown, ListCacheEntries_Uninitialized, ClearCache_Uninitialized, ReloadList_NilRequest, ReloadList_Uninitialized, GracefulStop. |
| **TEST-05** | End-to-end DNS→cache→BGP integration test | ✓ SATISFIED | `internal/dns/dns_e2e_test.go` — 3 tests covering DnsQueryToCacheToBgp, MultiDomainIPSharing, CacheHitReturnsAnswer. Cross-package BGP state via exported test helpers. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| *(none)* | — | TBD/FIXME/XXX | — | No debt markers in Phase 2 files |
| *(none)* | — | placeholder/coming soon | — | No stub patterns found |
| *(none)* | — | t.Parallel() | — | No parallel tests (global state concern) |
| *(none)* | — | MustCompile | — | No unsafe regex compilation |

### File Path Note

The PLAN frontmatter for 02-03 references `internal/dns_e2e_test.go` (underscore, package `internal`) but the actual file is `internal/dns/dns_e2e_test.go` (package `dns`). This is consistent with the SUMMARY.md which correctly lists `internal/dns/dns_e2e_test.go`. The test works correctly because:
1. Same-package access to `cache`, `resolvers`, `regexServeMux` internals (package `dns`)
2. Cross-package BGP state access via exported helpers: `bgp.NewBgpSrvForTest()`, `bgp.SetBgpForTest()`, `bgp.GetBgpRefCounter()`

### Race Condition Fix (Pre-existing Bug)

During Phase 2 verification, a pre-existing data race was identified and fixed:

- **Bug:** `cacheEntry.ttl` and `cacheEntry.expiration` were non-atomic fields (`time.Duration` and `time.Time`) accessed concurrently by `cache.loop()` (reader goroutine) and `cache.upsert()` (via `updateTtl()`).
- **Impact:** E2E tests (`dns_e2e_test.go`) failed with `-race` because they trigger `resolve()` → `upsert()` while the cache serve loop is running.
- **Fix:** Changed `ttl` to `atomic.Int64` (nanoseconds) and `expiration` to `atomic.Int64` (Unix nanoseconds). Updated all accessors in `cacheEntry.go`, `loop.go`, `cache.go`, and `cacheEntry_test.go`.
- **Verification:** Race detector passes consistently across 5+ consecutive runs of `go test ./internal/dns/ -race`.

### Git Commits for Phase 2

| Plan | Commit | Message |
|------|--------|---------|
| 02-01 | `86805f0` | test(02-01): add BGP reference counting unit tests (RED) |
| 02-01 | (feat) | feat(02-01): add nil-safety to bgp add() and remove() methods |
| 02-01 | `2e4c539` | feat(02-01): add DNS resolver failover unit tests |
| 02-02 | `785543e` | test(02-02): add cache eviction tests (RED) |
| 02-02 | `28f7e53` | test(02-02): add gRPC CLI lifecycle tests (RED) |
| 02-02 | `0f5105d` | feat(02-02): implement cache eviction and gRPC lifecycle tests |
| 02-03 | `a3913fa` | test(02-03): add E2E integration tests + BGP/helpers + config helper |

## Human Verification Required

None. All verification is programmatically observable:
- All 24 Phase 2 tests pass
- Full workspace: 63 tests pass
- Race detector clean across 5+ consecutive runs
- Build succeeds
- All 5 requirements trace to code evidence
- No anti-patterns or stubs found

## Summary

**Phase 02: Test Suite — PASSED**

All 3 plans executed successfully. Phase 2 created 5 new test files with 24 test functions covering BGP reference counting (6 tests), DNS resolver failover (4 tests), cache eviction (5 tests), gRPC lifecycle (6 tests), and end-to-end DNS→cache→BGP flow (3 tests). All tests pass with the race detector. Build succeeds. All 5 requirements (TEST-01 through TEST-05) are satisfied.

Additionally, Phase 2 uncovered and fixed 3 pre-existing issues in the production code:
1. Nil-safety guards on `bgp.add()` and `bgp.remove()` (enables testing without GoBGP)
2. `cache.onEntryEvicted` panic fix (cacheKey struct cast)
3. `ListCacheEntries` gRPC error code fix (FailedPrecondition)
4. Data race fix in `cacheEntry.ttl` and `cacheEntry.expiration` (atomic types)

No `t.Parallel()` used in any Phase 2 test (avoids shared global state issues). No stubs, placeholders, or debt markers found.

---

_Verified: 2026-06-15T04:30:00Z_
_Verifier: automated verification agent_
