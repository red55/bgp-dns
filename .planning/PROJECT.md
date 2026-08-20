# bgp-dns

## What This Is

bgp-dns is a DNS resolver daemon with BGP route advertisement. It maintains a domainlist file of FQDNs (and optionally regex patterns), resolves them via upstream DNS, caches the results, and advertises the resolved IPs via BGP to configured peers. A gRPC CLI (`bgp-dnsctl`) provides cache management and list reload capabilities.

## Core Value

Resolve domains from a configurable list and advertise their IPs via BGP — fast, correct route propagation with zero manual intervention.

## Requirements

### Validated

- ✓ Daemon listens on UDP :5354 and intercepts DNS queries for listed FQDNs — existing codebase
- ✓ Resolves listed domains via configurable upstream resolvers — existing codebase
- ✓ Caches DNS responses with TTL-based eviction — existing codebase
- ✓ Advertises resolved IPs via BGP (GoBGP) with reference-counted announcements — existing codebase
- ✓ File watcher triggers domain list reload on changes — existing codebase
- ✓ gRPC CLI (`bgp-dnsctl`) for cache list/clear and list reload — existing codebase
- ✓ Spike 001 validated: regexServeMux for wildcard/regex domain matching works correctly — spike-findings-bgp-dns
- ✓ gRPC admin socket forced to 0600 + unified nil-request/stream validation gate on all RPCs — Phase 5
- ✓ Optional per-peer BGP TCP-MD5 session key (`AuthPassword`) wired into GoBGP spec — Phase 5

### Active

- [ ] Add regex pattern support to domainlist file (exact > wildcard > regex > catch-all priority)
- [ ] Add unit/integration tests for BGP reference counting logic
- [ ] Add unit/integration tests for DNS resolver failover
- [ ] Replace global package-level state with explicit dependency injection
- [ ] Add DNS query timeout configuration
- [ ] Optimize O(n²) set difference to O(n) using maps
- [ ] Fix BGP context propagation (use cancellable context instead of context.Background())
- [ ] Add proper error handling for BGP operations (currently ignored)

### Out of Scope

- DNSSEC validation — out of scope for v1, not core to bgp-dns's purpose
- IPv6-only deployments — project targets IPv4 BGP advertisement
- Multi-instance HA — global state architecture prevents this; not a current requirement

## Context

- **Brownfield project**: Existing Go codebase with ~132 dependencies, Go 1.24.4, miekg/dns v1.1.67, GoBGP v3.37.0
- **Platform**: Linux-only (BGP requires NET_ADMIN, /proc/self/status for debugger detection)
- **Spike work**: regex domainlist interception spike (001) completed and validated — blueprint in `.opencode/skills/spike-findings-bgp-dns/`
- **Code quality**: No tests for BGP package, resolvers, config parsing; heavy global state; panic-on-init pattern
- **Production domains**: sample/my.lst contains production domains (cloudflare.com, google.com, github.com)

## Constraints

- **Linux only** — BGP server requires root/NET_ADMIN, netlink, /proc/self/status
- **Go 1.24.4** — toolchain specified in go.mod
- **Root privileges required** — BGP on port 179, DNS on port 5354, Unix socket in /run/
- **Single instance** — global state prevents multiple simultaneous daemons
- **No DNSSEC** — not in scope for current project

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Use regexServeMux for domain matching | miekg/dns doesn't support regex natively; custom mux with exact > wildcard > regex > catch-all priority validated in spike 001 | — Pending |
| `regex:` prefix syntax for domainlist | Simpler to parse than `/pattern/` delimiters; explicit and unambiguous | — Pending |
| Fix global state before adding features | Testing impossible with package-level singletons; refactoring first reduces regression risk | — Pending |
| Force gRPC Unix socket to 0600 via `os.Chmod` (not `syscall.Umask`) | umask is process-wide and affects other files; explicit chmod scoped to the socket is surgical and race-free after `net.Listen` | Verified Phase 5 |
| Unified validation gate = one shared interceptor logging rpc+elapsed only; nil-rejection stays in per-handler checks | A gRPC interceptor cannot distinguish a nil request (serialized as empty message); keeping authoritative nil checks in handlers avoids double-gating while giving every RPC one audit event | Verified Phase 5 |
| BGP auth shipped as opt-in `AuthPassword` → GoBGP `Conf.AuthPassword` (TCP-MD5) | RFC 2385/5925 enforced by GoBGP at the peer connection; default-off preserves existing deployments; credential never logged (rendered set/unset only) | Verified Phase 5 |

---
*Last updated: 2026-08-20 after Phase 5 (Security)*

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state (users, feedback, metrics)
