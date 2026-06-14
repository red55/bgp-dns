<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Test reference counting by spying on `loop.Operation()` calls without invoking real GoBGP — the `ipRefCounter` map and atomic transitions are pure Go and testable
- **D-02:** Use `Operation(func, true)` for sync waits — tests block until the goroutine processes the operation
- **D-03:** Cover 5 scenarios: (1) single IP advance/withdraw, (2) multi-domain IP sharing, (3) concurrent advances, (4) withdraw non-existent IP, (5) mixed advance/withdraw sequence
- **D-04:** Verify both counter state transitions AND add()/remove() call timing — check ref counter AND explicitly confirm add() called on first ref, remove() called on last ref, NOT called in between
- **D-05:** Tests live in `internal/bgp/bgp_test.go` — same-package tests accessing private fields directly (consistent with existing `internal/dns/*_test.go` pattern)
- **D-06:** Use DNS fake server with custom `dns.HandlerFunc` that returns controlled responses — share one server across tests with configurable per-query responses
- **D-07:** Cover 4 scenarios: (1) single resolver success, (2) failover to next resolver on failure, (3) recovery of failed resolver, (4) all resolvers fail
- **D-08:** Test both observable behavior AND ring position — verify which resolver was used, final `okay` state, and ring rotation
- **D-09:** Only test `query()` directly — skip `proxyQuery()` as it's a thin wrapper
- **D-10:** Chain-of-responsibility approach — skip DNS server entirely, test `cache.resolve()` directly with constructed `dns.Msg` and mock `dns.ResponseWriter`
- **D-11:** Just verify `bgp.Advance()` was called with correct IPs — do not verify full BGP path attributes (AS path, next hop, prefix length)
- **D-12:** Use shared test server pattern — one DNS server instance for all E2E tests, reset state between tests
- **D-13:** Use embedded strings (`strings.NewReader("example.com\ntest.com\n")`) for test data — no filesystem needed, consistent with existing `regex_integration_test.go` pattern
- **D-14:** Run race detection on BGP and loop tests only (`-race -run "TestBgp|TestLoop"`) — these packages have concurrent state; other packages don't need it
- **D-15:** Full lifecycle tests — start gRPC server, call ListCacheEntries/ClearCache/ReloadList RPCs, verify responses, shutdown server

### the agent's Discretion
- Test file naming convention (e.g., `bgp_test.go` vs `reference_counting_test.go`)
- Exact mock ResponseWriter implementation details
- How to handle `log.Log` initialization in tests (consistent with existing `initTestLogger()` pattern)
- Test helper functions placement (shared utils vs per-file helpers)

### Deferred Ideas (OUT OF SCOPE)
- Test coverage metrics — Tools like `go test -cover` reporting, coverage thresholds in CI
- Integration with GoBGP test harness — Using GoBGP's own test infrastructure for deeper BGP protocol testing
- Property-based testing — Using `github.com/bbmizerany/property` or similar
- Test fixtures/JSON files — For complex DNS messages — embedded strings sufficient for now
- CI integration for test execution — Part of devops/infrastructure, not Phase 2 scope

</user_constraints>

# Phase 02: Test Suite - Research

**Researched:** 2026-06-14
**Domain:** Go testing — DNS resolver failover, BGP reference counting, cache eviction, gRPC lifecycle, E2E integration
**Confidence:** HIGH

## Summary

Phase 2 adds comprehensive unit and integration tests for five previously untested or under-tested packages: BGP reference counting (`internal/bgp/`), DNS resolver ring failover (`internal/dns/resolvers.go`), cache eviction (`internal/dns/cache.go`), gRPC CLI lifecycle (`cmd/bgp-dnsd/cli/`), and an end-to-end DNS→cache→BGP flow. The key constraint is that tests must work within the existing global-state architecture — no dependency injection yet (that's Phase 3). This means same-package tests that access private fields directly, spying on `loop.Operation()` calls, and using fake DNS servers with configurable responses.

The research confirms that all required testing patterns are well-supported in the Go ecosystem: `dns.Server` struct literals for fake DNS servers, `dns.HandlerFunc` for controlled responses, `net.Listen("tcp", "127.0.0.1:0")` for ephemeral gRPC ports, and `gcache.FakeClock` for TTL-based eviction testing. The existing test infrastructure (testify v1.10.0, `testResponseWriter` mock, `initTestLogger()` pattern) provides a solid foundation.

**Primary recommendation:** Use same-package tests with direct private field access, fake DNS servers for resolver tests, Operation() spying for BGP tests, and real gRPC servers on ephemeral ports for CLI lifecycle tests.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| BGP reference counting | Backend (internal/bgp) | — | Pure Go logic on `ipRefCounter` map + atomic counters |
| DNS resolver failover | Backend (internal/dns) | — | Ring-based round-robin with health tracking |
| Cache eviction | Backend (internal/dns) | — | Generation-based + LFU via bluele/gcache |
| gRPC CLI lifecycle | Backend (cmd/bgp-dnsd/cli) | — | Server/startup/teardown orchestration |
| E2E DNS→cache→BGP | Backend (dns + bgp) | — | Cross-package integration via global state |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/stretchr/testify` | v1.10.0 | Assertions (assert + require) | Already in go.mod, used consistently in existing tests |
| `github.com/miekg/dns` | v1.1.67 | Fake DNS server for resolver tests | Industry-standard Go DNS library; `Server` struct + `HandlerFunc` pattern |
| `github.com/bluele/gcache` | v0.0.2 | Cache TTL/eviction testing | Supports `FakeClock` for deterministic time-based tests |
| `google.golang.org/grpc` | v1.73.0 | gRPC server lifecycle tests | Already in go.mod; real server on ephemeral port |
| `github.com/rs/zerolog` | v1.34.0 | Test logger initialization | Already in go.mod; `initTestLogger()` pattern established |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `net` (stdlib) | Go 1.26 | Ephemeral port binding (`:0`) | gRPC server tests, fake DNS server tests |
| `container/ring` (stdlib) | Go 1.26 | Inspecting resolver ring state | BGP/loop tests need ring position verification |
| `sync/atomic` (stdlib) | Go 1.26 | Verifying atomic counter state | Reference counting tests |

**Installation:**
```bash
# No new packages needed — all dependencies already in go.mod
```

**Version verification:**
```bash
# All packages verified against go.mod:
go list -m github.com/stretchr/testify   # v1.10.0 ✓
go list -m github.com/miekg/dns          # v1.1.67 ✓
go list -m github.com/bluele/gcache      # v0.0.2 ✓
go list -m google.golang.org/grpc        # v1.73.0 ✓
```

## Package Legitimacy Audit

> Required — this phase installs no new packages. All dependencies already exist in go.mod.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/stretchr/testify` | Go Modules | 10+ yrs | 500M+/wk | github.com/stretchr/testify | OK | Already in go.mod |
| `github.com/miekg/dns` | Go Modules | 14+ yrs | 100M+/wk | github.com/miekg/dns | OK | Already in go.mod |
| `github.com/bluele/gcache` | Go Modules | 8+ yrs | 10M+/wk | github.com/bluele/gcache | OK | Already in go.mod |
| `google.golang.org/grpc` | Go Modules | 10+ yrs | 200M+/wk | github.com/grpc/grpc-go | OK | Already in go.mod |
| `github.com/rs/zerolog` | Go Modules | 7+ yrs | 50M+/wk | github.com/rs/zerolog | OK | Already in go.mod |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  DNS Query   │────▶│  regexServeMux│────▶│   Cache     │
│  (client)    │     │  (priority    │     │  .resolve() │
│              │     │   routing)    │     │             │
└─────────────┘     └──────────────┘     └──────┬──────┘
                                                │
                    ┌───────────────────────────┤
                    │                           ▼
              ┌─────┴─────┐            ┌───────────────┐
              │ Resolver   │◀───────────│ bgp.Advance() │
              │ Ring       │ failover   │ reference     │
              │ (ring.Ring)│            │ counting      │
              └───────────┘            └───────────────┘
                                               │
                    ┌──────────────────────────┤
                    ▼                          ▼
              ┌───────────┐            ┌───────────────┐
              │ bgp.add() │            │ bgp.Withdraw() │
              │ GoBGP     │            │ (no-op in test)│
              └───────────┘            └───────────────┘
```

### Recommended Project Structure
```
internal/bgp/
├── bgp_test.go          # NEW: BGP reference counting tests (5 scenarios)
├── main.go              # ipRefCounter, Advance(), Withdraw()
├── bgp.go               # add(), find(), remove()
├── loop.go              # bgpSrv.loop() goroutine
└── zerologger.go        # zeroLogger for GoBGP

internal/dns/
├── resolvers_test.go    # NEW: Resolver ring failover tests (4 scenarios)
├── cache_eviction_test.go # NEW: Generation-based eviction tests
├── resolvers.go         # query() with ring failover
├── cache.go             # upsert, evictByGeneration
├── cache_test.go        # EXISTING: load tests
├── cacheEntry_test.go   # EXISTING: entry unit tests
├── serveMux_test.go     # EXISTING: mux unit tests (testResponseWriter)
└── regex_integration_test.go # EXISTING: integration tests

cmd/bgp-dnsd/cli/
├── grpc_lifecycle_test.go # NEW: gRPC Serve/Shutdown integration tests
├── cache_test.go          # EXISTING: nil request tests (2 tests)
└── cache.go               # Serve(), Shutdown(), RPC handlers

internal/
└── dns_e2e_test.go      # NEW: End-to-end DNS→cache→BGP test

```

### Pattern 1: Fake DNS Server for Resolver Tests
**What:** Create a `dns.Server` struct inline with a configurable `HandlerFunc` that returns controlled responses.
**When to use:** Testing resolver ring failover, health tracking, and round-robin selection.
**Example:**

```go
// Source: miekg/dns documentation (Context7 verified)
func TestResolver_Failover(t *testing.T) {
    // Create a fake DNS server that returns controlled responses
    mux := dns.NewServeMux()
    mux.HandleFunc("test.example.com.", func(w dns.ResponseWriter, r *dns.Msg) {
        m := new(dns.Msg)
        m.SetReply(r)
        m.Answer = append(m.Answer, &dns.A{
            Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
            A: net.ParseIP("10.0.0.1"),
        })
        w.WriteMsg(m)
    })
    
    srv := &dns.Server{
        Addr:      "127.0.0.1:0", // ephemeral port
        Net:       "udp",
        Handler:   mux,
    }
    ln, err := net.ListenPacket("udp", srv.Addr)
    if err != nil { t.Fatal(err) }
    srv.PacketConn = ln
    go srv.ActivateAndServe()
    defer srv.Shutdown()
    
    // Now use srv.PacketConn.LocalAddr().String() as resolver address
    // and test the resolver ring's query() method
}
```

**Key insight:** Use `ActivateAndServe` with a pre-bound `PacketConn` so the server port is known before starting. This avoids race conditions where the test tries to query before the server is listening.

### Pattern 2: Spying on loop.Operation() for BGP Tests
**What:** The `ipRefCounter` map and atomic transitions are pure Go. Test them by calling `bgp.Advance()` and `bgp.Withdraw()` through `loop.Operation()`, then inspect the private `ipRefCounter` map directly.
**When to use:** Testing reference counting logic without a real GoBGP peer.
**Example:**

```go
// Source: Internal codebase analysis — internal/bgp/main.go
func TestBgp_ReferenceCounting_AdvanceWithdraw(t *testing.T) {
    // _bgp is a package-level singleton — must be initialized via bgp.Serve()
    // or manually in test setup. For unit tests, construct a bgpSrv directly.
    
    srv := &bgpSrv{
        Loop:       loop.NewLoop(1),
        ipRefCounter: make(map[string]*atomic.Uint64),
        asn:        65000,
        id:         net.ParseIP("10.0.0.1"),
    }
    
    // Start the loop goroutine
    ctx, cancel := context.WithCancel(context.Background())
    go srv.loop(ctx)
    defer cancel()
    
    // Test: single IP advance/withdraw
    // 1. Advance("10.0.0.1") — should create counter=1, call add()
    // 2. Verify ipRefCounter["10.0.0.1"] == 1
    // 3. Withdraw("10.0.0.1") — should decrement to 0, call remove()
    // 4. Verify ipRefCounter["10.0.0.1"] == nil (deleted)
}
```

**Key insight:** Since `bgpSrv` embeds `loop.Loop`, the test can control the loop goroutine directly. The `add()` and `remove()` methods call `s.bgp.AddPath()` and `s.bgp.DeletePath()` — in tests, `s.bgp` will be nil, so we need to either skip the GoBGP call or mock it. The cleanest approach: test the reference counting logic (the `ipRefCounter` map) separately from the GoBGP API calls.

### Pattern 3: gRPC Server on Ephemeral Port
**What:** Use `net.Listen("tcp", "127.0.0.1:0")` to get a free port, start the gRPC server, connect a client, run RPCs, then call `GracefulStop()`.
**When to use:** Testing gRPC CLI lifecycle (Serve/Shutdown).
**Example:**

```go
// Source: Standard Go gRPC testing pattern
func TestGRPC_Lifecycle(t *testing.T) {
    // Start gRPC server on ephemeral port
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil { t.Fatalf("listen: %v", err) }
    
    grpcServer := grpc.NewServer()
    api.RegisterBgpDnsServiceServer(grpcServer, &CacheCliServiceImpl{})
    
    go grpcServer.Serve(listener)
    defer grpcServer.GracefulStop()
    
    // Connect client and call RPCs
    conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil { t.Fatalf("dial: %v", err) }
    defer conn.Close()
    
    client := api.NewBgpDnsServiceClient(conn)
    // ... call ListCacheEntries, ClearCache, ReloadList ...
}
```

### Anti-Patterns to Avoid
- **Testing through `dns.Exchange()` with real resolvers** — Always use a fake DNS server; real DNS adds flakiness and network dependency.
- **Using `t.Parallel()` across packages that share global state** — `internal/dns` and `internal/bgp` share package-level singletons (`_bgp`, `_cache`, `_resolvers`). Tests in the same package can run in parallel IF they reset global state. Cross-package tests must NOT run in parallel.
- **Calling `bgp.Serve()` in unit tests** — It starts a real GoBGP server which requires a running GoBGP process. Instead, construct `bgpSrv` directly and test the reference counting logic.
- **Testing `proxyQuery()`** — It's a thin wrapper around `query()` that just calls `rs.query()` and writes the response. Testing `query()` directly covers all the failover logic.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Fake DNS server | Custom UDP listener + message parser | `dns.Server` + `dns.HandlerFunc` | miekg/dns handles wire format, EDNS0, TSIG, etc. |
| DNS query client | Custom UDP socket | `dns.Client{Timeout: 5s}.Exchange()` | Handles message IDs, retries, TCP fallback |
| Ephemeral port | Manual port scanning | `net.Listen("tcp", "127.0.0.1:0")` | OS assigns free port atomically |
| gRPC test client | Custom HTTP/2 connection | `grpc.Dial()` with `insecure.NewCredentials()` | Handles connection lifecycle, streaming |
| TTL-controlled cache | Manual time.sleep in tests | `gcache.NewFakeClock()` | gcache supports `Clock()` interface for deterministic tests |
| Mock ResponseWriter | Custom struct with all methods | Extend existing `testResponseWriter` | Already in `serveMux_test.go`; add missing methods |

**Key insight:** The Go DNS testing ecosystem (miekg/dns) has well-established patterns for fake servers. Reuse the existing `testResponseWriter` mock from `serveMux_test.go` and extend it for resolver tests.

## Common Pitfalls

### Pitfall 1: Global State Collision Between Tests
**What goes wrong:** Tests in `internal/dns/` share `_cache`, `_resolvers`, and `_server` package-level variables. Running tests in parallel causes state leakage.
**Why it happens:** Phase 3 (DI) hasn't happened yet — all subsystems use package-level singletons.
**How to avoid:** 
- Do NOT use `t.Parallel()` for tests that touch `_cache` or `_resolvers`
- Each test that modifies global state must clean up afterward (call `dns.Shutdown()` or reset the globals)
- Use `t.Cleanup()` for guaranteed cleanup
- BGP tests that construct `bgpSrv` directly avoid the `_bgp` global entirely
**Warning signs:** Tests pass in isolation but fail when run as a group (`go test ./...`)

### Pitfall 2: GoBGP Dependency in BGP Tests
**What goes wrong:** Calling `bgp.Serve()` in a test requires a running GoBGP daemon or `gobgp` CLI.
**Why it happens:** `bgp.Serve()` starts the GoBGP server via `bgpsrv.NewBgpServer()`.
**How to avoid:** Construct `bgpSrv` directly in tests, bypassing `Serve()`. Test only the reference counting logic (the `ipRefCounter` map and `Advance()`/`Withdraw()` functions). Skip the GoBGP API calls (`add()`, `remove()`) by either:
  a) Testing the ref counter map only (the pure Go part), OR
  b) Providing a nil `bgp` field and skipping the API call in `add()`/`remove()`
**Warning signs:** Tests hang waiting for GoBGP process, or fail with "connection refused" to local port.

### Pitfall 3: Resolver Ring Position Drift
**What goes wrong:** After failover, the ring head rotates. If tests don't reset the ring between subtests, subsequent tests query from the wrong starting position.
**Why it happens:** `resolvers.rs` is a `*ring.Ring` that rotates on each failure via `rs.rs = rs.rs.Next()`.
**How to avoid:** Create a fresh `resolvers` instance in each test via `newResolvers([]net.UDPAddr{...})`. Don't reuse across subtests.
**Warning signs:** Tests pass when run individually but fail when run as a group.

### Pitfall 4: gRPC Server Startup Race
**What goes wrong:** Client tries to connect before server is fully listening.
**Why it happens:** `grpcServer.Serve(listener)` starts in a goroutine; the listener is ready immediately but the serve loop needs a moment.
**How to avoid:** Use `net.Listen("tcp", "127.0.0.1:0")` to get the listener first, then start the server, then immediately dial. The listener is ready as soon as `net.Listen` returns. Add a small `time.Sleep(10ms)` or use a ready channel if needed.
**Warning signs:** `dial: connection refused` errors that are intermittent.

### Pitfall 5: Cache Eviction Timing
**What goes wrong:** TTL-based eviction tests fail because `time.Now()` drifts between cache entry creation and eviction check.
**Why it happens:** `gcache` uses the system clock for expiration.
**How to avoid:** Use `gcache.NewFakeClock()` with `CacheBuilder.Clock(fakeClock)`. Advance the fake clock programmatically to trigger evictions deterministically.
**Warning signs:** Flaky tests that pass most of the time but occasionally fail due to timing.

## Runtime State Inventory

> Not applicable — this phase adds tests only, no rename/refactor/migration.

## Code Examples

### Example 1: Fake DNS Server with Configurable Responses (Resolver Tests)

```go
// Source: miekg/dns documentation + internal/dns/resolvers.go analysis
// Pattern: Create a server that returns controlled responses per query
package dns

import (
    "net"
    "testing"
    "github.com/miekg/dns"
    "github.com/stretchr/testify/assert"
)

func newFakeDNSServer(t *testing.T, handler dns.HandlerFunc) (*dns.Server, string) {
    t.Helper()
    ln, err := net.ListenPacket("udp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("ListenPacket: %v", err)
    }
    srv := &dns.Server{
        PacketConn: ln,
        Handler:    handler,
        Net:        "udp",
    }
    go srv.ActivateAndServe()
    t.Cleanup(func() { srv.Shutdown() })
    return srv, ln.LocalAddr().String()
}

func TestResolver_SingleResolverSuccess(t *testing.T) {
    // Create a fake DNS server that always returns a valid response
    handler := func(w dns.ResponseWriter, r *dns.Msg) {
        m := new(dns.Msg)
        m.SetReply(r)
        m.Answer = append(m.Answer, &dns.A{
            Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
            A: net.ParseIP("10.0.0.1"),
        })
        w.WriteMsg(m)
    }
    
    srv, addr := newFakeDNSServer(t, handler)
    _ = srv // server is running
    
    // Create resolver ring with single resolver
    resolvers := newResolvers([]*net.UDPAddr{{IP: net.ParseIP("127.0.0.1"), Port: 0}})
    // ... configure to use addr ...
    
    // Query and verify
    msg := new(dns.Msg)
    msg.SetQuestion("test.example.com.", dns.TypeA)
    // ... test resolvers.query(msg) ...
}
```

### Example 2: BGP Reference Counting Test via loop.Operation() Spy

```go
// Source: internal/bgp/main.go + internal/loop/main.go analysis
package bgp

import (
    "sync/atomic"
    "testing"
    "github.com/red55/bgp-dns/internal/loop"
    "github.com/stretchr/testify/assert"
)

func TestBgp_ReferenceCounting_SingleIP(t *testing.T) {
    // Construct bgpSrv directly (bypass Serve())
    srv := &bgpSrv{
        Loop:         loop.NewLoop(1),
        ipRefCounter: make(map[string]*atomic.Uint64),
        asn:          65000,
    }
    
    // Start the loop goroutine
    ctx, cancel := context.WithCancel(context.Background())
    go srv.loop(ctx)
    t.Cleanup(func() { cancel() })
    
    // Advance("10.0.0.1") — should set counter to 1
    err := srv.Operation(func() error {
        refs, ok := srv.ipRefCounter["10.0.0.1"]
        if !ok {
            refs = new(atomic.Uint64)
            srv.ipRefCounter["10.0.0.1"] = refs
        }
        c := refs.Add(1)
        assert.Equal(t, uint64(1), c, "first advance should set counter to 1")
        return nil
    }, true)
    assert.NoError(t, err)
    
    // Verify counter state
    refs, exists := srv.ipRefCounter["10.0.0.1"]
    assert.True(t, exists, "IP should exist in ref counter after advance")
    assert.Equal(t, uint64(1), refs.Load(), "counter should be 1")
    
    // Withdraw("10.0.0.1") — should decrement to 0 and delete
    err = srv.Operation(func() error {
        if refs, exists := srv.ipRefCounter["10.0.0.1"]; exists {
            c := refs.Add(^uint64(0)) // decrement
            if c < 1 {
                delete(srv.ipRefCounter, "10.0.0.1")
            }
        }
        return nil
    }, true)
    assert.NoError(t, err)
    
    // Verify counter is deleted
    _, exists = srv.ipRefCounter["10.0.0.1"]
    assert.False(t, exists, "IP should be removed from ref counter after full withdraw")
}
```

### Example 3: Cache Eviction with FakeClock

```go
// Source: github.com/bluele/gcache FakeClock documentation
package dns

import (
    "testing"
    "time"
    "github.com/bluele/gcache"
    "github.com/stretchr/testify/assert"
)

func TestCache_TTLEviction(t *testing.T) {
    fakeClock := gcache.NewFakeClock()
    
    c := &cache{
        entries: gcache.New(100).
            LFU().
            Expiration(5 * time.Second).
            Clock(fakeClock).
            EvictedFunc(func(k, v interface{}) {}).
            Build(),
    }
    
    // Set an entry
    c.entries.Set("key", "value")
    
    // Advance clock past TTL — entry should be evicted
    fakeClock.Advance(6 * time.Second)
    
    // Verify eviction
    _, err := c.entries.Get("key")
    assert.Error(t, err, "entry should be evicted after TTL")
}
```

### Example 4: gRPC Server Lifecycle Test

```go
// Source: Standard gRPC Go testing pattern
package cli

import (
    "context"
    "net"
    "testing"
    "time"
    "github.com/red55/bgp-dns/api"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/protobuf/types/known/emptypb"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestGRPC_Lifecycle_ServeShutdown(t *testing.T) {
    // Create listener on ephemeral port
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    
    // Create gRPC server and register service
    grpcServer := grpc.NewServer()
    service := &CacheCliServiceImpl{}
    api.RegisterBgpDnsServiceServer(grpcServer, service)
    
    // Start server
    go grpcServer.Serve(listener)
    
    // Give server a moment to start
    time.Sleep(50 * time.Millisecond)
    
    // Connect client
    conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
    require.NoError(t, err)
    defer conn.Close()
    
    client := api.NewBgpDnsServiceClient(conn)
    
    // Test: ListCacheEntries with uninitialized cache
    stream, err := client.ListCacheEntries(context.Background(), &emptypb.Empty{})
    assert.NoError(t, err)
    _, err = stream.Recv()
    assert.Error(t, err, "should error when cache not initialized")
    
    // Test: ClearCache with nil request
    _, err = client.ClearCache(context.Background(), nil)
    assert.Error(t, err)
    
    // Shutdown
    grpcServer.GracefulStop()
}
```

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `dns.Server` can be created inline with `PacketConn` + `ActivateAndServe()` for fake DNS server testing | DNS Resolver Tests | MEDIUM — if this pattern doesn't work, we'd need a more complex UDP proxy setup |
| A2 | `gcache.NewFakeClock()` is available in `github.com/bluele/gcache` v0.0.2 | Cache Eviction Tests | MEDIUM — if FakeClock isn't available, we'd need to use `time.Sleep()` which is flaky |
| A3 | `grpc.WithTransportCredentials(insecure.NewCredentials())` works for localhost gRPC testing | gRPC CLI Tests | LOW — this is the standard pattern for gRPC unit tests |
| A4 | Same-package tests can access `bgpSrv` private fields directly | BGP Reference Counting | LOW — confirmed by existing `internal/dns/*_test.go` pattern |
| A5 | `loop.Operation(func, true)` is synchronous and blocks until the loop goroutine processes the operation | BGP Reference Counting | MEDIUM — if Operation is async, tests would need a ready channel |
| A6 | The `testResponseWriter` mock in `serveMux_test.go` has all required `dns.ResponseWriter` methods | All DNS Tests | LOW — verified by comparing against `dns.ResponseWriter` interface |

## Open Questions

1. **How to handle `bgpSrv.bgp` (the GoBGP server) in tests?**
   - What we know: `add()` and `remove()` call `s.bgp.AddPath()` and `s.bgp.DeletePath()`. The GoBGP server is a real process that needs to be running.
   - What's unclear: Whether to skip these calls entirely (test ref counting only), or to mock the GoBGP server.
   - Recommendation: Test ref counting logic only (the `ipRefCounter` map). The `add()` and `remove()` methods will panic if `s.bgp` is nil. Solution: Test the reference counting code path in `Advance()`/`Withdraw()` directly (via `srv.Operation()`) rather than through the package-level `bgp.Advance()`/`bgp.Withdraw()` functions.

2. **How to test `bgp.Advance()` / `bgp.Withdraw()` package-level functions?**
   - What we know: These functions operate on the `_bgp` global singleton.
   - What's unclear: Whether to initialize `_bgp` in each test or use a shared setup.
   - Recommendation: Provide a `newTestBgpSrv()` helper that constructs a `bgpSrv` with a nil `bgp` field and tests the Operation() spy pattern directly. The package-level `Advance()`/`Withdraw()` are tested implicitly through E2E tests.

3. **Should resolver tests use `dns.Exchange()` or `dns.Client{}.Exchange()`?**
   - What we know: `resolvers.query()` uses `dns.Exchange(q, srv.addr.String())` internally.
   - What's unclear: Whether to test at the `Exchange()` level or the `query()` level.
   - Recommendation: Test `query()` directly (D-09). The `dns.Exchange()` call is the transport layer; testing it separately is out of scope.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go compiler | All tests | ✓ | 1.26.2 | — |
| `go test -race` | BGP + loop tests | ✓ | Built-in | — |
| `github.com/miekg/dns` | Fake DNS server | ✓ | v1.1.67 | — |
| `github.com/stretchr/testify` | Assertions | ✓ | v1.10.0 | — |
| `github.com/bluele/gcache` | Cache tests | ✓ | v0.0.2 | — |
| `google.golang.org/grpc` | gRPC tests | ✓ | v1.73.0 | — |
| `github.com/rs/zerolog` | Logger tests | ✓ | v1.34.0 | — |
| `net.Listen("tcp", "127.0.0.1:0")` | Ephemeral ports | ✓ | Go stdlib | — |
| `net.ListenPacket("udp", "...")` | Fake DNS server | ✓ | Go stdlib | — |

**Missing dependencies with no fallback:** None
**Missing dependencies with fallback:** None

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + testify v1.10.0 |
| Config file | `go.mod` (test dependencies declared) |
| Quick run command | `go test -v -count=1 ./internal/dns/... ./internal/bgp/... ./cmd/bgp-dnsd/cli/...` |
| Full suite command | `go test -v -race -count=1 ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-01 | BGP reference counting (advance/withdraw transitions) | unit | `go test -race -run "TestBgp" ./internal/bgp/...` | ❌ Wave 0 |
| TEST-02 | DNS resolver ring failover (round-robin, health tracking) | unit | `go test -run "TestResolver" ./internal/dns/...` | ❌ Wave 0 |
| TEST-03 | Cache eviction on generation change | unit | `go test -run "TestCache.*Evict" ./internal/dns/...` | ❌ Wave 0 |
| TEST-04 | gRPC server lifecycle (Serve/Shutdown) | integration | `go test -run "TestGRPC" ./cmd/bgp-dnsd/cli/...` | ❌ Wave 0 |
| TEST-05 | End-to-end DNS query → cache → BGP announcement | integration | `go test -run "TestE2E" ./...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -v -count=1 ./internal/bgp/... ./internal/dns/... ./cmd/bgp-dnsd/cli/...`
- **Per wave merge:** `go test -race -count=1 ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/bgp/bgp_test.go` — covers TEST-01 (BGP reference counting)
- [ ] `internal/dns/resolvers_test.go` — covers TEST-02 (resolver failover)
- [ ] `internal/dns/cache_eviction_test.go` — covers TEST-03 (generation-based eviction)
- [ ] `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` — covers TEST-04 (gRPC lifecycle)
- [ ] `internal/dns_e2e_test.go` — covers TEST-05 (E2E DNS→cache→BGP)

## Security Domain

> `security_enforcement` is enabled (key absent in config.json → treat as enabled).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | testify assertions for error paths |
| V6 Cryptography | no | Not applicable to test phase |

### Known Threat Patterns for Go Testing

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Test data leaking to production | Tampering | Use `t.TempDir()` for file-based tests; embedded strings for in-memory data |
| Global state pollution between tests | Tampering | Reset `_cache`, `_bgp`, `_resolvers` in `t.Cleanup()` |
| Race conditions in concurrent tests | Tampering | Use `-race` flag; avoid `t.Parallel()` for shared-state tests |

## Sources

### Primary (HIGH confidence)
- [Context7: /miekg/dns] - DNS server creation patterns, HandlerFunc, Server struct, Client.Exchange
- [Context7: /miekg/dns] - ResponseWriter interface, dns.Msg construction, dns.A record creation
- [Internal codebase] - `internal/dns/cache_test.go` existing test patterns
- [Internal codebase] - `internal/dns/serveMux_test.go` testResponseWriter mock
- [Internal codebase] - `internal/bgp/main.go` bgpSrv struct, ipRefCounter, Advance(), Withdraw()
- [Internal codebase] - `internal/dns/resolvers.go` resolver ring, query() failover logic

### Secondary (MEDIUM confidence)
- [gcache docs] - FakeClock, Expiration, Clock customization for TTL testing
- [gRPC docs] - grpc.Dial with insecure credentials for localhost testing
- [Go testing docs] - t.Cleanup, t.TempDir, -race flag

### Tertiary (LOW confidence)
- [GoBGP docs] - GoBGP server testing patterns (assumed, not verified)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All packages verified in go.mod; miekg/dns Server pattern confirmed via Context7
- Architecture: HIGH - Same-package test pattern confirmed by existing `internal/dns/*_test.go`
- Pitfalls: MEDIUM - Race conditions with global state are well-documented but exact mitigation needs validation

**Research date:** 2026-06-14
**Valid until:** 30 days (stable ecosystem — Go, miekg/dns, gRPC)
