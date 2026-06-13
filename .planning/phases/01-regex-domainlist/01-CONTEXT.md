# Phase 1 — Regex Domainlist

**Date:** 2026-06-13
**Phase:** 1 of 5
**Status:** Context captured ✓

## Domain

Add regex pattern matching to the domainlist file so that pattern-matched domains (not just exact FQDNs) can be intercepted, resolved via upstream DNS, and advertised via BGP. Existing exact-match entries continue to work without performance regression.

## Decisions

### Regex Syntax
- **Decision:** Use `regex:` prefix syntax in domainlist files
- **Format:** `regex:([a-z]+)\.internal\.corp`
- **Rationale:** Explicit and unambiguous; does not conflict with domain names containing `/`; easier to parse than `/pattern/` delimiter
- **Implementation:** In `cache.load()`, detect `strings.HasPrefix(line, "regex:")`, extract pattern via `strings.TrimPrefix()`, compile and register

### Wildcard Support
- **Decision:** Include wildcard domain entries (*.example.com) in Phase 1
- **Rationale:** Spike blueprint already implements wildcard matching in regexServeMux; keeping it separate would add unnecessary phase complexity
- **Priority:** exact > wildcard > regex > catch-all (as defined in spike blueprint)
- **Implementation:** Wildcard entries (starting with `*.`) register via `mux.HandleFunc()` with wildcard prefix matching logic in regexServeMux

### Pattern Safety Limits
- **Decision:** Validate at load time only — no additional runtime limits
- **Mechanism:** `regexp.Compile()` at domainlist load time; invalid patterns return error from `registerRegex()`
- **No compile timeout or pattern length enforcement** — trust the operator (daemon runs as root on controlled network)
- **Performance:** Worst case O(n) per query for n regex patterns (each `MatchString()` call is linear in pattern length)

### Error Handling for Invalid Regex
- **Decision:** Skip invalid regex patterns and log a warning; continue loading the rest of the file
- **Rationale:** Lenient approach prevents a single bad line from blocking the entire domainlist; operator sees the warning and can fix it
- **Implementation:** In `cache.load()`, if `registerRegex()` returns an error, log `Warn()` with the invalid pattern and line number, continue scanning remaining lines

### Integration with Global State
- **Decision:** Work within existing global state; defer DI refactoring to Phase 3
- **Mechanism:** Replace `dns.HandleFunc(".", proxyQuery)` catch-all with `regexServeMux` in `internal/dns/main.go`; `cache.register()` and `cache.unregister()` call methods on the mux instance stored in the `_cache` struct
- **Rationale:** Minimal scope creep; Phase 3 will refactor globals into explicit dependencies anyway
- **Key change:** `_cache` struct gains a `*regexServeMux` field; `newCache()` accepts and stores it; `register()` calls `mux.HandleFunc()` instead of `dns.HandleFunc()`

## Code Context

### Files to Modify
| File | Change |
|------|--------|
| `internal/dns/main.go` | Replace `dns.HandleFunc(".", proxyQuery)` with `regexServeMux`; create mux in `Serve()` |
| `internal/dns/cache.go` | Add `*regexServeMux` field to cache struct; update `register()` to call mux; update `load()` to detect `regex:` prefix; add `registerRegex()` method |
| `internal/dns/resolvers.go` | Add wildcard matching logic to regexServeMux; update `proxyQuery()` to accept mux |
| `internal/dns/cache.go:unregister()` | Call `mux.HandleRemove()` instead of `dns.HandleRemove()` |

### Files to Create
| File | Purpose |
|------|---------|
| `internal/dns/serveMux.go` | `regexServeMux` struct with exact/wildcard/regex/catch-all routing and `HandleRegex()` method |

### Existing Assets (Reused)
- Spike 001 blueprint: `.opencode/skills/spike-findings-bgp-dns/references/regex-domainlist.md` — complete regexServeMux implementation with ServeDNS priority routing
- Spike validated: 9/9 tests pass in standalone spike directory
- `miekg/dns` library already imported; no new dependencies needed
- `gcache` LFU cache with eviction callbacks — no changes needed

### Requirements Mapped
| Requirement | How Addressed |
|-------------|---------------|
| REGEX-01 | `regex:` prefix syntax in domainlist loader |
| REGEX-02 | regexServeMux priority: exact > wildcard > regex > catch-all |
| REGEX-03 | `regexp.Compile()` in `load()` / `registerRegex()`, not per-query |
| REGEX-04 | Invalid regex returns error from `registerRegex()`; logged as Warn, pattern skipped |
| REGEX-05 | Exact-match entries use same `register()` path; regexServeMux only adds branches |

## Canonical Refs

| Ref | Path | Purpose |
|-----|------|---------|
| Phase goal | `.planning/ROADMAP.md` §Phase 1 | Scope and success criteria |
| Requirements | `.planning/REQUIREMENTS.md` §Regex Domainlist | REGEX-01 through REGEX-05 |
| Project context | `.planning/PROJECT.md` | Key decisions, constraints, spike findings |
| Spike blueprint | `.opencode/skills/spike-findings-bgp-dns/references/regex-domainlist.md` | regexServeMux implementation pattern |
| Spike skill | `.opencode/skills/spike-findings-bgp-dns/SKILL.md` | Auto-loaded reference for downstream agents |
| DNS server entry | `internal/dns/main.go` | Current catch-all handler at line 49 |
| Domainlist loader | `internal/dns/cache.go` | `load()` at line 194, `register()` at line 136 |
| Resolver proxy | `internal/dns/resolvers.go` | `proxyQuery()` at line 154 (catch-all handler) |
| DNS config | `internal/config/dns.go` | Domainlist file path and resolver config |

---
*Context captured: 2026-06-13*
