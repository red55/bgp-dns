---
phase: 04-reliability
plan: "01"
subsystem: reliability
tags: [dns, timeout, miekg/dns, viper, config, set-difference, O(n)]

requires:
  - phase: 03-dependency-injection
    provides: logger injection pattern (call sites already take *zerolog.Logger), internal/dns test fixtures, config.TestConfig() harness
provides:
  - Flat Dns.Timeout config field (time.Duration, default 5s, enforced 3s-30s range at Init)
  - Per-resolver cached *dns.Client (cumulative Timeout only) threaded from config through NewDns -> newResolvers
  - Map-based O(n) utils.Difference preserving the asymmetric slice1\slice2 output shape
  - decodeHook string->time.Duration support for YAML duration fields
  - First unit tests for utils.Difference and the resolver timeout path
affects:
  - 04-02 (BGP context + error logging; disjoint files)
  - any phase touching DNS query path or cache upsert churn

actuals:
  tokens: ~5300   # chars/4 over realized diff
  tasks: 4
  commits: 1

tech-stack:
  added: []
  patterns:
    - per-resolver shared *dns.Client created once in newResolver (concurrency-safe; cumulative Timeout only, no Dial/Read/WriteTimeout)
    - config post-Unmarshal switch in Init(): normalize non-positive to default, then range-check with named-value error
    - decodeHook reflect.Kind()==String branch per target type (net.IP, zerolog.Level, time.Duration)
    - membership-map rewrite keeping Difference's exact public signature and result ordering

key-files:
  created:
    - internal/config/dns_timeout_test.go
    - internal/utils/main_test.go
  modified:
    - internal/config/dns.go
    - internal/config/main.go
    - internal/config/test.go
    - internal/dns/resolvers.go
    - internal/dns/main.go
    - internal/dns/resolvers_test.go
    - internal/dns/cache_test.go
    - internal/dns/dns_e2e_test.go
    - internal/utils/main.go
    - appsettings.yml

key-decisions:
  - "D-04 honored exactly: flat dnsCfg.Timeout (yaml 'Timeout'), default 5s when absent/non-positive, hard error outside 3-30s; ONE cumulative Client.Timeout per resolver"
  - "Difference semantics corrected to ASYMMETRIC slice1\\slice2 (see Deviations 1): CONTEXT.md D-11 described a two-pass symmetric shape, but the shipped loop ran once"
  - "Duplicate collapse within slice1 is safe: Advance gates on first ref (c==1), Withdraw on last (c<1) — duplicates in one call were always no-ops"
  - "Test callers pass literal 0 (= 5s default) per plan spec; production passes cfg.Dns.Timeout"

patterns-established:
  - "YAML durations must use Go literals ('5s'); bare integers land at ns scale and are rejected by the range check"
  - "Regression pin: E2E ref-counter suite asserts gone/arrived behavior end-to-end through upsert"

requirements-completed: [RELIAB-01, RELIAB-02]

coverage:
  - id: D1
    description: "Flat Dns.Timeout config with 5s default and 3s-30s range enforcement via real YAML Init"
    requirement: RELIAB-01
    verification:
      - kind: unit
        ref: "internal/config/dns_timeout_test.go#TestInit_DnsTimeout"
        status: pass
    human_judgment: false
  - id: D2
    description: "Per-resolver cached *dns.Client honoring Dns.Timeout; silent-upstream bounded-timeout regression test + fast-server control"
    requirement: RELIAB-01
    verification:
      - kind: unit
        ref: "internal/dns/resolvers_test.go#TestResolver_Timeout"
        status: pass
    human_judgment: false
  - id: D3
    description: "O(n) map-based utils.Difference, asymmetric contract pinned by first-ever unit tests incl. reference cross-check"
    requirement: RELIAB-02
    verification:
      - kind: unit
        ref: "internal/utils/main_test.go#TestDifference"
        status: pass
      - kind: e2e
        ref: "internal/dns/dns_e2e_test.go#TestE2E_DnsQueryToCacheToBgp"
        status: pass
    human_judgment: false

duration: ~90min
completed: 2026-08-18
status: complete
---

# Phase 04 Plan 01: DNS Timeout Config + O(n) Set Difference Summary

**DNS queries now run under an operator-configurable 5s-default cumulative timeout (one cached client per resolver instead of the untunable package-level Exchange), and utils.Difference runs in O(n) with its first unit tests.**

## Performance

- **Duration:** ~90 min (incl. root-causing the Difference contract deviation)
- **Tasks:** 4/4 completed and verified
- **Files modified:** 10 (+ 2 new test files)

## Accomplishments

- **RELIAB-01 (config):** `dnsCfg.Timeout time.Duration` (`yaml:"Timeout"`); `Init` defaults non-positive values to 5s and rejects out-of-range with `dns: timeout %v out of range (allowed: 3s-30s)` (boundaries accepted). `TestConfig()` sets 5s. Table-driven `TestInit_DnsTimeout` covers absent→5s, explicit 5s, 3s/30s OK, 2s/31s rejected — all through real YAML `Init`.
- **RELIAB-01 (resolvers):** `resolver` holds a shared concurrency-safe `*dns.Client{Net:"udp", Timeout: t}` built once in `newResolver(a, timeout)`; `newResolvers(c, timeout, logger)` / `setResolvers(c, timeout)` thread it; `query()` switched to `srv.client.Exchange(q, srv.addr.String())`. The package-level `dns.Exchange` (implicit untunable 2s library default per call) is off the hot path. `NewDns` passes `cfg.Dns.Timeout`. `appsettings.yml` documents `Timeout: 5s`.
- **RELIAB-02:** `utils.Difference` rewritten from O(n²) nested loops to a single map-based pass over slice1 (O(n+m)); same public signature and result ordering as before, duplicate entries collapsed (documented as intentional for BGP ref-counted /32 ops). First unit tests: table cases incl. fresh-entry and IP-change scenarios + hand-picked 20+16-element input cross-checked against an independent naive reference.
- **New regression test** `TestResolver_Timeout`: silent UDP listener must yield a query error well inside the bound (~3s deadline, asserted < 6s), and a healthy fake server under the same timeout still answers.

## Task Commits

All four tasks landed in one commit (executor session had no shell capability; orchestrator completed verification + fixes + commit):

1. **Task 1: Dns.Timeout config + validation** — included in feat(04) commit below
2. **Task 2: per-resolver cached *dns.Client** — included in feat(04) commit below
3. **Task 3: O(n) Difference + unit tests** — included in feat(04) commit below (with contract correction, see Deviations)
4. **Task 4: full regression gate** — `go build ./... && go vet ./... && go test -race ./...` ALL GREEN

## Files Created/Modified

- `internal/config/dns.go` — added `Timeout time.Duration` field to `dnsCfg`
- `internal/config/main.go` — post-Unmarshal default+range validation; decodeHook string→time.Duration branch; `time` import
- `internal/config/test.go` — `TestConfig()` sets `Timeout: 5*time.Second`
- `internal/config/dns_timeout_test.go` (new) — `TestInit_DnsTimeout`, table-driven over temp-file YAML fixtures via `Init(path)`
- `internal/dns/resolvers.go` — `resolver.client *dns.Client`; `newResolver(a, timeout)` builds `&dns.Client{Net:"udp", Timeout:eff}` (≤0 → 5s); threaded timeout params; `query()` uses `srv.client.Exchange`
- `internal/dns/main.go` — `NewDns` passes `cfg.Dns.Timeout` into `newResolvers`
- `internal/dns/resolvers_test.go` — helper updated to `newResolvers(udpAddrs, 0, &l)`; `time` import; new `TestResolver_Timeout` (silent listener + healthy-server control)
- `internal/dns/cache_test.go` — `r.setResolvers(nil, 0)`
- `internal/dns/dns_e2e_test.go` — both `newResolvers(...)` call sites pass `0`
- `internal/utils/main.go` — map-based O(n) asymmetric `Difference` with load-bearing doc comment
- `internal/utils/main_test.go` (new) — `TestDifference` table-driven + reference cross-check subtest
- `appsettings.yml` — `Timeout: 5s` under `Dns:`

## Decisions Made

- Followed plan code sketches except where reality disagreed (see Deviations 1–2); used the plan's exact error string format.
- Test callers pass literal `0` (= 5s default) per plan spec.
- Kept dedup-within-slice1 even though the old occurrence-preserving behavior was never observable downstream (Advance c==1 / Withdraw c<1 gates make duplicates no-ops).

## Deviations from Plan

### 1. Difference is asymmetric, not symmetric (correcting CONTEXT.md D-11 / plan Task 3)
- **Found during:** Task 4 gate — `TestE2E_DnsQueryToCacheToBgp` failed deterministically (empty BGP ref counter) after the planned "two ordered passes with per-half dedup" rewrite
- **Issue:** The pre-change implementation's `for i := 0; i < 1; i++` executes exactly ONCE; the swap at loop bottom never re-enters. The 'two-pass symmetric' descriptions in CONTEXT.md/RESEARCH.md/plan misread that. Concretely, for a fresh cache entry `gone = Difference(prevIps=[], ips)` became `[ips...]` instead of `nil`, so `upsert` withdrew every IP it had just advanced (refs net to zero → empty counter → E2E failure). Bisect proved the pristine HEAD green and isolated the delta to this function.
- **Fix:** Single map-based pass computing `slice1 \ slice2` (still O(n), order preserved, dedup kept); doc comment states the asymmetry is load-bearing for the gone/arrived pair; test suite pins both directions explicitly (`second empty returns all of slice1`, `first empty yields nothing`)
- **Verification:** full `-race` suite green including E2E ref-counter assertions (A+HTTPS → count 2)

### 2. decodeHook gains a string→time.Duration branch
- **Found during:** Task 4 gate — every YAML `"5s"` fixture failed decoding (`'Dns.Timeout' cannot parse value as 'time.Duration': strconv.ParseInt ... invalid syntax`); the reject-cases were passing only because they erred at parse, not at range check
- **Issue:** `decodeHook` handled net.IP and zerolog.Level strings but not durations; mapstructure weak-decodes duration strings via ParseInt and fails on Go literals
- **Fix:** Added the standard `reflect.TypeOf(time.Duration(0))` branch using `time.ParseDuration`, matching existing branch style
- **Verification:** `TestInit_DnsTimeout` fully green (explicit/boundary/rejected)

### 3. Executor session lacked shell capability
- Subagent produced code-complete work but could not run build/test gates or commit; orchestrator completed verification, the two corrective fixes above, and the commit.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Code AND verification complete for RELIAB-01/RELIAB-02; committed. Full baseline (Phases 1–3 suites + new tests) is `-race` clean.
- Suite runtime increased ~3s (TestResolver_Timeout silent-listener wait), within budget.
