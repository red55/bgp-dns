# Phase 02: Test Suite - Pattern Map

**Mapped:** 2026-06-14
**Files analyzed:** 5 (new test files)
**Analogs found:** 5 / 5

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/bgp/bgp_test.go` | test | CRUD (ref counting) | `internal/dns/cache_test.go` | exact — same-package test with `newTestXxx(t)` helper pattern |
| `internal/dns/resolvers_test.go` | test | request-response (DNS failover) | `internal/dns/serveMux_test.go` | exact — same-package test using `testResponseWriter` mock |
| `internal/dns/cache_eviction_test.go` | test | file-I/O (generation sweep) | `internal/dns/cache_test.go` | exact — same-package test loading domainlists via `t.TempDir()` |
| `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` | test | event-driven (gRPC lifecycle) | `cmd/bgp-dnsd/cli/cache_test.go` | exact — same-package test testing `CacheCliServiceImpl` |
| `internal/dns_e2e_test.go` | test | streaming (DNS→cache→BGP chain) | `internal/dns/regex_integration_test.go` | exact — integration test using `newTestCache(t)` + `testResponseWriter` |

## Pattern Assignments

### `internal/bgp/bgp_test.go` (test, CRUD — reference counting)

**Analog:** `internal/dns/cache_test.go`

**Package declaration** — same-package tests access private fields:
```go
package bgp
```
*(from `internal/dns/cache_test.go` line 1, `internal/dns/serveMux_test.go` line 1, all existing tests)*

**Imports pattern** (from `internal/dns/cache_test.go` lines 3-11):
```go
package bgp

import (
    "context"
    "sync/atomic"
    "testing"

    "github.com/red55/bgp-dns/internal/loop"
    "github.com/red55/bgp-dns/internal/log"
    "github.com/rs/zerolog"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**Logger initialization pattern** (from `internal/dns/cache_test.go` lines 13-15):
```go
func initTestLogger() {
    log.Init(zerolog.WarnLevel)
}
```

**Test helper with t.Helper()** (from `internal/dns/cache_test.go` lines 17-22):
```go
func newTestCache(t *testing.T) *cache {
    t.Helper()
    initTestLogger()
    l := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
    return newCache(100, time.Duration(60), newResolversWithLogger(&l), &l)
}
```

**Test naming convention** — `Test<Struct>_<Method>_<Scenario>` (from `internal/dns/cache_test.go` lines 32, 41, 62, 83):
```go
// TestCache_Load_FileNotFound verifies that load returns an error for a nonexistent file.
func TestCache_Load_FileNotFound(t *testing.T) { ... }

// TestCache_Load_GenerationIncrease verifies that loading a file increases the generation.
func TestCache_Load_GenerationIncrease(t *testing.T) { ... }
```

**Core test pattern for BGP reference counting** — construct `bgpSrv` directly, bypass `Serve()`, use `loop.Operation()` with sync wait:
```go
// From internal/bgp/main.go lines 36-46: how bgpSrv is constructed in production
srv := &bgpSrv{
    Loop:         loop.NewLoop(1),
    ipRefCounter: make(map[string]*atomic.Uint64),
    asn:          cfg.Bgp.Asn,
    id:           cfg.Bgp.Id,
}

// Test helper pattern (follow newTestCache helper convention):
func newTestBgpSrv(t *testing.T) *bgpSrv {
    t.Helper()
    initTestLogger()
    srv := &bgpSrv{
        Loop:         loop.NewLoop(1),
        ipRefCounter: make(map[string]*atomic.Uint64),
        asn:          65000,
    }
    // Start the loop goroutine — from internal/bgp/loop.go line 6
    ctx, cancel := context.WithCancel(context.Background())
    go srv.loop(ctx)
    t.Cleanup(func() { cancel() })
    return srv
}
```

**Reference counting test pattern** — test through `Operation(func, true)` (sync):
```go
// From internal/bgp/main.go lines 135-158: Advance() calls Operation with sync=true
// Test pattern: call Operation directly on the constructed srv

// Scenario 1: single IP advance/withdraw
func TestBgp_ReferenceCounting_SingleIP(t *testing.T) {
    srv := newTestBgpSrv(t)
    
    // Advance: refs, ok := srv.ipRefCounter[ip]; if !ok, create counter
    err := srv.Operation(func() error {
        refs, ok := srv.ipRefCounter["10.0.0.1"]
        if !ok {
            refs = new(atomic.Uint64)
            srv.ipRefCounter["10.0.0.1"] = refs
        }
        c := refs.Add(1)
        if c == 1 {
            // First reference — would call add() in production
        }
        return nil
    }, true)
    require.NoError(t, err)
    
    // Verify counter state directly
    refs, exists := srv.ipRefCounter["10.0.0.1"]
    assert.True(t, exists)
    assert.Equal(t, uint64(1), refs.Load())
}
```

**Assert patterns from existing tests** (from `internal/dns/cache_test.go` lines 33-39):
```go
// Pattern A: assert for behavioral checks
assert.True(t, c.generation() > 0)
assert.Equal(t, uint64(1), refs.Load())

// Pattern B: require for setup assertions (from regex_integration_test.go line 22)
require.NoError(t, err)
require.Len(t, c.mux.regex, 1)
```

---

### `internal/dns/resolvers_test.go` (test, request-response — DNS failover)

**Analog:** `internal/dns/serveMux_test.go`

**testResponseWriter mock** (from `internal/dns/serveMux_test.go` lines 12-27):
```go
// testResponseWriter is a minimal dns.ResponseWriter mock for unit tests.
type testResponseWriter struct {
    msg *dns.Msg
}

func (w *testResponseWriter) LocalAddr() net.Addr                         { return &net.UDPAddr{} }
func (w *testResponseWriter) RemoteAddr() net.Addr                        { return &net.UDPAddr{} }
func (w *testResponseWriter) WriteMsg(r *dns.Msg) error                   { w.msg = r; return nil }
func (w *testResponseWriter) Write([]byte) (int, error)                   { return 0, nil }
func (w *testResponseWriter) Close() error                                { return nil }
func (w *testResponseWriter) TsigStatus() error                           { return nil }
func (w *testResponseWriter) TsigTimersOnly(bool)                         {}
func (w *testResponseWriter) Hijack()                                     {}
func (w *testResponseWriter) Network() string                             { return "udp" }
func (w *testResponseWriter) CloseNotify() <-chan bool                    { return nil }
func (w *testResponseWriter) Requested() string                           { return "" }
```

**newTestMsg helper** (from `internal/dns/serveMux_test.go` lines 29-33):
```go
func newTestMsg(fqdn string, qtype uint16) *dns.Msg {
    m := new(dns.Msg)
    m.SetQuestion(fqdn, qtype)
    return m
}
```

**Fake DNS server pattern** (from RESEARCH.md lines 168-196):
```go
// Create a fake DNS server with controlled responses
func newFakeDNSServer(t *testing.T, handler dns.HandlerFunc) (*dns.Server, string) {
    t.Helper()
    ln, err := net.ListenPacket("udp", "127.0.0.1:0")
    require.NoError(t, err)
    
    srv := &dns.Server{
        PacketConn: ln,
        Handler:    handler,
        Net:        "udp",
    }
    go srv.ActivateAndServe()
    t.Cleanup(func() { srv.Shutdown() })
    return srv, ln.LocalAddr().String()
}
```

**Resolver test pattern** — construct `resolvers` directly, use fake DNS server:
```go
// From internal/dns/resolvers.go lines 48-61: resolvers struct and constructor
type resolvers struct {
    log.Log
    m  sync.RWMutex
    rs *ring.Ring
}

func newResolvers(c []*net.UDPAddr) *resolvers {
    r := &resolvers{
        Log: log.NewLog(log.L(), "resolvers"),
    }
    r.setResolvers(c)
    return r
}

// Test helper:
func newTestResolvers(t *testing.T, addrs []*net.UDPAddr) *resolvers {
    t.Helper()
    initTestLogger()
    return newResolvers(addrs)
}
```

**Test scenarios from RESEARCH.md D-07** — 4 failover scenarios:
```go
// Scenario 1: single resolver success
func TestResolver_SingleResolverSuccess(t *testing.T) {
    srv, addr := newFakeDNSServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
        m := new(dns.Msg)
        m.SetReply(r)
        m.SetRcode(r, dns.RcodeSuccess)
        m.Answer = append(m.Answer, &dns.A{
            Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
            A: net.ParseIP("10.0.0.1"),
        })
        w.WriteMsg(m)
    })
    
    resolvers := newTestResolvers(t, []*net.UDPAddr{{IP: net.ParseIP("127.0.0.1"), Port: 0}})
    // ... configure resolver to use addr ...
    
    msg := newTestMsg("test.example.com.", dns.TypeA)
    result, err := resolvers.query(msg)
    assert.NoError(t, err)
    assert.NotNil(t, result)
}

// Scenario 2: failover to next resolver on failure
func TestResolver_FailoverToNext(t *testing.T) {
    // First server returns error, second succeeds
    // Verify ring rotates: rs.rs = rs.rs.Next() (from resolvers.go line 115)
}

// Scenario 3: recovery of failed resolver
func TestResolver_Recovery(t *testing.T) {
    // Server fails, then succeeds on retry
    // Verify okay state transitions via srv.ok() / srv.fail()
}

// Scenario 4: all resolvers fail
func TestResolver_AllFail(t *testing.T) {
    // All servers return errors
    // Verify ErrNoResolvers or combined error (from resolvers.go lines 117-125)
}
```

**Ring position verification** (from `internal/dns/resolvers.go` lines 75-151):
```go
// The query() method iterates the ring:
// head := rs.rs
// ... for each resolver: rs.rs = rs.rs.Next()
// if head == rs.rs { all failed }
//
// Test: verify ring position after failures
// resolvers.m.RLock()
// current := resolvers.rs
// // Walk the ring and check resolver.okay state
// current.Do(func(v interface{}) {
//     r := v.(*resolver)
//     assert.False(t, r.isOk()) // or assert.True for success case
// })
// resolvers.m.RUnlock()
```

---

### `internal/dns/cache_eviction_test.go` (test, file-I/O — generation-based eviction)

**Analog:** `internal/dns/cache_test.go`

**newTestCache helper** (from `internal/dns/cache_test.go` lines 17-22):
```go
func newTestCache(t *testing.T) *cache {
    t.Helper()
    initTestLogger()
    l := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
    return newCache(100, time.Duration(60), newResolversWithLogger(&l), &l)
}
```

**TempDir + file write pattern** (from `internal/dns/cache_test.go` lines 42-58):
```go
tmpDir := t.TempDir()
listFile := filepath.Join(tmpDir, "test.lst")
err := os.WriteFile(listFile, []byte("example.com\n"), 0644)
require.NoError(t, err)

_ = c.load(listFile)
assert.True(t, c.generation() > 0)
```

**Cache eviction test pattern** — use gcache FakeClock for deterministic TTL testing:
```go
// From internal/dns/cache.go line 45: cache.entries is gcache.Cache
// r.entries = gcache.New(max).LFU().EvictedFunc(r.onEntryEvicted).Build()

// Test eviction by generation:
func TestCache_EvictionByGeneration(t *testing.T) {
    c := newTestCache(t)
    
    tmpDir := t.TempDir()
    listFile := filepath.Join(tmpDir, "test.lst")
    err := os.WriteFile(listFile, []byte("example.com\ntest.com\n"), 0644)
    require.NoError(t, err)
    
    err = c.load(listFile)
    require.NoError(t, err)
    gen1 := c.generation()
    
    // Verify entries exist
    _, err = c.entries.Get(newCacheKey("example.com.", dns.TypeA))
    assert.NoError(t, err)
    
    // Load new generation
    err = c.load(listFile) // increments generation
    require.NoError(t, err)
    gen2 := c.generation()
    assert.Greater(t, gen2, gen1)
    
    // Old generation entries should be evicted
    // via evictByGeneration() calling findKeysByGeneration(gen)
    // then unregister() which removes from mux AND entries
}
```

**FakeClock pattern for TTL eviction** (from RESEARCH.md lines 445-478):
```go
import "github.com/bluele/gcache"

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
    
    c.entries.Set("key", "value")
    fakeClock.Advance(6 * time.Second)
    
    _, err := c.entries.Get("key")
    assert.Error(t, err, "entry should be evicted after TTL")
}
```

---

### `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` (test, event-driven — gRPC lifecycle)

**Analog:** `cmd/bgp-dnsd/cli/cache_test.go`

**Existing test pattern** (from `cmd/bgp-dnsd/cli/cache_test.go` lines 13-44):
```go
package cli

import (
    "os"
    "path/filepath"
    "testing"
    
    "github.com/red55/bgp-dns/api"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

func TestReloadList_NilRequest(t *testing.T) {
    s := &CacheCliServiceImpl{}
    _, err := s.ReloadList(nil, nil)
    if err == nil {
        t.Error("expected error for nil request")
    }
}

func TestReloadList_UninitializedCache(t *testing.T) {
    tmpDir := t.TempDir()
    tmpFile := filepath.Join(tmpDir, "test-list.lst")
    if err := os.WriteFile(tmpFile, []byte{}, 0600); err != nil {
        t.Fatalf("failed to create temp list file: %v", err)
    }
    _listFile = tmpFile
    
    s := &CacheCliServiceImpl{}
    _, err := s.ReloadList(nil, &api.ReloadListRequest{})
    
    if err == nil {
        t.Error("expected error when cache is not initialized")
    }
    st, ok := status.FromError(err)
    if !ok {
        t.Error("expected gRPC status error")
    }
    if st.Code() != codes.FailedPrecondition {
        t.Errorf("expected FailedPrecondition, got %v", st.Code())
    }
}
```

**gRPC lifecycle test pattern** (from RESEARCH.md lines 481-535):
```go
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
    // Start gRPC server on ephemeral port
    listener, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    
    grpcServer := grpc.NewServer()
    service := &CacheCliServiceImpl{}
    api.RegisterBgpDnsServiceServer(grpcServer, service)
    
    go grpcServer.Serve(listener)
    defer grpcServer.GracefulStop()
    
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
}
```

**gRPC error code patterns** (from `cmd/bgp-dnsd/cli/cache.go` lines 79-124):
```go
// Pattern: Check for specific gRPC error codes
// InvalidArgument: nil request/stream
// FailedPrecondition: cache not initialized (dns.ENotInitialized)
// Internal: other errors

// From cache.go lines 97-111:
if req == nil {
    return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
}
if errors.Is(err, dns.ENotInitialized) {
    return nil, status.Errorf(codes.FailedPrecondition, "failed to clear cache: %v", err)
}
return nil, status.Errorf(codes.Internal, "failed to clear cache: %v", err)
```

---

### `internal/dns_e2e_test.go` (test, streaming — DNS→cache→BEP chain)

**Analog:** `internal/dns/regex_integration_test.go`

**Integration test pattern** (from `internal/dns/regex_integration_test.go` lines 16-36):
```go
package dns

import (
    "os"
    "path/filepath"
    "testing"
    
    "github.com/miekg/dns"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRegexIntegration_LoadRegexPattern(t *testing.T) {
    c := newTestCache(t)
    
    tmpDir := t.TempDir()
    listFile := filepath.Join(tmpDir, "mixed.lst")
    content := "example.com\nregex:([a-z]+)\\.internal\\.corp\n"
    require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))
    
    err := c.load(listFile)
    require.NoError(t, err)
    
    assert.True(t, c.generation() > 0, "generation should increase after load")
    
    // Behavioral assertion: verify exact entry is registered in mux
    _, hasExact := c.mux.exact["example.com."]
    assert.True(t, hasExact, "example.com should be registered in exact map")
    
    // Behavioral assertion: verify regex entry is registered
    require.Len(t, c.mux.regex, 1, "should have exactly 1 regex handler")
}
```

**E2E test pattern** — chain-of-responsibility: construct DNS msg, write to mock ResponseWriter, verify cache.resolve() calls bgp.Advance():
```go
// From internal/dns/resolve.go lines 8-33: resolve() calls rs.query(), then c.upsert()
// From internal/dns/cache.go lines 88-118: upsert() calls bgp.Advance(arrived) and bgp.Withdraw(gone)

func TestE2E_DnsQueryToCacheToBgp(t *testing.T) {
    c := newTestCache(t)
    
    // Register a domain in the mux
    c.mux.HandleFunc("example.com.", func(w dns.ResponseWriter, r *dns.Msg) {
        c.resolve(w, r, false)  // resolve with w=nil to skip WriteMsg, just upsert
    })
    
    // Create a fake DNS server that returns a controlled response
    handler := func(w dns.ResponseWriter, r *dns.Msg) {
        m := new(dns.Msg)
        m.SetReply(r)
        m.SetRcode(r, dns.RcodeSuccess)
        m.Answer = append(m.Answer, &dns.A{
            Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
            A: net.ParseIP("10.0.0.1"),
        })
        w.WriteMsg(m)
    }
    srv, addr := newFakeDNSServer(t, handler)
    _ = srv
    
    // Configure cache resolvers to point to fake server
    // ... (set up _resolvers or use test helper)
    
    // Simulate DNS query through mux
    w := &testResponseWriter{}
    msg := newTestMsg("example.com.", dns.TypeA)
    c.mux.ServeDNS(w, msg)
    
    // Verify: bgp.Advance() was called with correct IPs
    // (inspect ipRefCounter directly since it's same-package)
    // Note: we can't verify the actual bgp.AddPath call since bgp field is nil
    // But we can verify the ref counter was incremented
}
```

---

## Shared Patterns

### Logger Initialization
**Source:** `internal/dns/cache_test.go` lines 13-15
**Apply to:** All test files that need logging
```go
func initTestLogger() {
    log.Init(zerolog.WarnLevel)
}
```

### Test Helper Convention
**Source:** `internal/dns/cache_test.go` lines 17-22
**Apply to:** All `newTestXxx` helpers
```go
func newTestXxx(t *testing.T) *xxx {
    t.Helper()
    initTestLogger()
    // ... construct and return ...
}
```

### testify/assert vs testify/require
**Source:** `internal/dns/regex_integration_test.go`
- **`require`**: For setup assertions that must pass before continuing (file writes, load calls)
- **`assert`**: For behavioral assertions (checking state, verifying outcomes)
```go
// require for setup — stops test on failure
require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))
require.NoError(t, err)
require.Len(t, c.mux.regex, 1)

// assert for verification — continues even if false
assert.True(t, hasExact, "example.com should be registered")
assert.Equal(t, uint64(1), refs.Load())
```

### Cleanup Pattern
**Source:** `internal/dns/cache_test.go` (implicit via `t.TempDir()`), `regex_integration_test.go` line 19
```go
// t.TempDir() auto-cleans — no explicit cleanup needed for temp dirs
tmpDir := t.TempDir()

// For goroutines and resources, use t.Cleanup()
t.Cleanup(func() {
    srv.Shutdown()
    cancel()
    grpcServer.GracefulStop()
})
```

### Same-Package Private Field Access
**Source:** `internal/dns/regex_integration_test.go` lines 30-35
```go
// Access private fields directly since test is in same package
_, hasExact := c.mux.exact["example.com."]
assert.Len(t, c.mux.regex, 1)
assert.Equal(t, "([a-z]+)\\.internal\\.corp", c.mux.regex[0].pattern.String())
```

### testResponseWriter Mock
**Source:** `internal/dns/serveMux_test.go` lines 12-27
**Apply to:** resolver tests and E2E tests that need a DNS ResponseWriter mock
```go
// Copy the full testResponseWriter struct from serveMux_test.go
// It implements dns.ResponseWriter interface
// For resolver tests, may need to extend with additional methods
```

### Global State Isolation
**Source:** `internal/dns/cache_test.go` — each test calls `newTestCache(t)` which creates a fresh cache
**Apply to:** All tests that touch `_cache`, `_resolvers`, or `_bgp` globals
```go
// DO NOT use t.Parallel() for tests that modify global state
// Each test must create its own instance or reset globals in t.Cleanup()
func TestResolver_Failover(t *testing.T) {
    // Create fresh resolvers instance — don't reuse across subtests
    resolvers := newResolvers([]*net.UDPAddr{...})
    // ... test ...
}
```

## No Analog Found

No files — all 5 new test files have close analogs in the existing codebase.

## Metadata

**Analog search scope:** `internal/dns/`, `internal/bgp/`, `cmd/bgp-dnsd/cli/`, `internal/loop/`
**Files scanned:** 12 (5 existing test files + 7 source files)
**Pattern extraction date:** 2026-06-14
