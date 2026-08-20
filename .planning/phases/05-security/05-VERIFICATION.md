---
phase: 05-security
verified: 2026-08-20T04:40:55Z
status: passed
score: 13/13
behavior_unverified: 0
---

# Phase 5: Security — Verification Report

**Goal:** Harden the daemon's gRPC admin channel, enforce a DNS served-query-type policy for listed domains, and support optional BGP peer authentication (TCP-MD5).

## Goal Achievement

Every declared must-have truth is backed by a named, currently-passing test plus direct source/diff inspection. No truth is present-but-unverifiable, no artifact is missing/stubbed, no key link is unwired, and no blocker exists.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | [P1] After `cli.Serve()` binds a unix target, socket stat mode is exactly 0600 | ✓ VERIFIED | `TestServe_UnixSocketMode0600` (fresh) asserts `os.Stat(...).Mode().Perm()==0o600`; full `cmd/bgp-dnsd/cli` suite green under `-race` |
| 2 | [P1] Direct method calls with nil request/stream on all 3 RPCs return `codes.InvalidArgument` | ✓ VERIFIED | `TestGRPC_ClearCache_NilRequest`, `TestGRPC_ListCacheEntries_NilStream`, `TestGRPC_ReloadList_NilRequest` all PASS |
| 3 | [P1] Valid empty-but-non-nil request traverses the audit gate into business logic | ✓ VERIFIED | `TestGRPC_ServeShutdown` sends `&api.ClearCacheRequest{}` (non-nil) and asserts `codes.FailedPrecondition` (reached handler), never `InvalidArgument` |
| 4 | [P1] Pre-existing `TestGRPC_ReloadList_NilRequest` still passes unmodified in intent | ✓ VERIFIED | Unchanged test included in green run; reload nil-rejection intact |
| 5 | [P2] Listed-domain A/AAAA/HTTPS resolves normally via existing cache pipeline | ✓ VERIFIED | `TestServedQTypes`: allowed qtypes reach upstream (`countedFakeDNS` confirms hits) and resolve |
| 6 | [P2] Listed-domain other-qtype → `RcodeRefused` + empty Answer + ZERO upstream queries | ✓ VERIFIED | `TestDeniedQTypeRefused`: MX/TXT/CNAME/NS/PTR → `RcodeRefused`(5), empty Answer, zero upstream (per-qtype counted fake) |
| 7 | [P2] UNLISTED-domain ANY qtype still forwards upstream unchanged (catch-all guard) | ✓ VERIFIED | `TestCatchAllForward`: unlisted-domain MX transparently proxied through production catch-all wiring, no REFUSED |
| 8 | [P2] Upstream result for allowed-qtype on listed domain passes through unmodified by the guard | ✓ VERIFIED | `TestNXDomainPassthrough`: allowed qtypes hit upstream, reply rcode carried verbatim, guard never rewrites rcodes. (Client-visible silence for an upstream NXDOMAIN is a **pre-existing** cache-pipeline characteristic, documented in the plan as accepted state — orthogonal to this phase and outside its scope) |
| 9 | [P2] Prefetch set `{TypeA,TypeHTTPS}` (cache.go:140) byte-for-byte distinct from served allowlist | ✓ VERIFIED | `requestTypes` (L140) unchanged in phase diff; `servedQTypes` referenced only at handler seam — serving AAAA triggers no new prefetch/announce |
| 10 | [P3] YAML `AuthPassword` under a BGP peer parses into config struct with exact value preserved | ✓ VERIFIED | `TestInit_BgpAuthPassword`: present → exact round-trip |
| 11 | [P3] YAML without `AuthPassword` parses cleanly = `""` (feature-off default); absence never a startup error | ✓ VERIFIED | `TestInit_BgpAuthPassword`: absent → `""`, explicit-empty → `""`, no error |
| 12 | [P3] Per-peer GoBGP spec at `AddPeer` carries `Config.AuthPassword` == config value; empty → empty | ✓ VERIFIED | `TestBuildPeerSpec_AuthPassword`: `"k-123"` propagated into `Peer.Conf.AuthPassword`, unset stays empty, identity guard-rails (NeighborAddress/PeerAsn/Transport.LocalAddress/EbgpMultihop.Enabled) hold |
| 13 | [P3] `appsettings.yml` documents optional key; README notes TCP-MD5 semantics + secret hygiene | ✓ VERIFIED | `appsettings.yml:17` commented `#AuthPassword` sample (default OFF); README §BGP peer authentication documents RFC 2385/5925, opt-in per peer, `chmod 600`, never-logged |

**Score: 13/13 truths verified (0 present-behavior-unverified, 0 abstained)**

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `cmd/bgp-dnsd/cli/cache.go` | MODIFIED (chmod + interceptor chain) | EXISTS + SUBSTANTIVE | `os.Chmod` L63–67; `auditRPC` shared helper + `ChainUnary/StreamInterceptor` L78–91 |
| `cmd/bgp-dnsd/cli/socket_test.go` | NEW W0 | EXISTS + SUBSTANTIVE | `TestServe_UnixSocketMode0600` fresh + stale subtests, perm bits asserted |
| `cmd/bgp-dnsd/cli/grpc_lifecycle_test.go` | MODIFIED | EXISTS + SUBSTANTIVE | Nil-rejection legs on all 3 RPCs + `TestGRPC_ServeShutdown` gate-passthrough pin |
| `internal/dns/cache.go` | MODIFIED (shared qtype guard) | EXISTS + SUBSTANTIVE | `refuseIfNotServed` L152; wired at L184 (exact) + L212 (regex); `servedQTypes` L146 |
| `internal/dns/qtype_filter_test.go` | NEW W0 | EXISTS + SUBSTANTIVE | `TestServedQTypes` / `TestDeniedQTypeRefused` / `TestCatchAllForward` / `TestNXDomainPassthrough` |
| `internal/config/bgp.go` | MODIFIED (`bgpNeighbor` += `AuthPassword`) | EXISTS + SUBSTANTIVE | L12–14 `AuthPassword string` with dual `yaml`/`json` tags |
| `internal/config/bgp_auth_test.go` | NEW | EXISTS + SUBSTANTIVE | `TestInit_BgpAuthPassword` temp-file + `Init` pattern |
| `internal/bgp/main.go` | MODIFIED (`buildPeerSpec` extracted + `AuthPassword`) | EXISTS + SUBSTANTIVE | `buildPeerSpec` L58; `input.AuthPassword` L50 → `Conf.AuthPassword` L75; loop feed L156 |
| `internal/bgp/peer_spec_test.go` | NEW | EXISTS + SUBSTANTIVE | `TestBuildPeerSpec_AuthPassword` propagation + identity guard-rails |
| `appsettings.yml` | MODIFIED (sample peer key) | EXISTS + SUBSTANTIVE | L17 commented `#AuthPassword` sample (off by default) |
| `README.md` | MODIFIED (BGP auth note) | EXISTS + SUBSTANTIVE | §BGP peer authentication — RFC 2385/5925, opt-in, secret hygiene |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| [P1] `net.Listen` success | `os.Chmod(…,0o600)` | `proto == "unix"` guard, after listen, before `Serve` | WIRED | cache.go:54 listen → :63–67 chmod → :94 `go …Serve` |
| [P1] `grpc.NewServer` | both interceptors share ONE audit handler | `ChainUnaryInterceptor` + `ChainStreamInterceptor` over one `auditRPC` closure | WIRED | cache.go:78–91 — single `auditRPC` used by both chains at the sole `NewServer` site |
| [P1] handlers | nil-rejection stays authoritative | per-handler `if req/stream == nil` before business logic, not removed | WIRED | `ListCacheEntries` stream-nil, `ClearCache` req-nil, etc. — unchanged |
| [P2] listed-domain handlers | guard answers BEFORE `resolve` | `refuseIfNotServed` first line of `registerOn` AND `registerRegexOn` closures | WIRED | cache.go:184 (exact) + :212 (regex); Refused answered prior to `c.resolve` |
| [P2] served policy set | single definition | `servedQTypes` slice referenced only by handler seam | WIRED | cache.go:146 `var`, consumed at :153 via `containsQType` |
| [P3] existing per-peer spec literal | `buildPeerSpec` extraction | verbatim move, `AuthPassword` the only behavioral delta | WIRED | main.go:58 helper, :75 `Conf.AuthPassword` |
| [P3] `AddPeer` loop | `buildPeerSpec` call | one-to-one mapping incl. `AuthPassword` | WIRED | main.go:156 feeds `peer.AuthPassword` into input |
| [P3] config field | GoBGP spec | matching `yaml`/`json` tags + preserved `Addressess` typo | WIRED | config/bgp.go:9 `Addressess`, :14 `AuthPassword` (dual tags) |

### Requirements Coverage

| Requirement | Status | Blocking Issue |
|-------------|--------|----------------|
| SEC-01 | SATISFIED | — |
| SEC-02 | SATISFIED | — |
| SEC-03 | SATISFIED | — |
| SEC-04 | SATISFIED | — |

### Prohibited Behaviors (judgment-tier) — machine-proved

Each plan scoped the following prohibitions. Rather than treating these as soft human checkpoints, every one below is proven with deterministic source/diff/test evidence:

| Prohibition | Plans | Status | Evidence |
|-------------|-------|--------|----------|
| Never log `AuthPassword` / peer credential content | P1, P2, P3 | HONORED | grep for log sites referencing `peer\|neighbor\|authpass` across `internal/config` + `internal/bgp`: 0 matches (exit 1); `auditRPC` logs only rpc name + elapsed |
| No new goroutines (interceptors run inline) | P1 | HONORED | Production-code diff over `c0c31bc^..HEAD` adds zero `go X`; the only new `go …ActivateAndServe()` is a test-fixture fake DNS server (`qtype_filter_test.go:87`) |
| Do not use `syscall.Umask` for socket perms | P1 | HONORED | grep `Umask` across `cmd/bgp-dnsd/cli`: 0 matches; perms forced via `os.Chmod` |
| TCP listeners get no chmod change | P1 | HONORED | chmod strictly guarded by `if proto == "unix"` (cache.go:63) |
| No filtering added to `resolvers.proxyQuery` or `regexServeMux` (unlisted keep transparent forwarding) | P2 | HONORED | `git diff` over phase: `internal/dns/resolvers.go` untouched; `TestCatchAllForward` proves unlisted forwarding unchanged |
| Do not modify prefetch `requestTypes` (cache.go:140) or reference the served set there | P2 | HONORED | `requestTypes` (L140) byte-for-byte unchanged in diff; `servedQTypes` referenced only at the handler seam |
| No AAAA announce scheduled (announce stays IPv4-A-only) | P2 | HONORED | `resolvers.go` (announce path w/ A/AAAA gate, born pre-phase at e34e169/36d560d) untouched in phase diff |
| Omitted `AuthPassword` is NOT a validation/startup error | P3 | HONORED | `TestInit_BgpAuthPassword`: absent → `""` with no error |
| No new Go module dependencies and no `.proto` regeneration | P3 | HONORED | `git log --name-only` over phase: no `go.mod`/`go.sum`/`*.proto` changed |

### Anti-Patterns Found

None.

Scanned all 11 phase-modified files for `TBD`/`FIXME`/`XXX` (blocker), `TODO`/`HACK` (warning), placeholder content (blocker), and disabled/skipped tests — all greps returned 0 matches (exit 1). No circular tests detected (each phase test imports the SUT and asserts independently-derived expected values, e.g. counted-fake-upstream hit counts and exact qtype sets). Assertion strength is Behavioral/Value throughout (mode bits, rcode constants, propagated strings), not merely existence/type.

### Test Quality Audit

- **Disabled/skipped tests:** 0 across `socket_test.go`, `grpc_lifecycle_test.go`, `qtype_filter_test.go`, `bgp_auth_test.go`, `peer_spec_test.go`.
- **Circularity:** none — `countedFakeDNS` (in-test fake upstream) counts hits and asserts zero-upstream for refused qtypes; `buildPeerSpec`/config assertions compare against independent literals.
- **Assertion sufficiency:** requirements demanding specific values/behavior are asserted at Value/Behavioral level (perm `0o600`, `RcodeRefused`(5), `FailedPrecondition` vs `InvalidArgument`, exact `AuthPassword` round-trip).

## Human Verification Required

None — all 13 must-haves verified by named passing tests plus deterministic source/diff inspection. This is an infrastructure/security-hardening phase (gRPC admin channel, DNS qtype policy, BGP auth field) with no user-facing visual/realtime/performance surface. A live TCP-MD5 handshake against a physical router is outside the declared must-haves and is not runnable in this environment; bgp-dns's obligation — wiring the value into the GoBGP `Peer.Conf.AuthPassword` spec — is proven up to the GoBGP API boundary, where GoBGP itself enforces RFC 2385.

## Gaps Summary

**No gaps found.** All must-haves verified, no blockers, no missing/stubbed artifacts, no unwired links, no anti-patterns, no flagged-unverified prohibition, no human-required items outstanding.

## Verification Metadata

- **Approach:** goal-backward verification executed directly by the orchestrator. gsd-core's `verify.artifacts` / `verify.key-links` auto-verifiers returned empty sets in this build, so every truth, artifact, key link, and prohibition was checked manually against source (`read_file`), `git diff` over the phase commit range (`c0c31bc^..HEAD`), and passing test runs.
- **Must-haves source:** per-plan `frontmatter.get <plan> --field must_haves` (Option A) — authoritative plan-level truths/artifacts/key-links/prohibitions for 05-01, 05-02, 05-03.
- **Automated:** `go test ./... -count=1` → all packages `ok`, exit 0; `go vet ./...` clean; `go build ./...` clean; concurrent `-race` run during UAT green.
- **Human checks required:** 0
- **Duration:** ≈35m

---
*Verified:* 2026-08-20T04:40:55Z
*Verifier:* Qwen Code — GSD verify-phase (orchestrator)
