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
- ✓ Regex pattern support in domainlist (exact > wildcard > regex > catch-all priority) — v1.0
- ✓ Unit/integration tests for BGP ref-counting, DNS failover, cache eviction, gRPC lifecycle, E2E flow — v1.0 (82 test functions, -race green)
- ✓ Explicit dependency injection replacing global package-level state — v1.0
- ✓ Configurable DNS query timeout — v1.0
- ✓ O(n) map-based set difference with unit tests — v1.0
- ✓ Cancellable daemon context for all BGP operations — v1.0
- ✓ Structured Warn-level error logging for failed BGP Advance/Withdraw — v1.0
- ✓ gRPC admin socket forced to 0600 + unified nil-request/stream validation gate on all RPCs — v1.0
- ✓ Listed-domain qtype filter: serve A/AAAA/HTTPS, REFUSE everything else — v1.0
- ✓ Optional per-peer BGP TCP-MD5 session key (`AuthPassword`) wired into GoBGP spec, never logged — v1.0

### Active

- [ ] Multi-instance / HA support — now unblocked: DI + typed config key landed in v1.0, the original blocker is gone (scope decision for a future milestone)

### Out of Scope

- DNSSEC validation — out of scope for v1, not core to bgp-dns's purpose
- IPv6-only deployments — project targets IPv4 BGP advertisement
- ~~Multi-instance HA — global state architecture prevents this; not a current requirement~~ — constraint removed by v1.0 DI refactor; moved to Active

## Context

- **Brownfield project**: Started from an existing Go codebase; v1.0 shipped 2026-08-20 across 5 phases / 17 plans (see `.planning/MILESTONES.md` + `.planning/milestones/`)
- **Stack**: Go 1.24.4, miekg/dns v1.1.67, GoBGP v3.37.0 — zero new third-party dependencies added during v1.0 development
- **Post-v1.0 state**: ~6.6K LOC of Go; 82 test functions (unit + integration + E2E), full suite green under -race; every subsystem constructed via explicit DI constructors with a typed config context key
- **Known accepted states**: (a) serving AAAA on a listed domain performs no prefetch/BGP announce — BGP stays IPv4-A-only by design; (b) upstream NXDOMAIN replies are dropped by the pre-existing pipeline as silence (predates v1.0, documented in Phase 2 verification)
- **Production domains**: sample/my.lst contains production domains (cloudflare.com, google.com, github.com)

## Constraints

- **Linux only** — BGP server requires root/NET_ADMIN, netlink, /proc/self/status
- **Go 1.24.4** — toolchain specified in go.mod
- **Root privileges required** — BGP on port 179, DNS on port 5354, Unix socket in /run/
- **No DNSSEC** — not in scope for current project

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Use regexServeMux for domain matching | miekg/dns doesn't support regex natively; custom mux with exact > wildcard > regex > catch-all priority validated in spike 001 | ✓ Good (v1.0) |
| `regex:` prefix syntax for domainlist | Simpler to parse than `/pattern/` delimiters; explicit and unambiguous | ✓ Good (v1.0) |
| Fix global state before adding features | Testing impossible with package-level singletons; refactoring first reduces regression risk | ✓ Good (v1.0) — DI landed with zero behavioral regressions on an already-green test net |
| Force gRPC Unix socket to 0600 via `os.Chmod` (not `syscall.Umask`) | umask is process-wide and affects other files; explicit chmod scoped to the socket is surgical and race-free after `net.Listen` | ✓ Good (v1.0) |
| Unified validation gate = one shared interceptor logging rpc+elapsed only; nil-rejection stays in per-handler checks | A gRPC interceptor cannot distinguish a nil request (serialized as empty message); keeping authoritative nil checks in handlers avoids double-gating while giving every RPC one audit event | ✓ Good (v1.0) |
| BGP auth shipped as opt-in `AuthPassword` → GoBGP `Conf.AuthPassword` (TCP-MD5) | RFC 2385/5925 enforced by GoBGP at the peer connection; default-off preserves existing deployments; credential never logged (rendered set/unset only) | ✓ Good (v1.0) |

---
*Last updated: 2026-08-20 after v1.0 milestone*

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
