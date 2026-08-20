# Phase 02: Test Suite - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-14
**Phase:** 02-test-suite
**Areas discussed:** BGP reference counting tests, resolver failover tests, end-to-end test scope, test organization

---

## BGP Reference Counting Tests

| Option | Description | Selected |
|--------|-------------|----------|
| Reference counting only | Test ipRefCounter transitions, bypass GoBGP calls | |
| Integration with real GoBGP | Spin up GoBGP server in subprocess, requires root/NET_ADMIN | |
| Interface abstraction | Create bgpOp interface, mock it in tests (precursor to Phase 3) | |
| **Spy on operations** | Intercept loop.Operation() calls, assert ref counter state | ✓ |
| **Mock the bgpSrv** | Extract add()/remove() into interface, use mock | |
| **Test public API only** | Call Advance()/Withdraw() on real bgpSrv with fake GoBGP | |

**User's choice:** Spy on operations — test reference counting logic in isolation without calling real GoBGP add()/remove().

**Notes:**
- 5 scenarios selected: single IP advance/withdraw, multi-domain IP sharing, concurrent advances, withdraw non-existent IP, mixed sequence
- Both counter state AND add()/remove() call timing verified
- Sync wait via Operation(f, true) — test blocks until goroutine completes
- Tests in same-package `internal/bgp/bgp_test.go`

---

## Resolver Failover Tests

| Option | Description | Selected |
|--------|-------------|----------|
| DNS fake server | Spin up local dns.Server with controlled responses | ✓ |
| Interface extraction | Wrap dns.Exchange behind interface, inject mock | |
| nslookup-style mock | Share one server across tests with configurable responses | |
| **4 scenarios** | Single success, failover, recovery, all-fail | ✓ |
| 5 scenarios | Plus empty resolver list | |
| **Both** | Test observable behavior AND ring position | ✓ |
| Observable only | Verify which resolver used and final okay state | |
| **query() only** | Only test query(), skip proxyQuery | ✓ |
| Full path | Test proxyQuery end-to-end too | |

**User's choice:** DNS fake server with custom HandlerFunc, 4 scenarios, test both behavior and ring position, query() only.

---

## End-to-End Test Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Full integration | Real DNS + cache + BGP server, requires root | |
| Partial integration | DNS + cache real, mock BGP | |
| **Chain-of-responsibility** | Skip DNS server, test cache.resolve() directly | ✓ |
| **Verify Advance() calls** | Just check Advance() called with right IPs | ✓ |
| Verify full BGP path | Check AS path, next hop, prefix length | |
| **Shared test server** | One DNS server, reset state between tests | ✓ |
| Random ports | Each test binds to :0 | |
| **Skip DNS server** | Test cache.resolve() directly with mock ResponseWriter | |

**User's choice:** Chain-of-responsibility approach — skip DNS server, test cache.resolve() directly with mock ResponseWriter, verify Advance() called with correct IPs.

---

## Test Organization

| Option | Description | Selected |
|--------|-------------|----------|
| **Same-package** | internal/bgp/bgp_test.go, access private fields | ✓ |
| Separate test package | Black-box tests of public API | |
| Both | Same-package + separate package | |
| **Embedded strings** | strings.NewReader inline, no filesystem | ✓ |
| Temp files | t.TempDir() + os.WriteFile | |
| Both | Embedded for simple, temp files for complex | |
| Yes | Run all tests with -race | |
| No | Race detection is Phase 3 concern | |
| **Partial** | Race detector on BGP and loop tests only | ✓ |
| **Full lifecycle** | Start server, call all RPCs, verify, shutdown | ✓ |
| Unix socket only | Test CLI connects via Unix socket | |
| Streaming only | Test ListCacheEntries server streaming | |

---

## the agent's Discretion

- Test file naming convention (bgp_test.go vs reference_counting_test.go)
- Mock ResponseWriter implementation details
- Logger initialization pattern consistency
- Test helper function placement

## Deferred Ideas

- Test coverage metrics and CI thresholds
- GoBGP test harness integration
- Property-based testing
- Test fixture JSON files
- CI integration for test execution
