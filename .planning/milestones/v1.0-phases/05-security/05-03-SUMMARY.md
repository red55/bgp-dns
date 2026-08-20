---
phase: 05-security
plan: 03
status: complete
requirements: [SEC-04]
commits: 3
tasks_completed: 3
date: 2026-08-19
d_05_p6_path: extraction
---

# Plan 05-03 Summary — BGP AuthPassword chain (SEC-04)

**D-05-P6 path taken: EXTRACTED** (`buildPeerSpec` helper + unit test). The per-peer spec literal was a clean verbatim move — every field constant or a per-peer scalar, zero loop entanglement — so the authorized fallback (inline one-liner + human-verify checklist) was NOT needed.

## Changes

| File | Change |
|------|--------|
| `internal/config/bgp_auth_test.go` | NEW (W0, Task 1): `TestInit_BgpAuthPassword` — table-driven (absent→"", present→exact round-trip, explicit-empty→""), mirrors `dns_timeout_test.go` t.TempDir()+Init pattern; fixture uses the load-bearing `Addressess:` key spelling. Plain-testing house style (no testify in this package). |
| `internal/config/bgp.go` | MODIFIED: `bgpNeighbor` += `AuthPassword string \`yaml:"AuthPassword" json:"AuthPassword"\`` with RFC 2385/5925 comment; positioned last; sibling tags + the `Addressess` typo byte-for-byte preserved. |
| `internal/bgp/main.go` | MODIFIED: per-peer spec literal extracted VERBATIM into pure `buildPeerSpec(peerSpecInput) *bgpapi.Peer` (private input struct carries exactly the scalars consumed); AddPeer loop body = mapping + call, iteration/error handling unchanged; `Conf` gains `AuthPassword: in.AuthPassword` as its only behavioral delta. NewBgp signature and external behavior unchanged. No credential logging anywhere in the diff. |
| `internal/bgp/peer_spec_test.go` | NEW: `TestBuildPeerSpec_AuthPassword` — password-propagated ("k-123") / unset-stays-empty subtests + guard-rail identity asserts (NeighborAddress, PeerAsn, Transport.LocalAddress, EbgpMultihop.Enabled) to catch mis-extraction regressions. |
| `appsettings.yml` | MODIFIED: sample peer documents OPTIONAL `#AuthPassword: <tcp-md5-session-key>` commented out (shipped default stays OFF; no real secret in repo) with match-peer-router + file-mode hygiene notes. |
| `README.md` | MODIFIED: "BGP peer authentication" subsection — opt-in TCP-MD5 semantics, empty=pre-phase behavior, key must match peer router, chmod-600 discipline for secret-bearing configs, never-logged guarantee. |

## RED Evidence (Task 1)

Command: `go vet ./internal/config/` (pre-field):
```
vet: internal/config/bgp_auth_test.go:61:31: cfg.Bgp.Peers[0].AuthPassword
     undefined (type *bgpNeighbor has no field or method AuthPassword)
```
Compile failure IS the red state per plan (field-selection on absent struct member). Post-Task-2: all three parse subtests GREEN.

One fixture wrinkle during development (not part of the final diff): a stray `%s` inside the backtick YAML const parsed as a bogus YAML directive; removed. Tab-indented raw strings are YAML-illegal — kept the const at column 0 like all other fixtures.

## Verification Results

| Gate | Command | Result |
|------|---------|--------|
| Parse (GREEN leg) | `go test ./internal/config/ -run TestInit_BgpAuthPassword -count=1` | PASS 3/3 |
| Spec mapping | `go test ./internal/bgp/ -count=1 -v` | PASS incl. both TestBuildPeerSpec subtests |
| Build + vet | `go build ./... && go vet ./internal/config/ ./internal/bgp/` | clean |
| Quick run | `go test ./cmd/bgp-dnsd/... ./internal/dns/... ./internal/bgp/... ./internal/config/... -count=1` | ok (one transient flake, see below) |
| Dep drift | `go mod tidy && git diff --exit-code go.mod go.sum` | NONE |
| Confinement | `grep -rn AuthPassword --include="*.go"` | only the four planned Go files |
| Never logged | zerolog-call grep over AuthPassword sites | 0 matches |
| Addressess typo | struct tag unchanged | preserved |

Wave-merge full-suite gate (at phase wave close): `go vet ./... && go build ./... && go test ./...` — PENDING ORCHESTRATOR VERIFICATION.

## Pre-existing / environmental (NOT caused by this plan)

- `internal/dns/cacheEntry_test.go:305` (TTL-expiry freshness assert) failed ONCE inside the quick run under concurrent-package load, then passed on two isolated reruns. Timing-sensitive pre-existing test; internal/dns was untouched by this plan (05-02 committed before 05-03 work began). Reported per plan scope rule; not fixed here.

## Self-Check

PASSED — build/vet/tests green, import audit clean (no new imports in main.go: bgpapi already imported; peerSpecInput fields all primitive), symbol uniqueness repo-wide, EOL verified LF-consistent on all touched/new Go files (bgp.go, main.go were already LF).

commit: 3 planned below (W0 test → impl+mapping-test → docs), orchestrator-committed.
