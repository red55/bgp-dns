# Phase 02: Test Suite - Context

**Gathered:** 2026-06-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Add unit and integration tests for critical untested packages: BGP reference counting, DNS resolver failover, cache eviction, and gRPC CLI lifecycle. Tests must work within the existing global-state architecture — no refactoring for DI yet (Phase 3).

</domain>

<decisions>
## Implementation Decisions

### BGP Reference Counting Tests
- **D-01:** Test reference counting logic in isolation by spying on `loop.Operation()` calls without invoking real GoBGP — the `ipRefCounter` map and atomic transitions are pure Go and testable
- **D-02:** Use `Operation(f, true)` for sync waits — tests block until the goroutine processes the operation
- **D-03:** Cover 5 scenarios: (1) single IP advance/withdraw, (2) multi-domain IP sharing, (3) concurrent advances, (4) withdraw non-existent IP, (5) mixed advance/withdraw sequence
- **D-04:** Verify both counter state transitions AND add()/remove() call timing — check ref counter AND explicitly confirm add() called on first ref, remove() called on last ref, NOT called in between
- **D-05:** Tests live in `internal/bgp/bgp_test.go` — same-package tests accessing private fields directly (consistent with existing `internal/dns/*_test.go` pattern)

### DNS Resolver Failover Tests
- **D-06:** Use DNS fake server with custom `dns.HandlerFunc` that returns controlled responses — share one server across tests with configurable per-query responses
- **D-07:** Cover 4 scenarios: (1) single resolver success, (2) failover to next resolver on failure, (3) recovery of failed resolver, (4) all resolvers fail
- **D-08:** Test both observable behavior AND ring position — verify which resolver was used, final `okay` state, and ring rotation
- **D-09:** Only test `query()` directly — skip `proxyQuery()` as it's a thin wrapper

### End-to-End Integration Tests
- **D-10:** Chain-of-responsibility approach — skip DNS server entirely, test `cache.resolve()` directly with constructed `dns.Msg` and mock `dns.ResponseWriter`
- **D-11:** Just verify `bgp.Advance()` was called with correct IPs — do not verify full BGP path attributes (AS path, next hop, prefix length)
- **D-12:** Use shared test server pattern — one DNS server instance for all E2E tests, reset state between tests

### Test Data and Infrastructure
- **D-13:** Use embedded strings (`strings.NewReader("example.com\ntest.com\n")`) for test data — no filesystem needed, consistent with existing `regex_integration_test.go` pattern
- **D-14:** Run race detection on BGP and loop tests only (`-race -run "TestBgp|TestLoop"`) — these packages have concurrent state; other packages don't need it

### gRPC CLI Tests
- **D-15:** Full lifecycle tests — start gRPC server, call ListCacheEntries/ClearCache/ReloadList RPCs, verify responses, shutdown server

### the agent's Discretion
- Test file naming convention (e.g., `bgp_test.go` vs `reference_counting_test.go`)
- Exact mock ResponseWriter implementation details
- How to handle `log.Log` initialization in tests (consistent with existing `initTestLogger()` pattern)
- Test helper functions placement (shared utils vs per-file helpers)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Requirements
- `.planning/ROADMAP.md` — Phase 2 goal, success criteria, and plan structure
- `.planning/REQUIREMENTS.md` — TEST-01 through TEST-05 requirements
- `.planning/phases/01-regex-domainlist/01-VERIFICATION.md` — Phase 1 verification (confirms existing test patterns and coverage)

### Existing Test Patterns
- `internal/dns/cache_test.go` — Existing cache test pattern with `newTestCache()` and `initTestLogger()` helpers
- `internal/dns/cacheEntry_test.go` — Existing cacheEntry unit tests
- `internal/dns/regex_integration_test.go` — Phase 1 integration tests (7 tests, uses testify/assert + testify/require)
- `internal/dns/serveMux_test.go` — Mux unit tests (12+ tests, uses `testResponseWriter` mock)
- `cmd/bgp-dnsd/cli/cache_test.go` — Existing gRPC CLI tests (2 tests, nil request handling)

### Target Packages (No Existing Tests)
- `internal/bgp/main.go` — BGP server with reference counting (`ipRefCounter`, `Advance()`, `Withdraw()`)
- `internal/bgp/bgp.go` — BGP path operations (`add()`, `find()`, `remove()`)
- `internal/dns/resolvers.go` — DNS resolver ring with failover (`query()`, `proxyQuery()`)
- `internal/dns/loop.go` — Cache refresh loop
- `internal/loop/main.go` — Controlled single-threaded operation loop
- `cmd/bgp-dnsd/cli/cache.go` — gRPC server (Serve, Shutdown, handler implementations)

### Supporting Infrastructure
- `internal/loop/main.go` — `Loop` struct with `Operation(f, ret)` method (buffered channel, single goroutine)
- `internal/config/main.go` — Config initialization (`config.Init()`)
- `internal/log/main.go` — Logging infrastructure (`log.Init()`, `log.L()`)

### Spike Findings
- `.opencode/skills/spike-findings-bgp-dns/SKILL.md` — regexServeMux blueprint (Phase 1, but test patterns may be relevant)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `testResponseWriter` in `internal/dns/serveMux_test.go` — mock `dns.ResponseWriter` already exists, can be reused for resolver and E2E tests
- `newTestCache()` in `internal/dns/cache_test.go` — test cache helper with logger initialization
- `initTestLogger()` in `internal/dns/cache_test.go` — standard logger setup pattern using `zerolog.WarnLevel`
- `github.com/stretchr/testify` — assertion library already in go.mod (v1.10.0), used consistently across existing tests

### Established Patterns
- **Same-package tests:** All existing tests are in the same package as the code they test (`package dns`, `package bgp`) — this grants access to private fields like `ipRefCounter`, `cache.gen`, `resolvers.rs`
- **Test helpers:** `newTestXxx(t)` functions that call `t.Helper()` and return initialized structs
- **Temp directories:** `t.TempDir()` used sparingly (only in cache_test.go for file operations); embedded strings preferred for simple test data
- **Logger initialization:** `initTestLogger()` called at test setup; zerolog level set to Warn to suppress noise

### Integration Points
- BGP tests need `loop.Operation()` — the controlled loop serializes operations; tests must interact with it to verify reference counting
- Resolver tests need `dns.Exchange()` — DNS fake server intercepts these calls
- gRPC tests need `BgpDnsServiceServer` interface — can be implemented via embed + method overrides

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond ROADMAP success criteria — standard Go testing approaches apply.

</specifics>

<deferred>
## Deferred Ideas

- **Test coverage metrics** — Tools like `go test -cover` reporting, coverage thresholds in CI — belongs in a dedicated "test infrastructure" phase if needed
- **Integration with GoBGP test harness** — Using GoBGP's own test infrastructure for deeper BGP protocol testing — out of scope for Phase 2
- **Property-based testing** — Using `github.com/bmizerany/property` or similar — agent's discretion if deemed valuable
- **Test fixtures/JSON files** — For complex DNS messages — embedded strings sufficient for now
- **CI integration for test execution** — Part of devops/infrastructure, not Phase 2 scope

</deferred>

---

*Phase: 02-test-suite*
*Context gathered: 2026-06-14*
