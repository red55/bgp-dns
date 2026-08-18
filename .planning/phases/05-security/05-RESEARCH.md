# Phase 5: Security - Research

**Researched:** 2026-08-18
**Domain:** Hardening a Go daemon's gRPC/Unix-socket control plane, DNS query-type filtering (miekg/dns), BGP peer authentication (GoBGP v3)
**Confidence:** HIGH

## User Constraints (from CONTEXT.md)

No CONTEXT.md exists for this phase. There are no locked decisions, discretion areas, or deferred ideas from `/gsd-discuss-phase`. Research scope is therefore bounded by:

- **REQUIREMENTS.md security section** (SEC-01 … SEC-04, quoted in the Phase Requirements table below)
- **ROADMAP.md Phase 5** goal: *"Harden gRPC interface, add query validation, support BGP peer authentication."*
- **Explicit out-of-scope statement in REQUIREMENTS.md:** OAuth/API-key authentication for the gRPC interface is out of scope — the Unix-socket file permissions (0600) are deemed adequate access control for the CLI channel.
- **QWEN.md / AGENTS.md conventions** (see Project Constraints below).

## Phase Requirements

| ID | Description (from REQUIREMENTS.md) | Research Support |
|----|-------------------------------------|------------------|
| SEC-01 | gRPC Unix socket is created with mode 0600 (owner-only read/write) | Current socket creation lives in `cli.Serve()` (`cmd/bgp-dnsd/cli/cache.go:55`) with **no explicit mode** — mode today is whatever the umask yields. Fix point identified: synchronous `os.Chmod(target, 0o600)` immediately after `net.Listen` succeeds (see Architecture Patterns → Pattern 1 and Code Examples → SEC-01). Test pattern available: existing `grpc_lifecycle_test.go` helper + `t.TempDir()`. |
| SEC-02 | gRPC server validates incoming requests — existing nil checks preserved/enhanced | All three RPC request messages are **empty protos** (`ClearCacheRequest`, `ReloadListRequest`, `emptypb.Empty`) — verified in `proto/api/bgp-dns.proto`, so "empty-field" validation can only mean nil-request / nil-stream rejection. Centralization point identified: interceptors on `grpc.NewServer()` in `cli.Serve()` (`cmd/bgp-dnsd/cli/cache.go:60`); existing per-handler nil checks documented (§2). **No protobuf regeneration needed** — no `.proto` change. |
| SEC-03 | DNS handler filters query types: only A, AAAA, HTTPS served for listed domains (NXDOMAIN emerges naturally for nonexistent names) | Today **every qtype** reaches `c.resolve()` for listed domains — the registered handlers (`cache.go:152`, `cache.go:177`) ignore `r.Question[0].Qtype`. Insertion point identified: a small guard shared by `registerOn` and `registerRegexOn` in `internal/dns/cache.go` (NOT in the mux, NOT in `resolvers.proxyQuery`, which must keep forwarding arbitrary qtypes for unlisted domains). Exact constants: `dns.TypeA`, `dns.TypeAAAA`, `dns.TypeHTTPS` (all exist in miekg/dns v1.1.67 — see Standard Stack). Related observation: prefetch set `requestTypes` is `{TypeA, TypeHTTPS}` (`cache.go:140`) — AAAA coverage implications discussed in Open Questions. |
| SEC-04 | BGP peer config struct includes optional `AuthPassword` passed to GoBGP | Config struct `bgpNeighbor` (`internal/config/bgp.go:7-12`) gains one field; wiring point is the `bgpapi.PeerConf` literal in `NewBgp()` (`internal/bgp/main.go:98-100`). Exact GoBGP API confirmed against the pinned module: `PeerConf.AuthPassword string` (field 1) in `github.com/osrg/gobgp/v3@v3.37.0/api/gobgp.pb.go:6088` — it is the **TCP-MD5 session authentication key** (RFC 2385 / RFC 5925), applied by GoBGP via `setTCPMD5SigSockopt` when non-empty. Empty string = disabled = current behavior preserved. |

## Architectural Responsibility Map

Single-binary Go daemon; all capabilities live in the **daemon process (bgp-dnsd)**. "Tiers" here are subsystems, not network tiers.

| Capability | Primary Tier (subsystem) | Secondary Tier | Rationale |
|------------|--------------------------|----------------|-----------|
| SEC-01 socket 0600 | Control plane — `cmd/bgp-dnsd/cli` (gRPC listener owner) | — | The daemon's `cli.Serve()` owns `net.Listen` for the socket; only the creator can reliably set the mode. |
| SEC-02 request validation | Control plane — `cmd/bgp-dnsd/cli` (gRPC server + handlers) | — | Validation belongs at the service boundary that receives requests (`CacheCliServiceImpl` + server interceptors). |
| SEC-03 qtype filter | Data plane — `internal/dns` (registered-domain handler path) | `internal/dns/serveMux.go` (unchanged, routes only) | The listed-domain handlers are the seam that separates "our domains" from "proxied domains"; the mux is a generic router and must stay filter-free. |
| SEC-04 BGP auth | Routing plane — `internal/bgp` (GoBGP wrapper) + `internal/config` (schema) | — | `NewBgp()` is the sole producer of `bgpapi.Peer` specs; config package only deserializes. |
| No browser/CDN/database tiers exist | — | — | Pure daemon; no client-side code touched. |

## Standard Stack

**Key finding: Phase 5 introduces ZERO new external dependencies.** Every requirement is satisfiable with the standard library plus packages already pinned in `go.mod`:

### Core (already pinned — versions verified in `go.mod` AND in the local module cache, i.e. byte-identical to what compiles today)

| Library | Version (go.mod) | Purpose in Phase 5 | Why Standard |
|---------|------------------|--------------------|--------------|
| `os` (stdlib) | — | `os.Chmod(socketPath, 0o600)` for SEC-01 | Only reliable way to force mode regardless of umask. |
| `google.golang.org/grpc` | v1.73.0 `[VERIFIED: go.mod:15]` | Interceptors (`ChainUnaryInterceptor`, `ChainStreamInterceptor`) + `codes`/`status` for SEC-02 | Already imported in `cmd/bgp-dnsd/cli/cache.go:15-17`. |
| `github.com/miekg/dns` | v1.1.67 `[VERIFIED: go.mod:8]` | `dns.TypeA` / `dns.TypeAAAA` / `dns.TypeHTTPS`, `dns.RcodeRefused`, `(*Msg).SetReply` for SEC-03 | The project's DNS engine; constants confirmed present in the pinned source (ztypes.go). |
| `github.com/osrg/gobgp/v3` | v3.37.0 `[VERIFIED: go.mod:9]` | `bgpapi.PeerConf.AuthPassword` for SEC-04 | The project's BGP engine; field confirmed in pinned source (gobgp.pb.go:6088). |
| `github.com/stretchr/testify` | v1.10.0 `[VERIFIED: go.mod:14]` | New tests | Already used by every test package. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `gopkg.in/yaml.v3` (indirect via viper) | v3.0.1 `[VERIFIED: go.mod:48]` | Only if a config-parse test writes YAML by hand; preferred pattern is reusing the existing temp-file + `config.Init` approach from `internal/config/dns_timeout_test.go` | Prefer the existing test pattern; do not add yaml as a direct dependency. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.Chmod` post-`Listen` | `syscall.Umask(0o077)` at process start | Umask affects every file/socket the daemon creates (logs, future features); too blunt. Explicit chmod is the targeted control. |
| Per-handler nil checks only | gRPC interceptors in `cli.Serve()` | Interceptors centralize; per-handler checks stay as belt-and-braces at the same boundary (both kept, one helper). |
| Hard-coded qtype allowlist | Config-driven `Dns.List.QueryTypes` | Requirements fix the set (A/AAAA/HTTPS). Config surface is churn without a second consumer; revisit only if a user demands it. |
| GoBGP `AuthPassword` (TCP-MD5) | ChaCha20-Poly1305 signatures (RFC 9972) | GoBGP v3.37 exposes no API for the newer scheme; upgrading GoBGP major is out of scope. Accepted trade-off, documented in Security Domain. |

**Installation:** none — `go mod tidy` after the change should show no diff (plan must assert this as a verification step).

## Package Legitimacy Audit

No new external packages are installed by this phase, so the legitimacy gate is vacuously satisfied.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none introduced) | — | — | — | — | n/a | n/a |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

*Verification performed instead of registry checks: every symbol this phase depends on was read directly from the **pinned** module sources in the local Go module cache (`/home/agent/go/pkg/mod/github.com/osrg/gobgp/v3@v3.37.0/...`, `/home/agent/go/pkg/mod/github.com/miekg/dns@v1.1.67/...`), which are bit-for-bit what the build consumes.*

## Architecture Patterns

### System Architecture Diagram (request flows relevant to Phase 5)

```
                        ┌─────────────────────────────── bgp-dnsd process ───────────────────────────────┐
                        │                                                                                │
 CLI (bgp-dnsctl)       │  ┌─────────────┐      ┌──────────────────────────────┐      ┌──────────────┐  │
 ──gRPC over Unix       │  │ cli.Serve() │─────▶│ grpc.Server (+ interceptors) │─────▶│CacheCliSvc   │  │
 socket (SEC-01: 0600)  │  │ net.Listen  │      │ SEC-02: nil-request/stream   │      │handlers:      │  │
 ──────────────────────▶│  │ (SEC-01 fix)│      │ validation here              │      │ListCache/Clear│  │
                        │  └─────────────┘      └──────────────────────────────┘      │Cache/Reload    │  │
                        │                                                             └──────┬─────────┘  │
 External DNS clients   │  ┌─────────────┐      ┌──────────────────────────────┐            │           │
 ──UDP DNS query        │  │ dns.Server  │─────▶│ regexServeMux.ServeDNS       │            ▼           │
 ──────────────────────▶│  │ (UDP listen)│      │ exact>wildcard>regex>catchAll│      package-level    │
                        │  └─────────────┘      └──────┬─────────────────┬─────┘      _dns global →    │
                        │                              │ listed domain   │ unlisted                 cache.dump/clear/load
                        │                    ┌─────────▼─────────┐  ┌────▼──────────────────┐          │
                        │                    │ registered handler│  │ resolvers.proxyQuery  │          │
                        │                    │ cache.resolve()   │  │ (forwards ANY qtype — │          │
                        │                    │ SEC-03: A/AAAA/   │  │  must stay unfiltered)│          │
                        │                    │ HTTPS only + RREFUSED│                           │        │
                        │                    └────────┬──────────┘  └─────────────────────────┘         │
                        │                             │ upsert (A/HTTPS prefetch via                   │
                        │                             ▼ requestTypes list, cache.go:140)               │
                        │                    cache entry ──▶ bgp.Advance(ip)/bgp.Withdraw(ip)          │
                        │                                        │ refcount per IP                     │
                        │                    ┌───────────────────▼──────────────────────────┐          │
                        │                    │ internal/bgp (GoBGP wrapper)                │          │
                        │                    │ NewBgp(): StartBgp + AddPeer per cfg.Bgp.Peers       │
                        │                    │ SEC-04: PeerConf.AuthPassword = peer.AuthPassword     │
                        │                    │ (TCP-MD5 key → GoBGP setTCPMD5SigSockopt)             │
                        │                    └───────────────────┬──────────────────────────┘          │
                        └────────────────────────────────────────┼────────────────────────────────────┘
                                                                 │ BGP TCP :8179 (now optionally MD5-authenticated)
                                                           BGP peer(s)
```

Flow notes:
- **SEC-01 & SEC-02** are both inside the top control-plane box: same function (`cli.Serve`), adjacent lines. They can be planned as one task.
- **SEC-03** is in the data plane; the catch-all path (unlisted domains) deliberately remains open to all qtypes — that is the resolver's product behavior, not a hole.
- **SEC-04** only changes how the daemon talks to its BGP peers; no client-side change exists in this codebase for BGP.

### Recommended Project Structure (files touched — no new packages)

```
cmd/bgp-dnsd/cli/
├── cache.go                  # MODIFY: Serve() → chmod after Listen (SEC-01);
│                             #         grpc.NewServer(ChainUnaryInterceptor(...), ChainStreamInterceptor(...)) (SEC-02)
├── validation_test.go        # NEW: interceptor tests (nil request via direct calls, valid passthrough)
└── socket_test.go            # NEW: unix socket created with 0600 (or fold into grpc_lifecycle_test.go)

internal/dns/
├── cache.go                  # MODIFY: shared qtype guard used by registerOn()/registerRegexOn() (SEC-03)
└── qtype_filter_test.go      # NEW: table-driven allow/deny via mux+fake writer or e2e UDP client

internal/config/
├── bgp.go                    # MODIFY: bgpNeighbor += AuthPassword string (SEC-04)
└── bgp_auth_test.go          # NEW: Init() parses optional AuthPassword; absent → ""

internal/bgp/
├── main.go                   # MODIFY: PeerConf literal gains AuthPassword: peer.AuthPassword (SEC-04)
└── (optional) small test asserting the config→spec mapping if extracted; else covered by config test + code review

appsettings.yml               # MODIFY (docs/sample): show optional `AuthPassword:` under a peer
README.md / QWEN.md           # MODIFY (docs): document all four hardening measures
```

### Pattern 1: "Explicit mode after create" for filesystem security boundaries

**What:** After creating a socket/file whose permissions matter, synchronously normalize the mode with `os.Chmod` instead of relying on umask.

**When to use:** Any IPC endpoint where non-owner access must be impossible and deployment-time umask is unknown (systemd units commonly run with umask 0022 → sockets would be 0755 without this).

**Example:**
```go
// Source: standard Go pattern; applied at cmd/bgp-dnsd/cli/cache.go in Serve()
if _listener, e = net.Listen(proto, target); e != nil {
    ...existing fatal handling...
}
// SEC-01: force owner-only access regardless of the process umask.
if proto == "unix" {
    if ce := os.Chmod(target, 0o600); ce != nil {
        _app.L().Fatal().Err(ce).Msgf("CLI Service: Failed to chmod %s", target)
        return ce
    }
}
```

**Why safe here (and NOT elsewhere):** `cli.Serve()` does three things before any dialer could race us: (a) removes a stale socket file, (b) `net.Listen` creates the fresh one, (c) `grpcServer.Serve(l)` runs in a goroutine started *after* the chmod. The window where the socket exists at 0755 is a few microseconds inside the daemon, before it has logged readiness. This is the standard trade-off for AF_UNIX permissions and is acceptable for a local admin channel.

### Pattern 2: gRPC server-side interceptors as the single validation gate

**What:** Register unary + stream server interceptors on `grpc.NewServer(...)` so every RPC passes through one place before reaching handlers. Nil-message protection cannot be done purely in an interceptor for streams/unaries without peeking, so the idiomatic split is: **interceptors** provide uniform logging/hook point + reject obviously-bad contexts, **handler nil-checks** (already present) remain authoritative for message shape.

**When to use:** When a service has >1 RPC and you want guarantees that survive adding future RPCs.

**Example:**
```go
// Source: google.golang.org/grpc server interceptors (v1.73 API)
logValidation := func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
    h grpc.UnaryHandler) (any, error) {
    // Uniform structured audit log per RPC (zerolog, per project convention)
    _app.L().Debug().Str("rpc", info.FullMethod()).Msg("gRPC call received")
    return h(ctx, req)
}
grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(logValidation))
```

**Planning guidance:** Because all three existing request types are empty protos, the interceptor adds *logging + a structural gate*, while the substantive SEC-02 work is: keep the three existing nil checks (`cache.go:80`, `105`, `121`), add a missing symmetric check style consistency, and extend coverage with tests that call `ClearCache(nil)`, `ReloadList(nil)`, `ListCacheEntries(nil, nil)` directly against `&CacheCliServiceImpl{}`. That satisfies "rejects requests with empty/nil fields (existing checks enhanced)" given the proto reality. Flag this interpretation explicitly in the plan so the verifier knows what was delivered.

### Pattern 3: Precedence-aware response codes for DNS filtering

**What:** For denied query types on *listed* domains, answer with a proper DNS response (SetReply + RcodeRefused) rather than silence, so recursive clients don't hang retrying. NXDOMAIN falls out naturally: an allowed qtype for a name upstream doesn't have will get NOERROR/NXDOMAIN from upstream and pass through unchanged — do not special-case it.

**When to use:** Any selective qtype service.

**Example:**
```go
// Source: miekg/dns v1.1.67 pinned source — defaults.go:15 SetReply(request *Msg) *Msg,
//         types.go:134 `RcodeRefused = 5`, ztypes.go TypeA/TypeAAAA/TypeHTTPS — all VERIFIED this session
var servedQtypes = map[uint16]bool{dns.TypeA: true, dns.TypeAAAA: true, dns.TypeHTTPS: true}

func (c *cache) handleListed(w dns.ResponseWriter, r *dns.Msg) {
    if len(r.Question) == 0 || !servedQtypes[r.Question[0].Qtype] {
        m := new(dns.Msg)
        m.SetReply(r)
        m.Rcode = dns.RcodeRefused
        if e := w.WriteMsg(m); e != nil {
            c.L().Error().Err(e).Msg("Failed to write REFUSED reply")
        }
        return
    }
    c.resolve(w, r, true)
}
```

**Placement detail (read before planning):** today both call sites are inline closures:
- `registerOn` (`cache.go:152`): `c.mux.HandleFunc(...)` handler calls `c.resolve(rw, m, true)` — registered for every plain domain line in the list file (via `load()` → `registerOn(line, tempMux, false)` at `cache.go:258`).
- `registerRegexOn` (`cache.go:177`): same shape for regex lines.

The minimal-change implementation extracts the closure body into the guarded helper above and points both registration sites at it. The prefetch loop (`cache.go:154-159`) is untouched. Zero-question messages hitting these handlers are answered REFUSED by the guard, which matches the mux contract (mux only routes zero-question to catch-all when nothing else matched — a listed-domain query that somehow carries no question is malformed, refusing is correct).

### Anti-Patterns to Avoid

- **Filtering in `resolvers.proxyQuery`:** would break the legitimate "unlisted domain → forward anything" resolver behavior (the whole reason this daemon is useful as a transparent forwarder).
- **Filtering inside `regexServeMux`:** the mux is a generic router tested independently (serveMux_test.go); policy there couples routing to content and breaks the Phase 1 design (spike finding: mux stays dumb, policy lives in handlers).
- **Setting socket mode via umask:** nondeterministic across systemd/init environments; also affects future files.
- **Regenerating protobufs for nothing:** no `.proto` change is needed; running `make pb` without changes risks churn. Do not plan it.
- **Panic on optional field absence:** `AuthPassword` omitted in YAML must mean "no auth" (empty string), never a startup error.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| BGP session authentication | Custom TCP-MD5 HMAC in the daemon | GoBGP `PeerConf.AuthPassword` (internally `setTCPMD5SigSockopt`) | RFC-correct edge handling (socket-option scope, per-peer key, passive mode) already implemented and maintained upstream. |
| gRPC transport/security framing | Manual credential checks over the socket | OS file permissions (0600) + interceptors | Out-of-scope decision already recorded in REQUIREMENTS.md; socket perms are the trust boundary. |
| DNS message construction for refusals | Byte-level wire formatting | `(*dns.Msg).SetReply` + `m.Rcode = dns.RcodeRefused` | Correct ID/flag plumbing, compression off by default for replies. |
| YAML parsing of the new peer field | ad-hoc text parsing | viper unmarshal into the extended `bgpNeighbor` struct (existing decode hooks untouched) | Already wired; a plain `string` field needs no hook. |
| Socket permission enforcement at runtime | In-band ACL checks per connection | `os.Chmod` once at creation | Every-connection UID checks add a syscall per connect and still don't beat a well-set mode bit. |

**Key insight:** every item in this phase is a one-line-to-small-function integration with an existing vendor API. The risk is not algorithmic; it's *placement* (wrong seam), *regression* (breaking catch-all forwarding or prefetch), and *verification gap* (none of these behaviors have automated tests yet — see Validation Architecture).

## Runtime State Inventory

Phase 5 is a feature phase, not a rename/refactor/migration, so the full five-category inventory does not apply. Explicitly checked anyway because two categories could bite:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no database; DNS cache is in-memory only (gcache), keyed by `fqdn:qtype`. Adding AAAA to the served set creates *new* cache keys but requires no migration. | none |
| Live service config | Existing deployments' `appsettings.yml` may lack `AuthPassword` — that is the designed default (off). No external service (n8n/Datadog/etc.) references this project. | none (documentation only) |
| OS-registered state | Daemon socket path is fixed (`/run/bgp-dnsd/bgp-dnsd.sock` or `/run/user/<uid>/bgp-dnsd.sock` via `app.DefaultTarget()`). Mode change affects **already-running instances only at next start**; no OS registration embeds the old mode. Operators must restart the daemon to pick up 0600. | note in rollout/verification step: restart required |
| Secrets/env vars | `AuthPassword` is read from the YAML config file, not env. Plan should warn in README that config files containing passwords need the same file-mode discipline as credentials (documented practice, not code). | documentation |
| Build artifacts | None affected — no binary names/paths change; Goreleaser config untouched. | none |

## Common Pitfalls

### Pitfall 1: umask defeats the 0600 requirement
**What goes wrong:** Test passes locally (umask 0077) but production systemd (umask 0022) leaves the socket world-readable — requirement silently unmet.
**Why it happens:** `net.Listen` creates the socket honoring the process umask; nobody sets a mode.
**How to avoid:** Unconditional `os.Chmod(target, 0o600)` immediately after successful Listen (Pattern 1), plus the test asserts the mode with `os.Stat` — which forces correctness independent of the runner's umask.
**Warning signs:** CI green, field deploy socket shows `-rwxr-xr-x@`.

### Pitfall 2: Stale-socket removal races the previous instance
**What goes wrong:** Current `Serve()` does `os.Stat` → `os.Remove` → `Listen`. If a slow client holds an old socket open during a fast restart, removal + rebind is fine, but a *second* concurrent daemon would bind the new socket while the old one still serves — not a Phase 5 bug, but the chmod must happen on the *new* socket each start, not cached.
**How to avoid:** Chmod inside `Serve()` every invocation (it is — pattern above). Do not move chmod into an init-time helper that skips re-listen paths.
**Warning signs:** N/A (defensive).

### Pitfall 3: Filtering in the catch-all kills the forwarder
**What goes wrong:** Planner puts the qtype check in `proxyQuery` or in the mux; unlisted-domain TXT/SRV/MX queries stop resolving; users report "my resolver broke".
**Why it happens:** "Filter query types" reads globally but the requirement is scoped *"for listed domains"* — the seam is the registered-handler closure, not the transport.
**How to avoid:** The guard wraps `c.resolve` invocations made by `registerOn`/`registerRegexOn` only. Add a regression test: unlisted domain TXT query still forwards (e2e UDP harness exists — `dns_e2e_test.go`).
**Warning signs:** E2E forward test fails after change.

### Pitfall 4: Prefetch set vs served set divergence (silent)
**What goes wrong:** `requestTypes = []uint16{dns.TypeA, dns.TypeHTTPS}` drives background lookups at registration (`cache.go:140`, loop at `156`). After SEC-03, AAAA is *served on demand* but never prefetched/announced — BGP won't announce AAAA (by design: IPv4-only announcer per QWEN.md), so this is consistent — but the asymmetry is easy to "fix" wrongly later.
**How to avoid:** Document in the plan that `requestTypes` (announcement/prefetch scope) ≠ served-qtype set (client-facing scope); keep them distinct variables, don't unify them.
**Warning signs:** someone later makes AAAA announce to BGP; the daemon announces IPv6 routes though `_v4Family` is the only enabled AFI/SAFI → routes dropped by GoBGP.

### Pitfall 5: gRPC serializes nil → empty message
**What goes wrong:** Tests call the client with `nil` expecting `InvalidArgument`, get `FailedPrecondition` (or success) because the gRPC layer sends an empty message instead. Existing test file documents this trap in comments (`grpc_lifecycle_test.go:64-74`).
**How to avoid:** Test nil-rejection by calling the service methods directly on `&CacheCliServiceImpl{}` (exactly what `TestGRPC_ReloadList_NilRequest` does). Only then exercise real-client paths with populated messages.
**Warning signs:** flaky/wrong expected code assertions.

### Pitfall 6: Config tag mismatch already exists ("Addressess")
**What goes wrong:** The `bgpNeighbor.Address` field carries yaml/json tags `"Addressess"` (typo) yet works because `net.TCPAddr` subfields parse positionally-ish via nested map keys `Ip`/`Port` — actually viper matches the *outer* key case-insensitively… **do not copy this typo** when adding `AuthPassword`; give it clean tags `yaml:"AuthPassword" json:"AuthPassword"`.
**Why it happens:** historical typo predates Phase 5; fixing it now would break existing configs — leave it, just mirror correct tagging for the new field.
**How to avoid:** New test loads a YAML with `AuthPassword` set and asserts the parsed value; catches any tag mistake immediately.
**Warning signs:** config test fails with empty password.

### Pitfall 7: TCP-MD5 is legacy crypto — set expectations
**What goes wrong:** Reviewer treats `AuthPassword` as modern TLS-grade auth; or conversely, implementer assumes GoBGP negotiates something newer.
**How to avoid:** Document plainly: this implements BGP session authentication via TCP MD5 signatures (RFC 5925 option / RFC 2385 lineage), the scheme GoBGP v3 supports. Both sides must share the key. Note ChaCha20-Poly1305 (RFC 9972) requires a GoBGP upgrade beyond v3.37 API surface [ASSUMED — verify if user asks; not needed to deliver SEC-04].
**Warning signs:** n/a — documentation concern.

### Pitfall 8: Tests that dial the real default socket path
**What goes wrong:** A socket-mode test hardcodes `/run/bgp-dnsd/bgp-dnsd.sock` (needs root, collides with a running daemon) instead of using a temp path. Existing tests sidestep all of this by listening on `127.0.0.1:0` TCP (`grpc_lifecycle_test.go:23`).
**How to avoid:** For the SEC-01 test, drive `cli.Serve()` with `_app.Flags.Target = "unix://" + filepath.Join(t.TempDir(), "test.sock")`. This exercises the *exact* production branch (the `strings.HasPrefix(target, "unix://")` path in `Serve`) without root or collisions.
**Warning signs:** test fails on CI containers where `/run` layout differs.

## Code Examples

Verified patterns from pinned sources (all line numbers below are from the local module cache of the **pinned versions**, read this session):

### SEC-04 — Exact GoBGP v3 auth API (gobgp/v3@v3.37.0)

The field, verbatim from `api/gobgp.pb.go`:
```go
// /home/agent/go/pkg/mod/github.com/osrg/gobgp/v3@v3.37.0/api/gobgp.pb.go:6086-6092
type PeerConf struct {
	AuthPassword         string                 // bytes,1,opt,name=auth_password
	...
	NeighborAddress      string                 // bytes,4,opt,name=neighbor_address
	PeerAsn              uint32                 // varint,5,opt,name=peer_asn
```
`GetAuthPassword()` getter at line 6139. Semantics verified in the server implementation:
```go
// pkg/server/grpc_server.go:683 (AddPeer path): pconf.Config.AuthPassword = a.Conf.AuthPassword
// pkg/server/fsm.go:511:                      password := fsm.pConf.Config.AuthPassword
// pkg/server/server.go:3261-3262:             if c.Config.AuthPassword != "" { setTCPMD5SigSockopt(l, addr, ...) }
// also applied per-peer at server.go:3331, 3394, 3458
```
Conclusion: **non-empty `AuthPassword` ⇒ GoBGP enables TCP-MD5 signature verification for that peer**; empty ⇒ disabled. No other flag is required — there is no separate "enabled" bool in the v3 AddPeer path for this purpose `[VERIFIED: gobgp/v3@v3.37.0 source]`.

Wiring change in `internal/bgp/main.go` (`NewBgp`, inside the `for _, peer := range cfg.Bgp.Peers` loop at line 83):
```go
// current literal (main.go:98-100):
Conf: &bgpapi.PeerConf{
    NeighborAddress: peer.Address.IP.String(),
    PeerAsn:         peer.Asn,
},
// becomes:
Conf: &bgpapi.PeerConf{
    NeighborAddress: peer.Address.IP.String(),
    PeerAsn:         peer.Asn,
    AuthPassword:    peer.AuthPassword,   // "" when unset → auth stays off
},
```

### SEC-04 — Config struct extension (internal/config/bgp.go)

Current struct, verbatim (`internal/config/bgp.go:7-12`):
```go
type bgpNeighbor struct {
	Asn         uint32      `yaml:"Asn" json:"Asn"`
	Address     net.TCPAddr `yaml:"Addressess" json:"Addressess"` // pre-existing typo — DO NOT "fix" (breaks configs)
	Multihop    bool        `yaml:"Multihop" json:"Multihop"`
	PassiveMode bool        `yaml:"PassiveMode" json:"PassiveMode"`
}
```
Add one field — viper/unmarshal needs no decode-hook for plain strings (`internal/config/main.go:38-64` hooks only special-case `net.IP`, zerolog level, `time.Duration`):
```go
type bgpNeighbor struct {
	Asn          uint32      `yaml:"Asn" json:"Asn"`
	Address      net.TCPAddr `yaml:"Addressess" json:"Addressess"`
	Multihop     bool        `yaml:"Multihop" json:"Multihop"`
	PassiveMode  bool        `yaml:"PassiveMode" json:"PassiveMode"`
	AuthPassword string      `yaml:"AuthPassword" json:"AuthPassword"`
}
```
YAML sample addition (appsettings.yml under a peer entry):
```yaml
Bgp:
  Peers:
    - Asn: 65530
      Address:
        Ip: 192.168.151.44
        Port: 179
      AuthPassword: secret-key    # optional; absent or empty = no TCP-MD5 auth
```

### SEC-01 — Socket mode test skeleton (cmd/bgp-dnsd/cli)

Follows the established helper style in `grpc_lifecycle_test.go`; the production code path is identical except Target points at a temp unix socket:
```go
func TestServe_UnixSocketMode0600(t *testing.T) {
    target := filepath.Join(t.TempDir(), "bgp-dns-test.sock")
    _app := app.New("bgp-dnsd", zerolog.WarnLevel)
    _app.Flags.Target = "unix://" + target

    require.NoError(t, Serve(_app, "/nonexistent.lst")) // listFile unused by Serve itself
    t.Cleanup(func() { _ = Shutdown(_app) })

    fi, err := os.Stat(target)
    require.NoError(t, err)
    assert.Equal(t, fs.FileMode(0o600), fi.Mode().Perm())
}
```
Notes:
- `Serve` stores `_listFile` but does not touch it until an RPC calls `dns.Load` — passing a dummy path is safe (existing lifecycle tests never call ReloadList through an initialized daemon).
- Package-level globals (`_listener`, `_cancel`, `_listFile`) mean these Serve-based tests **must not be parallelized** and should reset state; existing test files already run sequentially. If two new tests both call `Serve` in one package, add a small reset between them (or use `subtests` + fresh `t` scopes with manual cleanup).
- The CLI side needs no change: `grpc.NewClient("unix:///path", ...)` (bgp-dnsctl `commands/root.go:25`) is unaffected by socket mode as long as it runs as the owning user.

### SEC-03 — qtype filter unit test pattern (internal/dns)

Reuse `testResponseWriter` and `newTestMsg` from `serveMux_test.go:16-38` (same package — no export issues):
```go
func TestServedQTypes_ListedDomain(t *testing.T) {
    // build a cache exactly like setupE2EEnv does (fake upstream + registerOn doLookup=false)
    c := setupCacheForFilterTest(t) // wraps existing newCache + newResolvers helpers
    require.NoError(t, c.register("example.com."))

    for _, tc := range []struct{ qtype uint16; wantRefused bool }{
        {dns.TypeA, false}, {dns.TypeAAAA, false}, {dns.TypeHTTPS, false},
        {dns.TypeTXT, true}, {dns.TypeMX, true}, {dns.TypeNS, true},
        {dns.TypeANY, true},
    } {
        w := &testResponseWriter{}
        mux := newRegexServeMux()
        c.SetMux(mux)
        // re-register against a fresh mux, then route:
        require.NoError(t, c.registerOn("example.com.", mux, false))
        mux.ServeDNS(w, newTestMsg("example.com.", tc.qtype))
        got := w.msg.Rcode
        assert.Equal(t, tc.wantRefused && got == dns.RcodeRefused || !tc.wantRefused && got != dns.RcodeRefused,
            true, "qtype %v", tc.qtype)
    }
}
```
(Caveat: `registerOn` registers on `c.mux` internally in some shapes — the planner should confirm whether to call `c.registerOn(fqdn, mux, false)` directly or register first then swap `SetMux`; both registration entry points take a `targetMux` parameter specifically so tests can aim at an isolated mux.)

Plus one E2E regression using the existing UDP harness (`dns_e2e_test.go`): query an **unlisted** domain for TXT through the catch-all and assert a forwarded answer arrives — proves the forwarder stayed open.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Rely on process umask for socket perms | Explicit `os.Chmod` post-create | this phase | Deterministic 0600 regardless of systemd/supervisor umask |
| Unauthenticated BGP sessions | Optional TCP-MD5 via `PeerConf.AuthPassword` | this phase (feature-flagged by config presence) | Peers without matching key rejected at TCP layer |
| Any qtype served for listed domains | A/AAAA/HTTPS only (REFUSED otherwise) | this phase | Smaller attack/surface area; NXDOMAIN semantics preserved |
| Per-handler ad-hoc nil checks | Same + unified interceptor audit logging | this phase | Consistent rejection codes + observability |

**Deprecated/outdated:**
- `grpc.Dial` (used in `grpc_lifecycle_test.go`) is deprecated in grpc-go v1.63+ in favor of `grpc.NewClient` (which the CLI binary already uses, `bgp-dnsctl/commands/root.go:25`). Not a Phase 5 requirement; opportunistically switch *only* in newly added tests. Do not rewrite existing tests just for this.
- BGP session auth via TCP-MD5 itself is considered legacy (RFC 9972 ChaCha20-Poly1305 is the IETF successor) — accepted here because the pinned GoBGP v3.37 API only exposes MD5-style `AuthPassword` `[ASSUMED for the absence of newer-scheme knobs; the positive fact that AuthPassword exists and maps to setTCPMD5SigSockopt is VERIFIED]`.

## Project Constraints (from QWEN.md / AGENTS.md)

Directives the plan must honor (verified against current code this session):
1. **Logging**: zerolog structured — all new log lines go through `zerolog` via the injected logger (`Log.L()` / `log.L()`); no `fmt.Println`. Every new error path logs with `.Err(e)`.
2. **Error handling**: fatal errors panic/Fatal; recoverable errors logged and handled gracefully. The chmod failure in `Serve()` follows the existing Fatal-and-return-error shape used right above it in the same function.
3. **Concurrency**: BGP/DNS ops serialized via `internal/loop` single-threaded loop. Phase 5 adds **no new goroutines**; the DNS guard runs inline in the handler (already inside miekg/dns' per-packet handler context). Keep it that way — do not dispatch filtering work onto the loop.
4. **DI (Phase 3 complete)**: constructors `dns.NewDns(cfg, l, logger)` / `bgp.NewBgp(ctx, cfg, l, logger)` take explicit deps; the typed `config.ConfigKey{}` carries config in contexts. New behavior must flow through existing constructor params (cfg fields), **not** new globals. Backward-compat package funcs (`Serve`/`Shutdown`/`Load`/`ClearCache`/`DumpCache`, `Advance`/`Withdraw`) and the `_dns`/`_bgp` globals must keep working — Phase 2 tests depend on them.
5. **Protobufs managed with Buf** (`make pb`): do NOT regenerate unless a `.proto` actually changes — none changes in this phase.
6. **Before commit**: `go mod tidy` (assert zero diff) and `go generate ./...`.
7. **Testing conventions** (AGENTS.md): table-driven where appropriate, testify, `go test ./...` green.

## Validation Architecture

> Nyquist validation is ENABLED (`workflow.nyquist_validation: true` in `.planning/config.json`). Orchestrator creates VALIDATION.md from this section.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/stretchr/testify v1.10.0` (assert/require) — no third-party test framework |
| Config file | none (no pytest/jest config); Makefile target `make all` builds, `go test ./...` runs |
| Quick run command | `go test ./cmd/bgp-dnsd/... ./internal/dns/... ./internal/bgp/... ./internal/config/...` |
| Full suite command | `go vet ./... && go build ./... && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEC-01 | `cli.Serve` on a `unix://` target produces socket file with perm 0600 | unit (in-package, drives real `Serve` against `t.TempDir()` path) | `go test ./cmd/bgp-dnsd/cli/ -run TestServe_UnixSocketMode0600 -count=1` | ❌ Wave 0 (new `socket_test.go` or extend `grpc_lifecycle_test.go`) |
| SEC-02a | Nil request rejected with `codes.InvalidArgument` for all three RPCs | unit (direct method calls on `&CacheCliServiceImpl{}`) | `go test ./cmd/bgp-dnsd/cli/ -run 'TestGRPC_(ClearCache|ReloadList)_NilRequest|TestGRPC_ReloadList_NilRequest' -count=1` | Partial — `TestGRPC_ReloadList_NilRequest` exists (`grpc_lifecycle_test.go:117`); ClearCache-nil and ListCacheEntries-nil-stream cases are missing → Wave 0 |
| SEC-02b | Valid (empty-but-non-nil) requests pass validation and reach the business logic (returning FailedPrecondition when subsystems uninit — i.e., NOT InvalidArgument) | unit (lifecycle harness, non-nil messages) | `go test ./cmd/bgp-dnsd/cli/ -run TestGRPC_ServeShutdown -count=1` | ✅ exists (`grpc_lifecycle_test.go:42`) — extend with explicit "non-nil passes gate" assertions |
| SEC-03a | Listed-domain query with qtype ∈ {A, AAAA, HTTPS} resolves normally | unit/e2e (mux + fake writer, plus e2e UDP client) | `go test ./internal/dns/ -run 'TestServedQTypes' -count=1` | ❌ Wave 0 (new `qtype_filter_test.go`) |
| SEC-03b | Listed-domain query with qtype ∉ {A, AAAA, HTTPS} gets REFUSED (RcodeRefused=5) and NO upstream lookup is triggered | unit (assert refusal + upstream stub received zero queries for denied types) | same as SEC-03a | ❌ Wave 0 |
| SEC-03c | Unlisted-domain query of ANY qtype still forwards upstream (regression guard) | e2e (existing UDP fake-resolver harness) | `go test ./internal/dns/ -run 'TestCatchAllForward' -count=1` | ❌ Wave 0 (harness `dns_e2e_test.go:30` reusable) |
| SEC-03d | Allowed-qtype nonexistent name propagates upstream NXDOMAIN unmodified | e2e (fake resolver returns NXDOMAIN; assert passthrough) | `go test ./internal/dns/ -run 'TestNXDomainPassthrough' -count=1` | ❌ Wave 0 |
| SEC-04a | YAML with `AuthPassword` under a peer parses into `bgpNeighbor.AuthPassword`; absent → `""` | unit (temp YAML file + `config.Init`, mirroring `internal/config/dns_timeout_test.go` pattern) | `go test ./internal/config/ -run TestBgpAuthPasswordParsing -count=1` | ❌ Wave 0 (new `bgp_auth_test.go`) |
| SEC-04b | `NewBgp` passes the parsed value into `bgpapi.PeerConf.AuthPassword` | code-level verification — GoBGP server construction requires root/netlink, so full E2E in sandbox is impractical; cover via (i) extracting the per-peer spec assembly into a pure helper `buildPeerSpec(peer *config.bgpNeighbor) *bgpapi.Peer` if trivially separable, else (ii) targeted code review checkpoint asserting the literal contains the field. Recommend (i) ONLY if it doesn't disturb the DI shape; the honest fallback is (ii). | `go test ./internal/bgp/ -run TestBuildPeerSpec -count=1` (if (i)) | ❌ Wave 0 / checkpoint:human-verify (if ii) |
| Cross-cut | No dependency drift | build gate | `go mod tidy && git diff --exit-code go.mod go.sum` | n/a (verification step) |
| Cross-cut | Whole-repo sanity | full suite | `go vet ./... && go build ./... && go test ./...` | n/a |

### Sampling Rate
- **Per task commit:** quick run command above scoped to touched packages (< 30 s; all in-process, no network beyond loopback UDP/TCP on ephemeral ports).
- **Per wave merge:** full suite command.
- **Phase gate:** full suite green + `go mod tidy` no-op + every row in the map executed at least once before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `cmd/bgp-dnsd/cli/socket_test.go` — covers SEC-01 (+ SEC-02a additions may live alongside)
- [ ] `internal/dns/qtype_filter_test.go` — covers SEC-03a/b (unit), SEC-03c/d (reuse e2e helpers)
- [ ] `internal/config/bgp_auth_test.go` — covers SEC-04a (pattern source: `internal/config/dns_timeout_test.go`)
- [ ] Decide SEC-04b strategy (extract helper vs human-verify checkpoint) — flag explicitly in PLAN so verifier knows which evidence to demand
- [ ] No framework install needed — testing toolchain already present.

## Security Domain

> `security_enforcement` not disabled in config — section applies. Phase 5 IS the security phase, so controls here are the deliverables themselves.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | Yes (network perimeter: BGP peers) | GoBGP `PeerConf.AuthPassword` (TCP-MD5, RFC 2385/5925 lineage) — shared-secret session auth; gRPC channel deliberately exempted per REQUIREMENTS.md out-of-scope decision (local Unix socket, same-user operator) |
| V3 Session Management | Partial — BGP FSM sessions | GoBGP-managed hold timers (project sets HoldTime 240, `internal/bgp/main.go:106-109`); MD5 protects session establishment. gRPC has no persistent sessions (stateless RPC over admin socket). |
| V4 Access Control | Yes | Filesystem DAC: socket 0600 = owner-only; documented assumption that `bgp-dnsctl` runs as the daemon's owner (deployment note). No ACL feature needed on the control plane. |
| V5 Input Validation | Yes | (a) gRPC nil-request/stream rejection with `codes.InvalidArgument`; (b) DNS qtype allowlist at the listed-domain boundary (REFUSED otherwise); (c) config load-time validation already present for `Dns.Timeout` (`internal/config/main.go:70-77`) — extend the same spirit: reject nothing new in BGP section (empty password is valid), optionally log a Warn if `AuthPassword` set with an unexpected-empty peer address (defensive, low priority). |
| V6 Cryptography | Yes (constrained) | MD5 *as a TCP signature keyed by a high-entropy shared secret* — MD5 collision weaknesses don't apply to keyed-HMAC-like MD5 usage the way they apply to hash integrity, but it remains legacy; document threat model ("trustworthy peering fabric, defense-in-depth against accidental mispeering/rogue neighbor", not "adversarial internet"). No TLS in scope (both interfaces are LAN/trusted-fabric transport). |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Local unprivileged user talks to daemon control socket (cache clear, list reload) | Elevation of Privilege | SEC-01: 0600 socket; document that socket dir must be owner-writable only (parent dir perms noted in rollout docs) |
| Malformed/zero-question/adversarial qtype DNS floods at listed domains | Tampering / DoS | SEC-03: early REFUSED before any resolver/cache/BGP work is done; cheap deny path prevents upstream amplification via this daemon |
| Rogue/accidental BGP neighbor injecting routes | Spoofing | SEC-04: TCP-MD5 key per peer; non-matching neighbors fail session setup |
| gRPC reflection/enumeration by local users | Information Disclosure | Low: service has 3 benign management RPCs, no credentials exposed; 0600 limits enumerators to the owner. No action beyond SEC-01/02 (documented). |
| Secret in plaintext config file | Information Disclosure | Inherent to chosen design; README guidance: restrict `appsettings.yml` file mode, don't commit real keys (sample shows placeholder) |
| Upstream DNS spoofing / hijack | Spoofing | Out of phase scope (would need DNSSEC — future work; note as follow-up idea only, do NOT schedule) |

### ASVS cross-check note for planner
Categories V7 (configuration) and V9/V10 (errors/logging): ensure the new `Warn`/`Info` logs do not echo the password value — **never log `AuthPassword` content**; if logging peer config, mask it (e.g., `"auth": "set"/"unset"`). This belongs in a task action, not just here.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GoBGP v3.37 offers no configuration knob for RFC 9972 (ChaCha20-Poly1305) session auth; `AuthPassword`/MD5 is the only scheme reachable via `AddPeer` | State of the Art, Don't Hand-Roll | Low — only affects documentation phrasing; if a newer knob exists, user might prefer it (would require config/API extension discussion, not code risk) |
| A2 | Interpreting SEC-02 as "nil request/stream rejection + uniform interceptors + consistent status codes" is faithful to the intent of "rejects requests with empty/nil fields", given all three protos are empty messages | Phase Requirements, Pattern 2 | Medium — if the user expected richer proto fields validated, the phase would need a proto change (out of stated scope; would surface in discuss/plan review) |
| A3 | Responding REFUSED (not silence, not NXDOMAIN) to denied qtypes on listed domains matches product intent; the roadmap's "NXDOMAIN for listed domains" refers to names that don't exist upstream, which naturally passes through | Pattern 3, Common Pitfalls | Medium — alternate reading is to reply NXDOMAIN for denied qtypes (masks unsupportedness as nonexistence); both defensible; REFUSED is more honest and standard practice (e.g., unbound's serve-refused behavior). Plan should make the choice explicit in the task text |
| A4 | The daemon's DNS listening socket (`Dns.Listen`, UDP :5354) is not part of the 0600 requirement — SEC-01 names the *gRPC* socket only | User Constraints, Runtime State Inventory | Low — requirement wording is explicit about "gRPC Unix socket"; if the user wants the UDP bind hardened that's a different control (bind-address choice), already configurable |

## Open Questions

1. **Does the user want AAAA prefetched/announced?**
   - What we know: `requestTypes = {TypeA, TypeHTTPS}` (`cache.go:140`) — IPv6 answers are now served to clients but never prefetched or announced (and cannot be announced: only `_v4Family` enabled, matching QWEN.md's "announces resolved A records (IPv4)").
   - What's unclear: whether "support AAAA fully" was intended somewhere past the letter of SEC-03.
   - Recommendation: treat as OUT of scope (requirements say filter-to-A/AAAA/HTTPS, not announce-AAAA); record the asymmetry in the summary so it's a conscious known-state. If user says otherwise, that's a new requirement (AFI/SAFI change in `internal/bgp/main.go:121-124`).
2. **Intercepted stream RPCs (`ListCacheEntries`) — how deep does nil protection go?**
   - What we know: the existing stream handler nil-checks `stream` (the server-side object, effectively un-nilable via gRPC) but not its empty request message (trivially empty anyway).
   - What's unclear: whether "enhanced checks" demands per-message validation in streaming responses.
   - Recommendation: add symmetric nil checks + one streaming passthrough test; document the reasoning in the plan so the verifier accepts it.
3. **Where to put SEC-04b evidence (helper extraction vs code-review checkpoint)?**
   - What we know: GoBGP `NewBgpServer` in tests hits netlink privileges; existing `NewBgpSrvForTest` deliberately avoids constructing a real GoBGP server (`internal/bgp/main.go:218-236`).
   - What's unclear: preferred trade-off between touching production shape for testability vs accepting a human checkpoint.
   - Recommendation: prefer the small pure-helper extraction (`buildPeerSpec`) since it preserves DI shape and yields a genuine automated check; escalate to checkpoint only if the extraction fights the existing loop wiring.

## Environment Availability

Phase changes are code-only; runtime dependencies already satisfied by the project baseline. Verified this session:

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | satisfies go.mod `go 1.24.4` (toolchain present — project builds per completed phases 01-04) | — |
| Module cache (offline-safe) | compilation | ✓ | pinned modules present in `/home/agent/go/pkg/mod` (gobgp v3.37.0, miekg/dns v1.1.67, grpc v1.73.0 all verified on disk) | `GOFLAGS=-mod=mod` + network proxy if cache ever incomplete |
| Loopback UDP/TCP for tests | e2e/unit DNS + gRPC tests | ✓ | ephemeral-port binding on 127.0.0.1 (already exercised by Phase 2/4 suites) | — |
| Root/netlink | **NOT needed** | ✗ intentionally avoided | GoBGP server construction requires CAP_NET_ADMIN; `NewBgpSrvForTest` exists precisely to skip it | SEC-04b covered via helper/unit tests, no privileged execution planned |
| Real second BGP speaker | validating actual MD5 handshake | ✗ | no quagga/frr/bird in environment | Manual verification step for field rollout only (two daemons or frr with `bgp md5-signored`/password); automated gate relies on unit+config tests |

**Missing dependencies with no fallback:** none blocking.
**Missing dependencies with fallback:** real BGP interop test → deferred to human/manual field verification (documented in VALIDATION.md as manual-only with justification: requires CAP_NET_ADMIN + second BGP implementation).

## Sources

### Primary (HIGH confidence — read directly this session from pinned/local sources)
- `github.com/osrg/gobgp/v3@v3.37.0` (local module cache): `api/gobgp.pb.go:6086-6170` (PeerConf.AuthPassword field + getters), `pkg/server/grpc_server.go:683,818`, `pkg/server/fsm.go:511`, `pkg/server/server.go:3261-3458` (TCP-MD5 activation semantics)
- `github.com/miekg/dns@v1.1.67` (local module cache): `types.go:134` (`RcodeRefused = 5`), `defaults.go:15,33` (`SetReply`, `SetQuestion`), `ztypes.go:96-97,123` (TypeA/TypeAAAA/TypeHTTPS constants)
- Repository sources (all quoted with line refs in this doc): `cmd/bgp-dnsd/cli/cache.go`, `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go`, `internal/dns/{main,cache,serveMux,resolve,cacheEntry,loop,resolvers}.go`, `internal/config/{config,bgp,dns,main,test}.go`, `internal/bgp/main.go`, `internal/app/main.go`, `cmd/bgp-dns{d,ctl}/commands/root.go`, `proto/api/bgp-dns.proto`, `go.mod`, `appsettings.yml`, `.planning/config.json`, `.planning/{ROADMAP,REQUIREMENTS,STATE}.md`, `.opencode/skills/spike-findings-bgp-dns/SKILL.md`

### Secondary (MEDIUM confidence)
- GoBGP upstream docs conceptually corroborate `AuthPassword` = TCP-MD5 key; treated as redundant with primary source reads (no external fetch required for correctness)
- RFC 2385 / RFC 5925 / RFC 9972 for MD5-session-auth semantics and successor schemes `[CITED: rfc-editor.org — standard knowledge, not fetched this session]`

### Tertiary (LOW confidence)
- None material. All load-bearing claims trace to primary sources or are explicitly marked [ASSUMED] in the Assumptions Log.

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — zero new dependencies; every symbol verified in the pinned module cache that the build consumes.
- Architecture: HIGH — every insertion point identified to file:line against the current tree (post-Phase-3/4 state); concurrency constraints checked (no new goroutines).
- Pitfalls: HIGH — most pitfalls discovered by reading the actual failure-prone seams (umask, catch-all scope, nil-serialization, prefetch divergence, tag typo).
- Verification: HIGH for automated gates; MEDIUM for SEC-04b end-to-end MD5 handshake (requires privileged/field environment — flagged for manual step).

**Research date:** 2026-08-18
**Valid until:** ~30 days (pinned-dependency facts won't rot; repo facts rot only if another phase lands before Phase 5 planning)
**Shell availability note:** This research session had no general-purpose shell tool; all verification was performed statically via file reads/greps of the repository AND the local Go module cache, which for pinned-module claims is equivalent evidence to running `go doc`. No claim in this document depends on a command that could not have been run.
