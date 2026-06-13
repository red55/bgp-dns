# bgp-dns — Project State

**Created:** 2026-06-13
**Last Updated:** 2026-06-13

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-06-13)

**Core value:** Resolve domains from a configurable list and advertise their IPs via BGP — fast, correct route propagation with zero manual intervention.
**Current focus:** Phase initialization

## Phase Status

| Phase | Name | Status | Plans | Progress |
|-------|------|--------|-------|----------|
| 1 | Regex Domainlist | ○ | 0/0 | 0% |
| 2 | Test Suite | ○ | 0/0 | 0% |
| 3 | Dependency Injection | ○ | 0/0 | 0% |
| 4 | Reliability | ○ | 0/0 | 0% |
| 5 | Security | ○ | 0/0 | 0% |

## Project Memory

### Decisions
- **2026-06-13:** Brownfield project — codebase already mapped, spike 001 (regex domainlist) validated
- **2026-06-13:** YOLO mode — auto-approve requirements and roadmap
- **2026-06-13:** Standard granularity — 5 phases, balanced scope per phase

### Risks
- **Global state** — Heavy package-level singletons make testing difficult; Phase 3 addresses this
- **No BGP tests** — Reference counting logic is critical but untested; Phase 2 prioritizes this
- **Spike findings in `.opencode/`** — regexServeMux blueprint exists but needs implementation review

### Lessons
- Spike 001 validated that custom regexServeMux works with miekg/dns — no need to fork or patch
- O(n²) set difference in `internal/utils/main.go` is a known performance concern
- gRPC has no authentication — Unix socket permissions are the only mitigation

## Artifacts

| Artifact | Path | Status |
|----------|------|--------|
| Codebase Map | `.planning/codebase/` | ✓ Complete |
| Spike Findings | `.opencode/skills/spike-findings-bgp-dns/` | ✓ Validated |
| Project Context | `.planning/PROJECT.md` | ✓ Created |
| Requirements | `.planning/REQUIREMENTS.md` | ✓ Created |
| Roadmap | `.planning/ROADMAP.md` | ✓ Created |
| Config | `.planning/config.json` | ✓ Created |

---
*State initialized: 2026-06-13*
