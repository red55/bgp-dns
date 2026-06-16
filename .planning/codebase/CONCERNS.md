# Codebase Concerns

**Analysis Date:** 2026-06-13

## Security Considerations

### No Authentication on gRPC Interface
**Risk:** The gRPC server (`cmd/bgp-dnsd/cli/cache.go:60`) uses `grpc.NewServer()` with no authentication or TLS. Any process that can reach the Unix socket or TCP port can execute `ClearCache`, `ReloadList`, and `ListCacheEntries`.
**Files:** `cmd/bgp-dnsd/cli/cache.go:60`, `cmd/bgp-dnsctl/commands/root.go:25`
**Current mitigation:** Unix socket in `/run/bgp-dnsd/bgp-dnsd.sock` (typically restricted by OS permissions). Default target uses `unix://` prefix.
**Recommendations:** Add Unix socket file permissions (0600), consider mTLS for TCP connections, or add a simple auth token via gRPC metadata.

### Insecure gRPC Credentials
**Risk:** The CLI connects with `grpc.WithTransportCredentials(insecure.NewCredentials())` (`cmd/bgp-dnsctl/commands/root.go:25`). No encryption or authentication.
**Files:** `cmd/bgp-dnsctl/commands/root.go:25`
**Recommendations:** Use TLS for TCP connections. For Unix sockets, rely on OS file permissions but document the assumption.

### No DNS Query Validation
**Risk:** DNS queries are forwarded to upstream resolvers without validation. A malicious or misconfigured domain list could cause DNS amplification or cache poisoning.
**Files:** `internal/dns/resolvers.go:89`, `internal/dns/resolve.go:12`
**Recommendations:** Add query type filtering (only allow A, AAAA, HTTPS), implement DNSSEC validation, add query rate limiting.

### BGP Peer Configuration Without Authentication
**Risk:** BGP peers are configured without MD5 authentication or TCP-AO. GoBGP supports peer authentication but it's not wired into the config.
**Files:** `internal/bgp/main.go:81-118`
**Recommendations:** Add `AuthPassword` field to `bgpNeighbor` config struct and pass to GoBGP peer configuration.

## Performance Characteristics

### O(n²) Set Difference
**Risk:** `internal/utils/main.go:3-27` implements `Difference()` with nested loops (O(n²) complexity). For domains with many IPs, this becomes slow.
**Files:** `internal/utils/main.go`
**Impact:** Every cache upsert (on every DNS resolution) calls `utils.Difference()` twice (arrived and gone).
**Fix approach:** Use a map-based set difference:
```go
func Difference(slice1, slice2 []string) []string {
    set := make(map[string]struct{}, len(slice2))
    for _, s := range slice2 { set[s] = struct{}{} }
    var diff []string
    for _, s := range slice1 { if _, ok := set[s]; !ok { diff = append(diff, s) } }
    return diff
}
```

### Round-Robin Resolver with No Backoff
**Risk:** `internal/dns/resolvers.go:84-115` iterates through the resolver ring on failure but has no exponential backoff or cooldown. Rapid failures could cause thundering herd.
**Files:** `internal/dns/resolvers.go:75-152`
**Impact:** All queries will cycle through failed resolvers before succeeding.
**Fix approach:** Add per-resolver last-failure timestamps and skip resolvers that failed recently.

### Cache Iteration on Every Loop Tick
**Risk:** `internal/dns/loop.go:23-51` calls `c.entries.GetALL(true)` on every tick, creating a full copy of the cache, then iterates all entries. With 5000 entries, this is expensive.
**Files:** `internal/dns/loop.go:23`
**Impact:** CPU usage scales linearly with cache size, even when no entries need refreshing.
**Fix approach:** Use a priority queue or time-sorted structure to only wake up when the next entry expires.

### Unbounded DNS Exchange Calls
**Risk:** `internal/dns/resolvers.go:89` calls `dns.Exchange(q, srv.addr.String())` for every query. Each call creates a new UDP connection.
**Files:** `internal/dns/resolvers.go:89`
**Impact:** High DNS query volume could exhaust ephemeral ports or overwhelm upstream resolvers.
**Fix approach:** Use a persistent `net.Dialer` with connection reuse or a DNS client with TCP fallback.

## Reliability / Robustness Concerns

### Global State Prevents Testing and Multiple Instances
**Risk:** Every subsystem (`dns`, `bgp`, `fswatcher`, `cli`) uses package-level singleton variables. No two instances of `bgp-dnsd` can run simultaneously.
**Files:** `internal/dns/main.go:16-20`, `internal/bgp/main.go:28`, `internal/fswatcher/main.go:24`, `cmd/bgp-dnsd/cli/cache.go:26-29`
**Impact:** Makes unit testing difficult (state leaks between tests), prevents horizontal scaling.
**Fix approach:** Refactor to pass state explicitly through function parameters and constructors.

### Race Condition in DNS Server Shutdown
**Risk:** `internal/dns/main.go:71-72` calls `_server.ShutdownContext(ctx)` but `_server` is set in a goroutine (`internal/dns/main.go:40-53`). If `Shutdown()` is called before the goroutine sets `_server`, it will panic with a nil pointer dereference.
**Files:** `internal/dns/main.go:71`
**Impact:** Crash on shutdown.
**Fix approach:** Add nil check before `_server.ShutdownContext(ctx)` or use a mutex to protect `_server` access.

### Missing Error Handling in BGP Operations
**Risk:** `internal/dns/cache.go:108-109` ignores BGP operation errors: `_ = bgp.Advance(arrived)` and `_ = bgp.Withdraw(gone)`.
**Files:** `internal/dns/cache.go:108-109`
**Impact:** BGP routes may become stale if the BGP server is unreachable, but the DNS cache continues to report success.
**Fix approach:** Log errors at Warn level or implement a retry mechanism. Monitor BGP peer state.

### Generation Counter Overflow
**Risk:** `cache.gen atomic.Uint64` will overflow after ~18 quintillion reloads. While unlikely in practice, the `findKeysByGeneration()` comparison (`gen <= oldGen`) would break.
**Files:** `internal/dns/cache.go:32`, `internal/dns/cache.go:126`
**Impact:** Cache corruption on overflow.
**Recommendation:** Acceptable risk for current use case, but document the assumption.

### File Watcher Blocks on dns.Load
**Risk:** `internal/fswatcher/loop.go:27` calls `dns.Load()` synchronously from the fsnotify event handler goroutine. If DNS resolution takes a long time (e.g., slow upstream resolvers), file events are queued and processed sequentially.
**Files:** `internal/fswatcher/loop.go:26-30`
**Impact:** Delayed reloads, potential event backlog.
**Fix approach:** Queue reload requests via the loop channel instead of calling synchronously.

### No DNS Query Timeout
**Risk:** `internal/dns/resolvers.go:89` calls `dns.Exchange(q, srv.addr.String())` with no timeout. A hung resolver will block the goroutine indefinitely.
**Files:** `internal/dns/resolvers.go:89`
**Impact:** DNS queries hang, cache entries never refresh.
**Fix approach:** Use `dns.Client{Timeout: ...}` with a configurable timeout.

## Test Coverage Status

### Packages With Tests
| Package | Test Files | Coverage Notes |
|---------|-----------|----------------|
| `internal/dns` | `cacheEntry_test.go` (303 lines, 13 tests), `cache_test.go` (134 lines, 7 tests) | Tests `cacheEntry` IP extraction, TTL, failures, generation, cache load behavior |
| `internal/debugdetect` | `debug_linux_test.go` (31 lines, 2 tests) | Tests `/proc/self/status` parsing |
| `cmd/bgp-dnsd/cli` | `cache_test.go` (45 lines, 2 tests) | Tests gRPC `ReloadList` nil request and uninitialized cache |
| `cmd/bgp-dnsctl/commands` | `list_test.go` (30 lines, 2 tests) | Tests CLI command structure |

### Packages Without Tests
| Package | Risk Level | Reason |
|---------|-----------|--------|
| `internal/bgp` | **High** | Core route announcement logic — reference counting, BGP path add/remove — completely untested |
| `internal/dns` (resolvers, resolve, loop) | **High** | DNS query routing, resolver failover, cache refresh loop — no tests |
| `internal/config` | **Medium** | Config parsing with decode hooks — no tests |
| `internal/log` | **Low** | Logging infrastructure — minimal risk |
| `internal/fswatcher` | **Medium** | File watching and reload triggering — no tests |
| `cmd/bgp-dnsd/cli` (Serve, Shutdown) | **Medium** | gRPC server lifecycle — no tests |
| `cmd/bgp-dnsctl/commands` (cache.go) | **Medium** | Cache list/clear CLI commands — no tests |

### Test Gaps
- **BGP reference counting logic** (`internal/bgp/main.go:135-182`) — the core mechanism that prevents premature withdrawal is untested
- **Resolver failover** (`internal/dns/resolvers.go:75-152`) — the ring-based round-robin with health tracking is untested
- **Cache eviction on entry expiration** (`internal/dns/loop.go`) — the refresher loop that re-resolves expired entries is untested
- **Integration tests** — no end-to-end tests that verify DNS → cache → BGP flow
- **gRPC server lifecycle** — no tests for `Serve()`/`Shutdown()` in `cmd/bgp-dnsd/cli/cache.go`

## Known Technical Debt

### TODO: BGP Context Propagation
**File:** `internal/bgp/bgp.go:43`
**Issue:** `//TODO: pass context` — `add()`, `find()`, `remove()` all use `context.Background()` instead of a cancellable context.
**Impact:** BGP operations cannot be cancelled during shutdown, potentially causing goroutine leaks.

### Typo in Config Struct Tag
**File:** `internal/config/bgp.go:9`
**Issue:** `Addressess` (double 's') instead of `Address` in the YAML/JSON tag.
**Impact:** Configuration files using `Addressess` will work, but it's inconsistent and confusing. New users will use the correct spelling and get silent failures.

### Unused Commented Code
**File:** `internal/dns/resolvers.go:127-148`
**Issue:** Large block of commented-out error handling code (22 lines of dead code with detailed error unwrapping logic).
**Impact:** Clutters the codebase, creates confusion about intended behavior.

### Unused Commented Code in BGP
**File:** `internal/bgp/main.go:19`
**Issue:** `//ipRefCounter  *hashmap.Map[string, *atomic.Uint64]` — commented-out alternative data structure.
**Impact:** Minimal, but suggests the code was migrated from a third-party hashmap to the standard map.

### Hardcoded Values
**File:** `internal/bgp/main.go:94`
**Issue:** `HoldTime: 240` — BGP hold timer is hardcoded.
**Files:** `internal/bgp/main.go:90` — `MultihopTtl: 254`, `internal/bgp/main.go:99` — `MtuDiscovery: true`
**Impact:** Cannot be tuned per-deployment without code changes.

### Missing `go generate` Directive
**File:** `Makefile:5`
**Issue:** Protobuf generation is triggered by `make pb` (buf generate) but there's no `//go:generate` directive in any Go file.
**Impact:** New developers won't know to regenerate protobuf code when modifying `.proto` files.

## Documentation Quality

### README.md
**File:** `README.md` (10 lines)
**Issue:** README is minimal — only contains a one-line description and an outdated YAML config snippet with `Timeouts` section that doesn't exist in the actual `AppCfg` struct.
**Impact:** Users must read source code to understand configuration.

### QWEN.md (Project Memory)
**File:** `QWEN.md` (138 lines)
**Issue:** This is the most comprehensive documentation, but it's named `QWEN.md` suggesting it was generated for an AI agent. It contains accurate architecture, config, and build instructions.
**Impact:** Good for developers who read it, but non-obvious file name for humans.

### Missing Documentation
- No `CONTRIBUTING.md`
- No `CHANGELOG.md`
- No `SECURITY.md`
- No architecture diagrams beyond ASCII art in docs
- No operational runbook (how to monitor, troubleshoot, scale)
- No API documentation for the gRPC service (beyond the proto file)

### Sample Domain List
**File:** `sample/my.lst` (7 domains)
**Issue:** Contains real production domains (cloudflare.com, google.com, github.com). May not be appropriate for all users.
**Impact:** Users may copy the sample without realizing it's production-specific.

## Scaling Limits

### Cache Size
**Current capacity:** 5000 entries (default from `appsettings.yml`)
**Limit:** `gcache.New(max).LFU()` — memory usage scales with entries. Each entry stores a full DNS message.
**Scaling path:** Increase `MaxEntries` in config. Consider memory-mapped storage for very large caches.

### BGP Peers
**Current capacity:** Limited by GoBGP server and system resources.
**Limit:** Each peer requires TCP connections and memory for route tables.
**Scaling path:** Configure peer addresses in YAML. GoBGP supports thousands of peers.

### DNS Query Rate
**Current capacity:** Unbounded — limited by system resources.
**Limit:** Each query spawns a goroutine for `dns.Exchange`. High query rates could exhaust file descriptors.
**Scaling path:** Add connection pooling, query rate limiting, and worker pool for DNS resolution.

---

*Concerns audit: 2026-06-13*
