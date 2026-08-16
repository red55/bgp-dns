# Architecture

**Analysis Date:** 2026-06-13

## System Overview

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│                          bgp-dnsd (Daemon)                                   │
│                                                                              │
│  ┌─────────────┐    ┌─────────────┐    ┌──────────────┐    ┌─────────────┐  │
│  │  DNS Server  │    │  DNS Cache   │    │  File Watcher │    │  gRPC CLI   │  │
│  │ miekg/dns    │    │  gcache LFU   │    │  fsnotify     │    │  Server     │  │
│  │ :5354 UDP    │    │  entries:5000 │    │  my.lst       │    │  Unix sock  │  │
│  └──────┬───────┘    └──────┬───────┘    └──────────────┘    └──────┬──────┘  │
│         │                   │                                       │           │
│         ▼                   ▼                                       │           │
│  ┌─────────────┐    ┌─────────────┐                                │           │
│  │ Upstream    │    │ BGP Router   │◄───────────────────────────────┘           │
│  │ Resolvers    │    │ (GoBGP)      │  gRPC client (bgp-dnsctl)                │
│  │ 77.88.8.8:53 │    │ ASN:65530    │                                            │
│  │ 77.88.8.1:53 │    │ :8179        │                                            │
│  └─────────────┘    └──────┬───────┘                                            │
│                            │                                                     │
│                            ▼                                                     │
│                   ┌─────────────────┐                                           │
│                   │  BGP Peers      │                                            │
│                   │  192.168.151.44 │                                            │
│                   │  :179           │                                            │
│                   └─────────────────┘                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| **Daemon Entrypoint** | Bootstrap all subsystems, handle graceful shutdown, signal management | `cmd/bgp-dnsd/main.go` |
| **DNS Server** | Listen on UDP, intercept queries for listed domains, proxy unknown queries upstream | `internal/dns/main.go` |
| **DNS Cache** | Store resolved A/HTTPS records with TTL, eviction, generation tracking for reload | `internal/dns/cache.go` |
| **Cache Entry** | Hold DNS response, track TTL/expiration, failure counter, generation | `internal/dns/cacheEntry.go` |
| **DNS Resolver Ring** | Round-robin upstream resolver selection with health tracking | `internal/dns/resolvers.go` |
| **DNS Refresher Loop** | Periodically re-resolve expired cache entries | `internal/dns/loop.go` |
| **BGP Server** | Manage GoBGP server, peer connections, route announcements/withdrawals with reference counting | `internal/bgp/main.go` |
| **BGP Path Operations** | Add/remove BGP paths for IP prefixes, find existing prefixes | `internal/bgp/bgp.go` |
| **BGP Zero Logger** | Bridge GoBGP's log interface to zerolog | `internal/bgp/zerologger.go` |
| **File Watcher** | Monitor domain list file for changes, trigger reload | `internal/fswatcher/main.go` |
| **File Watcher Loop** | Handle fsnotify events, call `dns.Load()` on write/create | `internal/fswatcher/loop.go` |
| **Controlled Loop** | Serialize operations on BGP/DNS state through a single goroutine | `internal/loop/main.go` |
| **Configuration** | YAML config parsing via Viper with custom decode hooks for net.IP/zerolog.Level | `internal/config/main.go` |
| **Config Models** | AppCfg, bgpCfg, dnsCfg, logCfg typed structs | `internal/config/config.go`, `internal/config/bgp.go`, `internal/config/dns.go`, `internal/config/log.go` |
| **Logging** | Thread-safe zerolog wrapper with module names, global logger | `internal/log/main.go` |
| **Application** | Bootstrap, global flags (target/config), debugger detection, StdOut/StdErr helpers | `internal/app/main.go` |
| **Version** | Build-time version injection via ldflags | `internal/version/main.go` |
| **gRPC Server** | Expose cache management API (ListCacheEntries, ClearCache, ReloadList) | `cmd/bgp-dnsd/cli/cache.go` |
| **gRPC Proto** | Protocol buffer definitions for BgpDnsService | `proto/api/bgp-dns.proto` |
| **gRPC Generated** | Compiled protobuf/GRPC Go code | `api/bgp-dns.pb.go`, `api/bgp-dns_grpc.pb.go` |
| **CLI Tool** | `bgp-dnsctl` — control daemon via gRPC (cache list/clear, list reload) | `cmd/bgp-dnsctl/main.go`, `cmd/bgp-dnsctl/commands/` |
| **Utils** | Set difference helper for BGP announce/withdraw logic | `internal/utils/main.go` |
| **Debug Detect** | Platform-specific debugger detection (Linux /proc, Windows API, macOS fallback) | `internal/debugdetect/` |

## Pattern Overview

**Overall: Multi-subsystem daemon with controlled concurrency**

**Key Characteristics:**
- **Subsystem-per-goroutine architecture**: Each major subsystem (DNS, BGP, File Watcher, gRPC) runs in its own goroutine with lifecycle management
- **Generation-based reload**: Cache entries track a generation number; when the domain list file is reloaded, a new generation is incremented and stale entries are evicted
- **Reference-counted BGP announcements**: IP prefixes are announced/withdrawn based on a reference counter (`ipRefCounter map[string]*atomic.Uint64`) to handle multiple cache entries for the same IP
- **Controlled loop pattern**: The `internal/loop` package provides a single-threaded operation queue that serializes mutations to shared state (BGP server, DNS cache)
- **Context-based configuration passing**: Config is injected via `context.WithValue(ctx, "cfg", &AppCfg{})` — a string-keyed context value pattern
- **Global package-level state**: Each subsystem (`dns`, `bgp`, `fswatcher`) uses package-level singleton variables (`_server`, `_bgp`, `_watcher`)

## Layers

### Application Layer
- **Location:** `cmd/bgp-dnsd/main.go`, `cmd/bgp-dnsctl/main.go`
- **Contains:** Bootstrap, signal handling, binary entrypoints
- **Depends on:** All internal packages
- **Used by:** OS process manager / systemd

### Service Layer (Subsystems)
- **Location:** `internal/dns/`, `internal/bgp/`, `internal/fswatcher/`, `cmd/bgp-dnsd/cli/`
- **Purpose:** Core daemon functionality — DNS resolution, BGP routing, file watching, CLI management
- **Contains:** Server implementations, cache management, protocol handling
- **Depends on:** `internal/config/`, `internal/log/`, `internal/loop/`
- **Used by:** Application layer, external clients (DNS resolvers, BGP peers, CLI)

### Infrastructure Layer
- **Location:** `internal/loop/`, `internal/log/`, `internal/utils/`
- **Purpose:** Shared concurrency primitives, logging, utilities
- **Contains:** Single-threaded operation queue, zerolog wrapper, set operations
- **Depends on:** Standard library, `github.com/rs/zerolog`
- **Used by:** All service layer packages

### Configuration Layer
- **Location:** `internal/config/`
- **Purpose:** YAML config loading, parsing, type conversion
- **Contains:** Viper initialization, decode hooks for `net.IP` and `zerolog.Level`
- **Depends on:** `github.com/spf13/viper`, `github.com/rs/zerolog`
- **Used by:** All service layer packages (via `ctx.Value("cfg")`)

### External Integration Layer
- **Location:** `proto/api/`, `api/`
- **Purpose:** gRPC service definitions and generated code
- **Contains:** Protobuf definitions, Go gRPC client/server stubs
- **Depends on:** `google.golang.org/grpc`, `google.golang.org/protobuf`
- **Used by:** `cmd/bgp-dnsd/cli/`, `cmd/bgp-dnsctl/commands/`

## Data Flow

### Primary DNS Query Path

1. **Client sends DNS query** to `bgp-dnsd` on UDP `:5354` (`internal/dns/main.go:41-53`)
2. **DNS handler dispatches:**
   - If query matches a registered FQDN from the domain list → `cache.resolve()` intercepts (`internal/dns/cache.go:136-153`)
   - Otherwise → `_resolvers.proxyQuery()` forwards upstream (`internal/dns/resolvers.go:154-165`)
3. **For registered domains:** `cache.resolve()` queries upstream resolvers via `resolvers.query()` (`internal/dns/resolve.go:8-32`)
4. **On success:** `cache.upsert()` stores the response, compares with previous IPs, triggers BGP `Advance()`/`Withdraw()` (`internal/dns/cache.go:86-117`)
5. **Response written** back to client via `dns.ResponseWriter.WriteMsg()` (`internal/dns/resolve.go:17-22`)

### BGP Route Announcement Path

1. **Cache upsert** computes set difference between previous and current IPs (`internal/dns/cache.go:105-106`)
2. **`bgp.Advance(arrived)`** queues operations to the BGP loop (`internal/bgp/main.go:135-158`)
3. **BGP loop** processes operations: increments reference count, calls `bgp.add()` if first reference (`internal/bgp/main.go:136-158`)
4. **`bgp.add()`** constructs a `bgpapi.Path` with NLRI, AS path, next hop, origin attributes, and calls GoBGP `AddPath()` (`internal/bgp/bgp.go:12-51`)
5. **On eviction/update:** `bgp.Withdraw(gone)` decrements reference count, calls `bgp.remove()` if count reaches zero (`internal/bgp/main.go:160-182`)

### Domain List Reload Path

1. **File watcher** detects `fsnotify.Write` or `fsnotify.Create` (`internal/fswatcher/loop.go:26`)
2. **`dns.Load(fn)`** opens file, increments generation, registers each domain, evicts stale entries (`internal/dns/cache.go:194-239`)
3. **`bgp-dnsctl list reload`** triggers the same path via gRPC `ReloadList` RPC (`cmd/bgp-dnsd/cli/cache.go:113-124`)

### gRPC CLI Path

1. **`bgp-dnsctl cache list`** connects to the daemon's gRPC server via Unix socket (`cmd/bgp-dnsctl/commands/root.go:24-27`)
2. **Server streaming RPC** `ListCacheEntries` iterates all cache entries and streams responses (`cmd/bgp-dnsctl/commands/cache.go:44-73`)
3. **Unary RPC** `ClearCache` clears all entries and returns count (`cmd/bgp-dnsctl/commands/cache.go:76-101`)

## Key Abstractions

### Controlled Single-Threaded Loop
- **Purpose:** Serialize mutations to shared state (BGP server, DNS cache) to avoid races
- **Implementation:** `internal/loop/main.go` — a buffered channel of `loopOp` structs processed by a single goroutine
- **Usage:** `bgp.Advance()`, `bgp.Withdraw()`, `cache.notifyChanged()` all use `loop.Operation()` to queue work
- **Pattern:** `chan *loopOp` with buffered size 1, `Operation()` sends a function to the channel and optionally waits for result

### Generation-Based Cache Management
- **Purpose:** Track which cache entries belong to which domain list load, enabling atomic reload/evict
- **Implementation:** `cache.gen atomic.Uint64` incremented on each `load()`, each `cacheEntry` stores its generation
- **Eviction:** `evictByGeneration()` finds all entries with `gen <= oldGen`, unregisters and removes them
- **Usage:** `cache.load()` → `increaseGeneration()` → `register()` → `evictByGeneration(oldGeneration)`

### Reference-Counted BGP Announcements
- **Purpose:** Same IP may be resolved by multiple domains; only withdraw when last domain no longer resolves to it
- **Implementation:** `ipRefCounter map[string]*atomic.Uint64` in `bgpSrv`
- **Advance:** Increment counter; only call `bgp.add()` when counter transitions from 0→1
- **Withdraw:** Decrement counter; only call `bgp.remove()` when counter transitions from 1→0

## Entry Points

### `bgp-dnsd` Daemon
- **Location:** `cmd/bgp-dnsd/main.go`
- **Triggers:** Process start (systemd, manual, container)
- **Responsibilities:**
  1. Initialize application and logging (`app.New()`)
  2. Parse CLI flags (`cobra.Command.Execute()`)
  3. Load configuration (`config.Init()`)
  4. Start BGP server → gRPC CLI server → DNS server → load domain list → start file watcher
  5. Block on signal (SIGINT) or context cancellation
  6. Graceful shutdown in reverse order

### `bgp-dnsctl` CLI
- **Location:** `cmd/bgp-dnsctl/main.go`
- **Triggers:** User invocation
- **Responsibilities:**
  1. Initialize application and logging
  2. Connect to daemon via gRPC (Unix socket or TCP)
  3. Execute command (cache list, cache clear, list reload)
  4. Close connection

## Architectural Constraints

- **Threading:** Each subsystem runs in its own goroutine; BGP and DNS mutations are serialized through `internal/loop` (single goroutine per subsystem with buffered channel)
- **Global state:** Heavy use of package-level variables — `_server`, `_resolvers`, `_cache`, `_cancel` in `internal/dns`; `_bgp` in `internal/bgp`; `_watcher` in `internal/fswatcher`; `_listener`, `_listFile` in `cmd/bgp-dnsd/cli/cache.go`
- **Circular imports:** None detected — the dependency graph is strictly hierarchical: `cmd/` → `internal/` → external packages
- **Context-based config:** Configuration is passed via `context.WithValue(ctx, "cfg", *config.AppCfg)` — string-keyed context values, not a typed context value pattern
- **Platform-specific code:** `internal/debugdetect/` uses build tags for Linux (`/proc/self/status`), Windows (`IsDebuggerPresent`), Darwin (no-op), and other platforms (no-op fallback)

## Anti-Patterns

### Global Package-Level State

**What happens:** Each subsystem (`dns`, `bgp`, `fswatcher`, `cli`) uses package-level singleton variables (`_server`, `_bgp`, `_watcher`, `_cache`, `_listener`, `_listFile`).

**Why it's wrong:** Makes testing difficult (state leaks between tests), prevents multiple instances, and creates hidden coupling between initialization and usage. Functions like `bgp.Advance()` depend on `_bgp` being non-nil.

**Do this instead:** Pass state explicitly through function parameters or a struct. For example, `func Advance(srv *bgpSrv, ips []string) error` instead of relying on the package-level `_bgp`.

### Context Value for Configuration

**What happens:** Config is stored in context as `ctx.Value("cfg").(*config.AppCfg)` using a string key (`internal/dns/main.go:29`, `internal/dns/loop.go:17`, `internal/bgp/main.go:37`).

**Why it's wrong:** String-keyed context values lose type safety at compile time, are not self-documenting, and can collide with other context values.

**Do this instead:** Use a typed context key:

```go
type configKey struct{}
ctx = context.WithValue(ctx, configKey{}, cfg)
cfg := ctx.Value(configKey{}).(*config.AppCfg)
```

### Panic on Initialization Failure

**What happens:** `cmd/bgp-dnsd/main.go` uses `panic()` for BGP start failure, CLI server start failure, DNS start failure, domain list load failure, and file watcher start failure.

**Why it's wrong:** Panics prevent graceful error handling, make testing impossible, and bypass defer cleanup in some scenarios. The `log.L().Fatal()` in `internal/dns/main.go:51` and `internal/bgp/main.go:66,117` also call `Fatal()` which calls `os.Exit(1)` via zerolog.

**Do this instead:** Return errors and let the caller decide whether to panic, log, or handle gracefully.

## Error Handling

**Strategy:** Mixed — panics/fatals for startup failures, returned errors for runtime operations.

**Patterns:**
- **Startup failures:** `panic(e)` in `cmd/bgp-dnsd/main.go` (lines 35, 39, 57, 66, 75, 84, 88)
- **Binding failures:** `log.L().Fatal()` in `internal/dns/main.go:51` and `cmd/bgp-dnsd/cli/cache.go:41,56,71`
- **Runtime errors:** BGP operations return errors but are mostly ignored with `_ = bgp.Advance(...)` (`internal/dns/cache.go:108-109`)
- **gRPC errors:** `status.Error(codes.X, ...)` for validation and precondition failures (`cmd/bgp-dnsd/cli/cache.go:81,99,104,115,119`)
- **DNS resolver failures:** `ErrNoResolvers`, `ErrEmptyAnswer` sentinel errors (`internal/dns/resolvers.go:16-17`)

## Cross-Cutting Concerns

**Logging:** zerolog with console writer, structured fields, module name prefix (`"m"` field), configurable level via YAML config. All subsystems embed `log.Log` for consistent formatting.

**Validation:** Minimal — gRPC handlers validate nil requests (`cmd/bgp-dnsd/cli/cache.go:81,99,115`), domain list entries validate length ≥ 2 (`internal/dns/cache.go:137-138`).

**Authentication:** None — gRPC server uses insecure credentials, no TLS or auth on DNS or BGP interfaces.

---

*Architecture analysis: 2026-06-13*
