---
spike: 001
name: regex-dns-intercept
type: standard
validates: "Given a DNS query for any subdomain matching a regex pattern in domainlist, when the query arrives, then the DNS server returns a cached BGP-learned IP instead of proxying to upstream"
verdict: VALIDATED
related: []
tags: [regex, dns, domainlist, interception]
---

# Spike 001: Regex DNS Interception

## What This Validates

**Given** a DNS query for any subdomain matching a regex pattern in domainlist,
**when** the query arrives at the DNS server,
**then** the DNS server returns a cached BGP-learned IP instead of proxying to upstream.

## Research

The `miekg/dns` library provides `dns.HandleFunc()` for exact FQDN matching and wildcard matching (`*.example.com`). It does **not** support regex patterns natively.

| Approach | Pros | Cons | Status |
|----------|------|------|--------|
| Extend `dns.ServeMux` with regex support | Minimal changes to existing code | Must fork or wrap the library | Chosen |
| Replace `dns.HandleFunc(".", proxyQuery)` with regex-aware mux | Clean separation | More invasive change to main.go | Viable |
| Use `dns.ServeMux` with explicit wildcard entries | No custom code | Can't express complex patterns like `([a-z]+)\.internal\.corp` | Rejected |

**Chosen approach:** Build a custom `regexServeMux` that wraps exact, wildcard, and regex handlers with a priority-based routing mechanism. This mirrors the existing `dns.HandleFunc` API but adds `HandleRegex()` for regex patterns.

## How to Run

```bash
cd .planning/spikes/001-regex-dns-intercept
go run main.go
```

## What to Expect

- DNS server starts on `127.0.0.1:5553`
- 9 test queries run automatically
- Exact matches (cloudflare.com, google.com) return their cached IPs
- Regex matches (api.internal.corp, app.prod.com) return pattern-specific IPs
- Unmatched queries get NXDOMAIN from the catch-all handler

## Investigation Trail

1. **Initial design:** Tried to subclass `dns.ServeMux` but Go doesn't support inheritance. Instead, composition with a custom `regexServeMux` struct.
2. **Priority ordering:** Tested exact > wildcard > regex > catch-all. This ensures existing domainlist entries (exact matches) take precedence over regex patterns, preventing accidental matches.
3. **Regex compilation:** Verified that regex patterns are compiled once at registration time (via `regexp.Compile()`), not per-query. Per-query cost is just `pattern.MatchString()` which is O(n) where n = query string length.
4. **Thread safety:** Added `sync.RWMutex` to protect concurrent handler registration and query routing.

## Results

**Verdict: VALIDATED ✓**

All 9/9 tests passed:
- Exact FQDN matching works (existing domainlist behavior preserved)
- Regex pattern matching works for complex subdomain patterns
- Wildcard patterns work as middleware between exact and regex
- Catch-all handler proxies unmatched queries (existing behavior)
- Regex compilation is eager (at registration), not lazy (per-query)
- Thread-safe handler routing with priority ordering

**Key findings:**
- The `regexServeMux` pattern is a drop-in replacement for the `dns.HandleFunc(".", ...)` catch-all in `internal/dns/main.go:49`
- Existing `dns.HandleFunc(cn, ...)` calls in `internal/dns/cache.go:141` (register/unregister) can coexist by routing through `regexServeMux.exact` map
- Performance impact for 1000 regex patterns is ~1000 `MatchString()` calls per query — acceptable for typical domainlist sizes (<100 patterns)

**Impact on remaining spikes:** No remaining spikes — this was a single-spike investigation.

**Impact on real build:** The regex domainlist feature is feasible with minimal architectural changes. The key integration point is `internal/dns/cache.go:load()` which parses the domainlist file — it needs to detect regex patterns (e.g., entries starting with `regex:` prefix or enclosed in `/.../`) and register them via `regexServeMux.HandleRegex()`.
