# Phase 04: Reliability — Research

**Date:** 2026-08-18
**Mode:** Orchestrator-inline (subagent spawn unavailable due to runtime ContextState gap)
**Scope:** Validate feasibility of LOCKED decisions D-04, D-05, D-10..D-13; gather concrete evidence for D-06 (open); identify risks and executor gotchas.

## Findings Summary

All four locked design areas are **feasible with no contradictions** discovered. Two minor implementation notes are flagged below (not blockers):

1. The current bare `dns.Exchange()` already has an implicit 2s cap (library default). Making it configurable at 3–30s (default 5s) is a *relaxation* for most configs — confirm this is intentional (it is, per REQUIREMENTS: operators need tuning headroom above the 2s floor).
2. GoBGP v3.37.0's `AddPath`/`DeletePath` do NOT consume their `ctx` parameter internally (only `ListPath` does, mid-iteration). Propagating `s.ctx` is therefore a pure correctness/plumbing improvement with zero observable behavior change today — exactly matching the phase's "no external behavior change" guardrail.

---

## Focus Area 1: miekg/dns Client Semantics (D-04 / RELIAB-01)

### Version
`github.com/miekg/dns v1.1.67` (go.mod direct dep)

### Current Behavior (no explicit timeout)

The sole query site is `internal/dns/resolvers.go:91`:
```go
if a, e := dns.Exchange(q, srv.addr.String()); e == nil && len(a.Answer) > 0 {
```

Package-level `Exchange` (client.go:78):
```go
func Exchange(m *Msg, a string) (r *Msg, err error) {
    client := Client{Net: "udp"}
    r, _, err = client.Exchange(m, a)
    return r, err
}
```

This constructs a **zero-valued Client** per call (only `Net` set). All timeout fields are 0, so the library falls back to its internal constant:
```go
// client.go:16
dnsTimeout = 2 * time.Second
```
Applied independently to dial, read, and write (client.go:84-107: `dialTimeout()`, `readTimeout()`, `writeTimeout()` each return `dnsTimeout` when no override is set).

**Effective current bound: 2s per resolver attempt** (dial + write + read each capped at 2s, worst-case ~6s total across three phases, though in practice the read dominates). Not configurable; not visible to operators.

### Client Concurrency Safety

A single `*dns.Client` instance is **safe for concurrent `Exchange` calls**. Evidence:
- `Client` struct fields are all plain values (durations, strings, maps) — no mutexes, no atomics, no channels in the struct definition (verified by grep: no `sync.Mutex`, `sync.RWMutex`, `atomic`, or channel fields in client.go struct).
- `Exchange` → `c.Dial(address)` allocates a **fresh `net.UDPConn`** per call (DialContext at client.go:108 creates a new `net.Dialer` + `conn`). No persistent connection pooling.
- `ExchangeWithConn` reads only immutable config fields (`c.UDPSize`, `c.writeTimeout()`, `c.readTimeout()`) and operates on the per-call connection.
- No mutation of receiver state in the hot path.

Therefore: building one `*dns.Client{Timeout: t}` per resolver in `newResolver` and sharing it across all concurrent queries for that resolver is correct and eliminates the per-call allocation overhead of the current package-level `Exchange`.

### Timeout Field Semantics (for the plan)

Setting `Timeout` on the client (per docs at client.go:55-58):
> "Timeout is a cumulative timeout for dial, write and read, defaults to 0 (disabled) — overrides DialTimeout, ReadTimeout, WriteTimeout when non-zero."

So a single `Timeout: 5*time.Second` gives a **total budget of 5s** for the entire exchange (not 5s each). This is stricter than setting individual Read/Write timeouts. For DNS-over-UDP (single send/receive), a cumulative timeout is the right semantic — it bounds the user-visible latency of one resolver attempt.

**Recommendation for plan:** Use `&dns.Client{Timeout: cfg.Timeout}` (cumulative). Do NOT set individual Dial/Read/Write timeouts separately.

### Threading the Timeout Parameter

Current call graph:
```
cmd/bgp-dnsd/main.go:48  →  dns.NewDns(cfg, ...)
                              └→ newResolvers(cfg.Dns.Resolvers, logger)   [resolvers.go:56]
                                  └→ setResolvers(addrs)                    [resolvers.go:63]
                                      └→ newResolver(a)                     [resolvers.go:42]
```

Test callers of `newResolvers`:
- `internal/dns/resolvers_test.go:34` (helper `newTestResolvers`)
- `internal/dns/dns_e2e_test.go:88, 148`

After change:
```
newResolvers(addrs []*net.UDPAddr, timeout time.Duration, logger *zerolog.Logger) *resolvers
```
- Production caller passes `cfg.Dns.Timeout` (from new flat field).
- Test callers pass `0` → `newResolver` applies default (5s) when `timeout == 0`.
- `setResolvers` gains the same param (internal, single caller).
- `newResolver(a *net.UDPAddr, timeout time.Duration)` builds `client: &dns.Client{Timeout: effective}`.

### Config Field Placement

`internal/config/dns.go` — `dnsCfg` gains:
```go
Timeout time.Duration `yaml:"Timeout" json:"Timeout"`
```
Flat sibling of `Cache`, `Listen`, `Resolvers`, `List`. Consistent with existing style (flat sub-fields, not nested structs).

**Validation** (in `config.Init` or a post-unmarshal hook): if `cfg.Dns.Timeout <= 0`, set default `5 * time.Second`; if outside `[3s, 30s]`, return error. Viper unmarshals YAML durations natively (`"5s"`, `"10"`, etc.).

---

## Focus Area 2: GoBGP v3 Context Cancellation (D-05 / RELIAB-03)

### Version
`github.com/osrg/gobgp/v3 v3.37.0`

### Where ctx Is Actually Consumed

| Method | ctx usage | Evidence |
|--------|-----------|----------|
| `AddPath(ctx, r)` (server.go:2331) | **NOT consumed.** Body delegates to `s.mgmtOperation(f, true)` which enqueues to `s.mgmtCh` (buffered chan, size 1) and blocks on response. The `ctx` param is present in the signature but never read. | grep: zero `ctx` references between signature and closing brace |
| `DeletePath(ctx, r)` (server.go:2358) | **NOT consumed.** Same `mgmtOperation` path. | ditto |
| `ListPath(ctx, r, fn)` (server.go:2850) | **Consumed.** Inside the destination-iteration callback, `select { case <-ctx.Done(): ... }` stops iteration early if ctx is cancelled mid-list. | server.go offset ~76 from func start: `case <-ctx.Done():` |
| `StartBgp(ctx, r)` | Used to seed background goroutine lifecycles. | Existing behavior unchanged by our edit |
| `StopBgp(ctx, r)` | Passed through but the shutdown sequence is driven by internal state, not ctx deadline. | Not affected |

### Behavioral Impact of Replacing `context.Background()` with `s.ctx`

- **Normal operation (ctx alive):** Zero difference. AddPath/DeletePath ignore ctx entirely. ListPath only checks `ctx.Done()` which fires only on cancellation.
- **During shutdown (daemon signals cancel):** Sequence in `bgpSrv.Shutdown(ctx)`:
  1. `s.bgp.StopBgp(ctx, ...)` — tears down BGP sessions (peers flap, RIB cleaned)
  2. `s.cancel()` — cancels `s.ctx` → `loop(ctx)` exits → no more operations dispatched
  3. `s.bgp.Stop()` — stops the GoBGP serve loop
  4. `s.wg.Wait()` — drains

  By step 2, the loop is exiting; no new `find()`/`add()`/`remove()` calls enter the pipeline. An in-flight `ListPath` (inside `find()`) could theoretically observe `ctx.Done()` and stop early — but since `StopBgp` already completed in step 1, the RIB is being torn down anyway. No user-visible impact.

- **Conclusion:** Pure plumbing improvement. No new failure modes. Safe for "no external behavior change" constraint.

### Implementation Confirmation

Exact sites to change (verified against source):
1. `internal/bgp/main.go` — `NewBgp` signature: add `ctx context.Context` as first param. Replace `context.WithCancel(context.Background())` with `context.WithCancel(ctx)`. Store child in `s.ctx`.
2. `internal/bgp/main.go` — `bgpSrv` struct: add `ctx context.Context` field (beside existing `cancel context.CancelFunc`).
3. `internal/bgp/bgp.go:39` (approx) — remove `//TODO: pass context` comment. Change `context.Background()` → `s.ctx` in `add`, `find`, `remove` (three sites).
4. `cmd/bgp-dnsd/main.go:83` — pass daemon ctx: `bgp.NewBgp(ctx, cfg, loop.NewLoop(1, log.L()), log.L())`.
5. `internal/bgp/main.go` — legacy shim `Serve(ctx)`: already receives ctx, currently discards it. Change `NewBgp(cfg, l, log.L())` → `NewBgp(ctx, cfg, l, log.L())`.
6. `internal/bgp/main.go` — `NewBgpSrvForTest(t)`: creates `ctx, cancel := context.WithCancel(context.Background())` internally (already does this at line ~178). Just store it: `srv.ctx = ctx`. **No signature change needed.**

---

## Focus Area 3: BGP Error Surfacing Inventory (D-06 / RELIAB-04)

### Current State (file:line, severity, content)

| # | Location | Current Log | Severity | Issue |
|---|----------|-------------|----------|-------|
| 1 | `internal/dns/cache.go:114` | `_ = bgp.Advance(arrived)` | **NONE — completely discarded** | Advance failures invisible to operator |
| 2 | `internal/dns/cache.go:115` | `_ = bgp.Withdraw(gone)` | **NONE — completely discarded** | Withdraw failures invisible at call site (logged internally at #3 but also lost here) |
| 3 | `internal/bgp/main.go:240` | `_bgp.L().Error().Err(e)` (inside `Withdraw` closure, after `remove()` fails) | Error | Too severe for transient RIB misses; no structured context (no prefix, no peer) |
| 4 | `internal/bgp/bgp.go` `add()` | Returns `fmt.Errorf("unable to add path: %v, %w", prefix, e)` | N/A (return value) | Caller (Advance closure) assigns to `e` but never logs it |
| 5 | `internal/bgp/bgp.go` `remove()` | Returns raw `DeletePath` error or `"prefix %s is not found"` | N/A (return value) | Same issue |

### Peer-IP Context Available

- **Local router ID:** `s.id net.IP` (bgpSrv field, from `cfg.Bgp.Id`)
- **Configured peers:** `cfg.Bgp.Peers []*peerCfg`, each with `Address *net.UDPAddr` (IP + Port) and `Asn uint32`
- Routes are announced to ALL configured peers simultaneously (GoBGP session handles fan-out). There is no per-peer routing decision in this codebase.

### Recommended Log Format (for planner decision)

Per ROADMAP SC-4: "Warn level with peer-IP context." Concrete proposal:

```go
// At cache.go:114 (or better, inside Advance/Withdraw themselves):
s.L().Warn().
    Str("op", "advance").
    Strs("ips", ips).
    Str("router_id", s.id.String()).
    Strs("peers", peerIPs). // extracted from cfg.Bgp.Peers
    Err(err).
    Msg("BGP advance failed")
```

Where `peerIPs` is computed once at construction (stored on bgpSrv or derived from cfg). Similarly for withdraw.

**Severity choice: Warn** (not Error) because:
- These are transient operational events (a momentary RIB inconsistency), not crashes.
- The system self-heals on next resolution cycle.
- Error level would pollute alerting for recoverable conditions.

**Planner freedom:** Exact field names, whether to log inside `Advance`/`Withdraw` (package-level functions in bgp package — have access to `s.L()`) vs. at the call site in cache.go, and whether to include the full peer list or just the count. Constraint: must be Warn, must include peer IP(s) and the affected prefix/IPs.

---

## Focus Area 4: Difference() Call-Site Contract (D-10..D-13 / RELIAB-02)

### Current Implementation

`internal/utils/main.go:3-27` — nested-loop O(n²) two-pass symmetric set difference:
- Pass 1: append all `slice1` elements not found in `slice2` (linear scan)
- Swap slices
- Pass 2: append all (former) `slice2` elements not found in (former) `slice1`
- Returns combined slice

### Production Call Sites (sole consumer)

`internal/dns/cache.go:113-115`:
```go
var gone = utils.Difference(prevIps, ips)     // in old, not in new → withdraw
var arrived = utils.Difference(ips, prevIps)  // in new, not in old → advance
_ = bgp.Advance(arrived)
_ = bgp.Withdraw(gone)
```

### Duplicate Analysis

- `prevIps` = `ce.Ip4s()` from previous DNS answer
- `ips` = `ce.Ip4s()` from current DNS answer
- `Ip4s()` (cacheEntry.go:73-87) iterates `answer.Answer` extracting `.A.String()` from A records and `.Hint` from HTTPS/SVCB IPv4 hints
- **Can duplicates occur?** In principle, a malformed DNS response could contain duplicate A records. In practice, well-behaved upstream resolvers (unbound, dnsmasq, public resolvers) do not. Even if they did:
  - Old code: each duplicate occurrence not matching the other set gets appended → duplicate entries in output
  - New code (D-12 dedup): collapsed to one
  - **Both call sites feed BGP operations that treat IPs as a set** (advance/withdraw by /32 prefix). Duplicates in the input to `Advance([]string)` would cause the ref-counter to increment twice for the same IP (idempotent at BGP level since `add` checks `c == 1`). Duplicates in `Withdraw` input would cause decrement twice (guarded by `c < 1` check). **Dedup is provably safe — no observable change for real-world inputs, strictly better for degenerate cases.**

### Test Coverage

- **No existing tests** for `utils.Difference()` (no `*_test.go` files in `internal/utils/`).
- The function is exercised indirectly by the E2E tests in `dns_e2e_test.go` (cache upsert → resolve → verify BGP state), but no unit test pins the exact output shape/order.

### Planner Note

The plan should include a **new unit test file** `internal/utils/main_test.go` with table-driven cases:
- Normal asymmetric sets
- Identical sets (empty result)
- Empty/nil inputs (both, one)
- Overlapping but not identical
- Duplicate entries within inputs (verify dedup behavior)
- Order preservation (first-half elements in original relative order, then second-half)

---

## Validation Architecture

### What Is Being Tested

| Fix | Observable Behavior to Verify |
|-----|------------------------------|
| RELIAB-01 (DNS timeout) | Config loads with `Dns.Timeout`; invalid values rejected; resolvers use bounded-timeout client; query to a black-hole resolver returns error within timeout |
| RELIAB-02 (O(n) Difference) | Output matches old O(n²) for all inputs; dedup behavior; performance (optional benchmark) |
| RELIAB-03 (BGP ctx) | `NewBgp` accepts ctx; cancelling parent ctx causes loop exit; no panic/deadlock on concurrent cancel+operation |
| RELIAB-04 (error logging) | Advance/Withdraw errors produce structured Warn log with peer IP; capturable via zerolog test writer |

### Stack

- **Language/Runtime:** Go 1.24.4
- **Test framework:** Standard `testing` package + `github.com/stretchr/testify v1.10.0` (assert/require)
- **Race detection:** `go test -race ./...` (mandatory gate)
- **DNS test infrastructure:** Existing `fakeDNSServer` pattern in `internal/dns/resolvers_test.go` (`newFakeDNSServer`, `startFakeServer`, `makeSuccessMsg`, `makeErrorResponse`) — reusable for timeout tests (add a handler that sleeps before responding, or a server on a non-routable address)
- **BGP test infrastructure:** Existing `NewBgpSrvForTest(t)` + `SetBgpForTest` pattern (no real BGP daemon needed for unit tests; GoBGP library runs in-process)
- **Config tests:** Extend `internal/config/test.go` `TestConfig()` with `Timeout: 5 * time.Second`; add validation test cases for range enforcement

### Test Data Strategy

- **Reused fixtures/helpers:**
  - `newFakeDNSServer(t, handler)` — localhost UDP, ephemeral port
  - `makeSuccessMsg(q)` / `makeErrorResponse(q)` — canned DNS responses
  - `mustResolveUDP(t, addr)` — address parsing helper
  - `initTestLogger()` — zerolog setup (defined in `cache_test.go`)
  - `config.TestConfig()` — minimal AppCfg
  - `bgp.NewBgpSrvForTest(t)` — in-process BGP service without network peers
  - `bgp.SetBgpForTest(srv)` / `bgp.GetBgpRefCounter()` — state inspection

- **New fixtures needed:**
  - Timeout test: a fake DNS handler that introduces artificial delay (`time.Sleep`) to exceed the configured client timeout; assert error type is deadline-exceeded
  - Black-hole resolver: `10.255.255.1:53` (non-routable in sandbox) with a very short timeout (3s minimum) — but this adds 3s to test duration. Alternative: use localhost with a listening socket that accepts but never responds (raw TCP trick over UDP not applicable; use a `net.ListenPacket` that reads but never writes)
  - BGP ctx test: create bgpSrv via `NewBgpSrvForTest`, get the internal ctx (or expose a test-only accessor), cancel it, verify loop exits (use a done channel or wg)
  - Error log capture: zerolog `io.Buffer` backend; parse JSON lines; assert `level=="warn"` and presence of `peers`/`router_id` fields

### Environment / Setup Facts for Executor

- All tests run on `127.0.0.1` with ephemeral ports — no external network needed
- `go test -race ./...` is the single verification command; baseline is 37 tests passing (Phase 2) + Phase 3 additions
- No CGO required; no Docker; no root privileges for tests (BGP tests use in-process GoBGP library, not a real daemon)
- GOMODCACHE is populated (deps downloaded); `go build ./...` works offline
- Build tags: none active; no platform-specific code
- The `sample/my.lst` domain list file is referenced by config but not needed for unit tests (tests construct their own FQDNs)

### Gate Commands

```bash
# Must pass before AND after each wave:
go build ./...
go vet ./...
go test -race ./...
```

---

## Risks & Gotchas for Planner/Executor

1. **Implicit 2s → explicit 5s default is a behavioral relaxation.** If any Phase 2 test relies on the 2s implicit timeout firing (unlikely given all fake servers respond instantly), it will still pass (longer timeout = more patience). No risk identified.

2. **`setResolvers` visibility.** It's unexported and called only from `newResolvers`. The timeout param threads cleanly. But if the plan adds `setResolvers` calls elsewhere (domain-list reload with different resolvers?), the param must follow. Check: `Dns.List.Resolvers` uses the SAME `resolvers` struct or a separate one? Looking at the code: `Dns.List.Resolvers` in config defines alternate upstream resolvers for listed domains, but the `Service` struct has only one `*resolvers` field (built from `cfg.Dns.Resolvers`). The list-specific resolvers appear to be a config structure not yet wired to a separate resolver instance (or they replace the primary set at load time). **Executor should verify this doesn't require a second `newResolvers` call site.**

3. **GoBGP `mgmtCh` buffer size is 1.** Operations queue serially. If the loop dispatches faster than the mgmt goroutine processes (possible under high churn), operations block on the channel send. This is existing behavior, not introduced by Phase 4, but worth noting: a cancelled ctx does NOT unblock a pending `mgmtCh <-` send (the channel send has no select-with-ctx). So if the BGP mgmt goroutine is stuck, a cancelled ctx won't save us. Mitigation: `StopBgp` in `Shutdown` runs BEFORE `s.cancel()`, giving the mgmt goroutine a chance to drain.

4. **`Difference()` has no existing unit tests.** The plan MUST add `internal/utils/main_test.go` as part of this phase (not deferred). Without it, the rewrite is untested at unit level and only caught by E2E integration.

5. **Viper duration parsing.** Viper/unmarshal handles `time.Duration` from YAML strings like `"5s"` or `"10"` (interpreted as seconds? — needs verification). If the YAML has `Timeout: 5` (integer), Viper's standard unmarshal to `time.Duration` treats it as nanoseconds. The config example should use `Timeout: 5s` explicitly. **Executor should add a config test that parses `Timeout: 5s` from YAML and asserts the duration is 5*time.Second.**

6. **appsettings.yml sample.** The reference config file at repo root should be updated to include `Timeout: 5s` under `Dns:` for discoverability.

---

## Source References

| File | Lines | Relevance |
|------|-------|-----------|
| `internal/dns/resolvers.go` | 91 (query), 42 (newResolver), 56 (newResolvers), 63 (setResolvers) | RELIAB-01 target |
| `internal/config/dns.go` | 15-19 (dnsCfg struct) | RELIAB-01 config field |
| `internal/config/main.go` | 21-78 (Init func) | RELIAB-01 validation hook |
| `internal/bgp/main.go` | 27-36 (struct), 42-120 (NewBgp), 124-130 (Serve), 133-145 (Shutdown), 150-170 (Advance), 220-255 (Withdraw), 175-190 (NewBgpSrvForTest) | RELIAB-03 + RELIAB-04 targets |
| `internal/bgp/bgp.go` | 39-50 (add), 52-80 (find), 82-100 (remove) | RELIAB-03 ctx sites |
| `cmd/bgp-dnsd/main.go` | 60-64 (ctx creation), 83 (NewBgp call) | RELIAB-03 wiring |
| `internal/utils/main.go` | 3-27 (Difference) | RELIAB-02 target |
| `internal/dns/cache.go` | 113-115 (call sites), 56-58 (eviction withdraw) | RELIAB-02 + RELIAB-04 |
| `internal/dns/main.go` | 44-60 (NewDns) | RELIAB-01 threading |
| `internal/dns/resolvers_test.go` | 20-90 (helpers) | Test infrastructure |
| miekg/dns@v1.1.67/client.go | 16 (dnsTimeout), 55-60 (fields), 78-82 (pkg Exchange), 163-170 (Client.Exchange), 205-206 (deadline logic) | RELIAB-01 library semantics |
| gobgp@v3.37.0/pkg/server/server.go | 315-320 (mgmtOperation), 2331 (AddPath), 2358 (DeletePath), 2850 (ListPath) | RELIAB-03 library semantics |

## RESEARCH COMPLETE
