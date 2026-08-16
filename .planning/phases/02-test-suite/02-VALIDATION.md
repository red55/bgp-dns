---
phase: 02
slug: test-suite
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-14
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test with testify/assert+require |
| **Config file** | none — go test built-in |
| **Quick run command** | `go test ./internal/bgp/ ./internal/dns/ -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/bgp/ ./internal/dns/ -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | TEST-01 | — | BGP ref counting unit tests | unit | `go test ./internal/bgp/ -run TestBgpRefCounting -count=1` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | TEST-01 | — | Resolver failover unit tests | unit | `go test ./internal/dns/ -run TestResolverFailover -count=1` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 2 | TEST-02, TEST-03 | — | Cache eviction tests | unit | `go test ./internal/dns/ -run TestCache -count=1` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 2 | TEST-04 | — | gRPC CLI lifecycle tests | integration | `go test ./cmd/bgp-dnsd/cli/ -run TestGrpc -count=1` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 3 | TEST-05 | — | E2E DNS→cache→BGP verification | integration | `go test ./internal/dns/ -run TestE2E -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/bgp/bgp_test.go` — BGP reference counting tests (TEST-01)
- [ ] `internal/dns/resolvers_test.go` — Resolver failover tests (TEST-02)
- [ ] `internal/dns/cache_test.go` — Cache eviction tests (TEST-03, extends existing)
- [ ] `cmd/bgp-dnsd/cli/cli_test.go` — gRPC CLI lifecycle tests (TEST-04)
- [ ] `internal/dns/e2e_test.go` — End-to-end integration tests (TEST-05)

*Wave 0 installs test infrastructure: fake DNS server, loop.Operation() spy, gRPC ephemeral server*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| BGP path attribute correctness | TEST-01 | No real GoBGP peer available in tests | Verify Advance() called with correct IP, skip path attribute checks |
| Multi-domain IP sharing | TEST-01 | Requires concurrent goroutines | Run `go test -race ./internal/bgp/` to verify no data races |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
