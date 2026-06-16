---
phase: 03
slug: dependency-injection
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-16
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (`go test`) + `testify/assert` |
| **Config file** | none (Go native) |
| **Quick run command** | `go test ./internal/dns ./internal/bgp ./internal/fswatcher -count=1 -short` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/dns ./internal/bgp ./internal/fswatcher -count=1 -short`
- **After every plan wave:** Run `go test ./... -count=1 -race`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 03-W0-01 | 03-01 | 0 | REFACTOR-02 | T-03-01 | Typed configKey prevents context collisions | unit | `go test ./internal/config -count=1` | ✅ existing | ⬜ pending |
| 03-W0-02 | 03-01 | 0 | REFACTOR-01 | T-03-02 | Loop accepts explicit logger parameter | unit | `go test ./internal/loop -count=1` | ✅ existing | ⬜ pending |
| 03-W0-03 | 03-01 | 0 | REFACTOR-01 | T-03-03 | Resolvers accepts explicit logger parameter | unit | `go test ./internal/dns -count=1` | ✅ existing | ⬜ pending |
| 03-01-01 | 03-01 | 1 | REFACTOR-01 | T-03-04 | dns.New() returns (Service, error) | integration | `go test ./internal/dns -count=1` | ✅ existing | ⬜ pending |
| 03-01-02 | 03-01 | 1 | REFACTOR-01 | T-03-05 | Wrapper functions delegate to struct | integration | `go test ./internal/dns -count=1` | ✅ existing | ⬜ pending |
| 03-02-01 | 03-02 | 2 | REFACTOR-01 | T-03-06 | bgp.New() returns (Service, error) | integration | `go test ./internal/bgp -count=1` | ✅ existing | ⬜ pending |
| 03-02-02 | 03-02 | 2 | REFACTOR-01 | T-03-07 | Wrapper functions delegate to struct | integration | `go test ./internal/bgp -count=1` | ✅ existing | ⬜ pending |
| 03-03-01 | 03-03 | 3 | REFACTOR-01 | T-03-08 | fswatcher.New() returns (Service, error) | integration | `go test ./internal/fswatcher -count=1` | ✅ existing | ⬜ pending |
| 03-04-01 | 03-04 | 4 | REFACTOR-03 | T-03-09 | main.go handles errors gracefully | integration | `go test ./cmd/bgp-dnsd -count=1` | ✅ existing | ⬜ pending |
| 03-05-01 | 03-05 | 5 | REFACTOR-04 | — | Commented error handling removed | manual | `grep -c "^[[:space:]]*//.*error" internal/dns/resolvers.go` | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/main.go` — add `type configKey struct{}` definition
- [ ] `internal/loop/main.go` — add logger parameter to `NewLoop()`
- [ ] `internal/dns/resolvers.go` — add logger parameter to `newResolvers()`
- [ ] `internal/dns/loop.go` — remove `ctx.Value("cfg")` and `log.L()` calls
- [ ] `internal/fswatcher/loop.go` — remove `ctx.Value("cfg")` and `log.L()` calls

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dead code removed (resolvers.go) | REFACTOR-04 | Code cleanliness check | `grep -n "^.*//.*error" internal/dns/resolvers.go` — should find no commented error handling blocks |
| Dead code removed (bgp/main.go) | REFACTOR-04 | Code cleanliness check | `grep -n "^.*//.*hashmap" internal/bgp/main.go` — should find no commented hashmap |
| No panic calls in constructors | REFACTOR-03 | Safety verification | `grep -rn "panic(" internal/dns/ internal/bgp/ internal/fswatcher/` — should find no panic calls in New* functions |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** {pending / approved YYYY-MM-DD}
