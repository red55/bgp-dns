# Milestones

## v1.0 MVP (Shipped: 2026-08-20)

**Phases completed:** 5 phases, 17 plans, 37 tasks

**Key accomplishments:**

- Regex domainlist support: regexServeMux with exact > wildcard > regex > catch-all priority, load-time pattern compilation, atomic-swap reload, full unit coverage (REGEX-01–05)
- Test suite for critical paths: BGP reference counting, resolver-ring failover, cache eviction/TTL/LFU, gRPC Serve/Shutdown lifecycle, and DNS→cache→BGP end-to-end flows (TEST-01–05), all green under -race
- Dependency injection across every subsystem: constructors with explicit deps, typed config context key, panic-on-init replaced with returned errors, dead code removed (REFACTOR-01–04)
- Reliability fixes: operator-configurable DNS query timeout, O(n) set difference with unit tests, BGP on cancellable daemon context, structured Warn-level op-failure logging (RELIAB-01–04)
- Security hardening: gRPC unix socket forced to 0600, unified nil-request/stream validation gate on all RPCs with per-RPC audit log, listed-domain qtype filter (A/AAAA/HTTPS served, else REFUSED), opt-in per-peer BGP TCP-MD5 session key never logged (SEC-01–04)

---
