---
status: complete
phase: 05-security
source: [05-01-SUMMARY.md, 05-02-SUMMARY.md, 05-03-SUMMARY.md]
started: 2026-08-20T03:47:54Z
updated: 2026-08-20T04:05:24Z
completed: 2026-08-20T04:05:24Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

number: null
name: null
expected: |
  All 7 tests complete.
awaiting: none

## Tests

### 1. Unix admin socket forced to 0600
expected: TestServe_UnixSocketMode0600 (fresh + stale subtests) passes — socket perm bits exactly 0o600 in both cases, verified via the production Serve() path
result: pass
verified_at: 2026-08-20T03:47Z (orchestrator ran target suite: fresh + stale subtests PASS)
requirement: SEC-01
coverage_id: D1

### 2. Nil request/stream rejected on all three RPCs
expected: Direct method calls with nil request/stream return codes.InvalidArgument — TestGRPC_ClearCache_NilRequest, TestGRPC_ListCacheEntries_NilStream, TestGRPC_ReloadList_NilRequest all pass
result: pass
verified_at: 2026-08-20T03:47Z (orchestrator ran target suite: all 3 nil-rejection tests PASS)
requirement: SEC-02
coverage_id: D2

### 3. Valid non-nil requests traverse the audit gate
expected: A valid empty-but-non-nil &api.ClearCacheRequest{} traverses the interceptor chain and reaches handler business logic — TestGRPC_ServeShutdown returns exactly codes.FailedPrecondition (proving it got past the gate), never InvalidArgument
result: pass
verified_at: 2026-08-20T03:47Z (orchestrator ran target suite: TestGRPC_ServeShutdown PASS, FailedPrecondition = gate passthrough)
requirement: SEC-02
coverage_id: D3

### 4. Listed domains answer only A/AAAA/HTTPS
expected: TestServedQTypes — A, AAAA, HTTPS queries for listed domains resolve successfully; TestDeniedQTypeRefused — MX, TXT, CNAME, NS, PTR receive RcodeRefused(5) with empty Answer and zero upstream queries (per-qtype counted fake)
result: pass
verified_at: 2026-08-20T03:47Z (orchestrator ran internal/dns suite under -race: TestServedQTypes + TestDeniedQTypeRefused PASS)
requirement: SEC-03

### 5. Unlisted domains forward unchanged
expected: TestCatchAllForward — an unlisted domain's MX query is transparently proxied through the production catch-all wiring (no REFUSED); TestNXDomainPassthrough — allowed qtypes still hit upstream and any written reply carries the upstream rcode verbatim (guard never rewrites rcodes)
result: pass
verified_at: 2026-08-20T03:47Z (orchestrator ran internal/dns suite under -race: TestCatchAllForward + TestNXDomainPassthrough PASS)
requirement: SEC-03

### 6. Optional BGP peer AuthPassword parsing + propagation
expected: TestInit_BgpAuthPassword — absent→"", present→exact round-trip, explicit-empty→"" (all pass). TestBuildPeerSpec_AuthPassword — password propagated into Peer.Conf.AuthPassword ("k-123"), unset stays empty, plus guard-rail identity asserts (NeighborAddress, PeerAsn, Transport.LocalAddress, EbgpMultihop.Enabled)
result: pass
verified_at: 2026-08-20T03:47Z (orchestrator ran config+bgp suites under -race: TestInit_BgpAuthPassword + TestBuildPeerSpec_AuthPassword PASS)
requirement: SEC-04

### 7. No credential ever logged
expected: grep over zerolog call sites touching AuthPassword yields 0 matches across internal/config and internal/bgp; README documents the never-logged guarantee and chmod-600 discipline for secret-bearing configs
result: pass
verified_at: 2026-08-20T04:05Z (orchestrator re-grep: zero log statements touch AuthPassword; README L26 "The daemon never logs credential values")
requirement: SEC-04

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0

## Gaps

[none yet]
