# bgp-dns Roadmap

**Created:** 2026-06-13
**Project:** bgp-dns — DNS resolver with BGP route advertisement
**Requirements:** 22 v1 requirements mapped across 5 phases

| Phase | Name | Requirements | Depends On |
|-------|------|--------------|------------|
| 1 | 2/3 | In Progress|  |
| 2 | 3/3 | Complete   | 2026-06-15 |
| 3 | Dependency Injection | REFACTOR-01–04 | Phase 2 |
| 4 | Reliability | RELIAB-01–04 | Phase 3 |
| 5 | Security | SEC-01–04 | Phase 3 |

---

## Phase 1: Regex Domainlist

**Goal:** Add regex pattern matching to domainlist file with exact > wildcard > regex > catch-all query priority.

**Requirements:** REGEX-01, REGEX-02, REGEX-03, REGEX-04, REGEX-05

**Success Criteria:**

1. Domainlist file accepts `regex:([a-z]+)\.internal\.corp` syntax and compiles pattern at load time
2. DNS queries for regex-matched domains resolve correctly and trigger BGP announcements
3. Existing exact-match entries continue to work without performance regression (<1ms query latency increase)
4. Invalid regex patterns produce clear error: `invalid regex "pattern": <reason>` on daemon start
5. Spike 001 regexServeMux blueprint implemented in `internal/dns/`

**SPIKE FINDINGS:** `.opencode/skills/spike-findings-bgp-dns/SKILL.md` — regexServeMux validated with 9/9 tests. Use `regex:` prefix syntax. Priority: exact > wildcard > regex > catch-all.

**Plans:** 2/3 plans executed

Plans:

- [x] 01-01-PLAN.md — Create regexServeMux struct with priority routing and 12 unit tests
- [x] 01-02-PLAN.md — Wire mux into cache and main.go, replace global dns.HandleFunc calls
- [ ] 01-03-PLAN.md — Integration tests for regex domainlist loading and routing

---

## Phase 2: Test Suite

**Goal:** Add unit and integration tests for critical untested packages (BGP, DNS resolvers, cache, gRPC).

**Requirements:** TEST-01, TEST-02, TEST-03, TEST-04, TEST-05

**Success Criteria:**

1. `internal/bgp/` has tests covering reference counting (advance/withdraw transitions, multi-domain IP sharing)
2. `internal/dns/resolvers.go` has tests for ring failover, health tracking, round-robin selection
3. `internal/dns/cache.go` has tests for generation-based eviction, TTL expiration, capacity limits
4. `cmd/bgp-dnsd/cli/` has integration tests for gRPC Serve/Shutdown lifecycle
5. End-to-end test: DNS query → cache hit → BGP announcement verification

**Notes:** Tests must work with existing global state — Phase 3 refactoring will improve testability.

**Plans:** 3/3 plans executed

Plans:

- [x] 02-01-PLAN.md — BGP reference counting tests (6 scenarios) + DNS resolver failover tests (4 scenarios)
- [x] 02-02-PLAN.md — Cache eviction tests (5 scenarios) + gRPC CLI lifecycle tests (6 scenarios)
- [x] 02-03-PLAN.md — E2E integration tests (3 scenarios: DNS→cache→BGP, multi-domain IP sharing, cache hit)

---

## Phase 3: Dependency Injection

**Goal:** Replace global package-level state with explicit dependency injection to enable testing and future multi-instance support.

**Requirements:** REFACTOR-01, REFACTOR-02, REFACTOR-03, REFACTOR-04

**Success Criteria:**

1. All subsystem constructors (`dns.New()`, `bgp.New()`, `fswatcher.New()`) accept explicit dependencies
2. Context config uses typed key (`type configKey struct{}`) instead of string `"cfg"`
3. Startup failures return errors instead of panicking; main.go handles errors gracefully
4. Dead code removed: commented error handling in resolvers.go, commented hashmap in bgp/main.go

**Dependencies:** Must pass Phase 2 tests before and after refactoring to verify behavioral equivalence.

**Plans:** 6/6 plans complete

Plans:

- [x] 03-01-PLAN.md — Foundation: typed configKey, Loop/logger injection, resolver logger injection, cache config field, test helper updates
- [x] 03-02-PLAN.md — DNS service struct, NewDns constructor, backward-compatible wrappers (Serve, Shutdown, Load, DumpCache, ClearCache)
- [x] 03-03-PLAN.md — BGP service NewBgp constructor, updated Serve/Shutdown/Advance/Withdraw wrappers, test helper updates
- [x] 03-04-PLAN.md — FSWatcher service struct with cfg field, NewFsWatcher constructor, updated Serve/Shutdown wrappers
- [x] 03-05-PLAN.md — main.go: typed configKey, constructor-based service creation, error handling, deferred cleanup
- [x] 03-06-PLAN.md — Dead code removal: commented hashmap in bgp/main.go, commented error handling in resolvers.go

---

## Phase 4: Reliability

**Goal:** Fix known reliability issues: DNS timeout, O(n²) set difference, BGP context propagation, error handling.

**Requirements:** RELIAB-01, RELIAB-02, RELIAB-03, RELIAB-04

**Success Criteria:**

1. DNS queries use `dns.Client{Timeout: 5s}` (configurable via `Dns.Timeouts.Query` in YAML)
2. `internal/utils/main.go` set difference uses map-based O(n) implementation
3. BGP operations use context from daemon lifecycle (cancellable on shutdown)
4. BGP Advance/Withdraw errors logged at Warn level with peer IP context

**Notes:** These are low-risk changes that don't alter external behavior but improve operational resilience.

---

## Phase 5: Security

**Goal:** Harden gRPC interface, add query validation, support BGP peer authentication.

**Requirements:** SEC-01, SEC-02, SEC-03, SEC-04

**Success Criteria:**

1. gRPC Unix socket created with mode 0600 (owner-only read/write)
2. DNS handler filters query types: only A, AAAA, HTTPS, NXDOMAIN for listed domains
3. BGP peer config struct includes optional `AuthPassword` field passed to GoBGP
4. gRPC server rejects requests with empty/nil fields (existing checks enhanced)

**Dependencies:** Phase 3 refactoring makes auth wiring cleaner (config struct changes).

**Plans:** 3 planned

Plans:

- [ ] 05-01-PLAN.md — Tracer: unix socket 0600 + unified gRPC validation gate (SEC-01, SEC-02)
- [ ] 05-02-PLAN.md — DNS qtype filter for listed domains: serve A/AAAA/HTTPS, REFUSED otherwise (SEC-03)
- [ ] 05-03-PLAN.md — Optional BGP peer AuthPassword → GoBGP spec + operator docs (SEC-04)

---

## Phase Dependencies

```
Phase 1 (Regex Domainlist) ──────────────────────────────────────────►
Phase 2 (Test Suite) ────────────────────────────────────────────────►
Phase 3 (Dependency Injection) ◄──────────────────────────────────────┘
    ├── Phase 4 (Reliability) ────────────────────────────────────────►
    └── Phase 5 (Security) ───────────────────────────────────────────►
```

- Phase 1 and Phase 2 can run in parallel (independent features)
- Phase 3 requires Phase 2 tests passing (safety net for refactoring)
- Phase 4 and Phase 5 can run in parallel after Phase 3

---

*Roadmap created: 2026-06-13*
*Last updated: 2026-06-13 after initial creation*
