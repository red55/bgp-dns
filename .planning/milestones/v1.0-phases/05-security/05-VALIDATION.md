---
phase: 5
slug: security
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-18
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `github.com/stretchr/testify v1.10.0` (assert/require) — no third-party test framework |
| **Config file** | none — existing infrastructure covers all phase requirements; Makefile target `make all` builds, `go test ./...` runs |
| **Quick run command** | `go test ./cmd/bgp-dnsd/... ./internal/dns/... ./internal/bgp/... ./internal/config/...` |
| **Full suite command** | `go vet ./... && go build ./... && go test ./...` |
| **Estimated runtime** | ~30 seconds (all in-process, no network beyond loopback UDP/TCP on ephemeral ports) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/bgp-dnsd/... ./internal/dns/... ./internal/bgp/... ./internal/config/...`
- **After every plan wave:** Run `go vet ./... && go build ./... && go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green + `go mod tidy` asserts zero diff
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

Task IDs are provisional — the planner assigns final IDs/plan numbers; rows key off requirement coverage.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-XX-01 | 0X | 1 | SEC-01 | T-5-EoP (local unprivileged user → daemon control socket) | `cli.Serve` on a `unix://` target produces socket file with perm 0600 | unit (in-package, real `Serve` against `t.TempDir()` path) | `go test ./cmd/bgp-dnsd/cli/ -run TestServe_UnixSocketMode0600 -count=1` | ❌ W0 | ⬜ pending |
| 05-XX-02 | 0X | 1 | SEC-02 | T-5-InfoDisc (gRPC enumeration/input) | Nil request rejected with `codes.InvalidArgument` for all three RPCs | unit (direct method calls on impl struct) | `go test ./cmd/bgp-dnsd/cli/ -run 'TestGRPC_(ClearCache\|ReloadList)_NilRequest' -count=1` | Partial — `TestGRPC_ReloadList_NilRequest` exists (`grpc_lifecycle_test.go:117`) | ⬜ pending |
| 05-XX-03 | 0X | 1 | SEC-02 | — | Valid (empty-but-non-nil) requests pass validation and reach business logic (NOT InvalidArgument) | unit (lifecycle harness, non-nil messages) | `go test ./cmd/bgp-dnsd/cli/ -run TestGRPC_ServeShutdown -count=1` | ✅ exists (`grpc_lifecycle_test.go:42`) — extend with explicit "non-nil passes gate" assertions | ⬜ pending |
| 05-XX-04 | 0X | 1 | SEC-03a | T-5-Tamper/DoS (adversarial qtype floods at listed domains) | Listed-domain query with qtype ∈ {A, AAAA, HTTPS} resolves normally | unit/e2e (mux + fake writer, plus e2e UDP client) | `go test ./internal/dns/ -run TestServedQTypes -count=1` | ❌ W0 | ⬜ pending |
| 05-XX-05 | 0X | 1 | SEC-03b | T-5-Tamper/DoS | Listed-domain query with qtype ∉ {A, AAAA, HTTPS} gets REFUSED (RcodeRefused=5); NO upstream lookup triggered | unit (refusal + assert upstream stub received zero queries for denied types) | `go test ./internal/dns/ -run 'TestServedQTypes|TestDeniedQType.*Refused' -count=1` | ❌ W0 | ⬜ pending |
| 05-XX-06 | 0X | 1 | SEC-03c | regression guard | Unlisted-domain query of ANY qtype still forwards upstream | e2e (existing UDP fake-resolver harness, `dns_e2e_test.go:30`) | `go test ./internal/dns/ -run TestCatchAllForward -count=1` | ❌ W0 | ⬜ pending |
| 05-XX-07 | 0X | 1 | SEC-03d | T-5-Spoof (upstream passthrough honesty) | Allowed-qtype nonexistent name propagates upstream NXDOMAIN unmodified | e2e (fake resolver returns NXDOMAIN; assert passthrough) | `go test ./internal/dns/ -run TestNXDomainPassthrough -count=1` | ❌ W0 | ⬜ pending |
| 05-XX-08 | 0X | 1 | SEC-04a | T-5-InfoDisc (secret in config) | YAML `AuthPassword` under a peer parses into `bgpNeighbor.AuthPassword`; absent → `""` | unit (temp YAML + `config.Init`, pattern: `internal/config/dns_timeout_test.go`) | `go test ./internal/config/ -run TestBgpAuthPasswordParsing -count=1` | ❌ W0 | ⬜ pending |
| 05-XX-09 | 0X | 1 | SEC-04b | T-5-Spoof (rogue BGP neighbor injecting routes) | `NewBgp` passes parsed value into `bgpapi.PeerConf.AuthPassword` (GoBGP activates TCP-MD5 when non-empty) | code-level — extract pure `buildPeerSpec(peer *config.bgpNeighbor)` helper if trivially separable (recommended), else human-verify checkpoint | `go test ./internal/bgp/ -run TestBuildPeerSpec -count=1` (if helper extracted) / checkpoint:human-verify (fallback) | ❌ W0 / checkpoint | ⬜ pending |
| Cross-cut | — | — | all | — | No dependency drift | build gate | `go mod tidy && git diff --exit-code go.mod go.sum` | n/a (verification step) | ⬜ pending |
| Cross-cut | — | — | all | — | Whole-repo sanity | full suite | `go vet ./... && go build ./... && go test ./...` | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/bgp-dnsd/cli/socket_test.go` — covers SEC-01 (+ SEC-02 additions may live alongside)
- [ ] `internal/dns/qtype_filter_test.go` — covers SEC-03a/b (unit), SEC-03c/d (reuse e2e helpers)
- [ ] `internal/config/bgp_auth_test.go` — covers SEC-04a (pattern source: `internal/config/dns_timeout_test.go`)
- [ ] Decide SEC-04b strategy (extract `buildPeerSpec` helper vs human-verify checkpoint) — flag explicitly in PLAN so verifier knows which evidence to demand
- [ ] No framework install needed — testing toolchain already present.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| (SEC-04b fallback ONLY) | SEC-04b | GoBGP server construction in tests hits netlink privileges (CAP_NET_ADMIN); full E2E impractical in sandbox | If the `buildPeerSpec` extraction fights the existing loop wiring: targeted code-review checkpoint asserting the `bgpapi.Peer` literal in `internal/bgp/main.go` contains `AuthPassword: <peer value>`; record checkpoint result in SUMMARY |

*If the helper extraction succeeds, all phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
