<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Regex Syntax:** Use `regex:` prefix syntax in domainlist files. Format: `regex:([a-z]+)\.internal\.corp`. Implementation: In `cache.load()`, detect `strings.HasPrefix(line, "regex:")`, extract pattern via `strings.TrimPrefix()`, compile and register.
- **Wildcard Support:** Include wildcard domain entries (*.example.com) in Phase 1. Priority: exact > wildcard > regex > catch-all.
- **Pattern Safety Limits:** Validate at load time only — no additional runtime limits. `regexp.Compile()` at domainlist load time; invalid patterns return error from `registerRegex()`. No compile timeout or pattern length enforcement.
- **Error Handling for Invalid Regex:** Skip invalid regex patterns and log a warning; continue loading the rest of the file.
- **Integration with Global State:** Work within existing global state; defer DI refactoring to Phase 3. Replace `dns.HandleFunc(".", proxyQuery)` catch-all with `regexServeMux` in `internal/dns/main.go`. `_cache` struct gains a `*regexServeMux` field; `newCache()` accepts and stores it; `register()` calls mux methods instead of global `dns.HandleFunc()`.

### the agent's Discretion
- Exact line-level changes to `cache.go`, `main.go`, `resolvers.go`
- The `regexServeMux` struct design (spike blueprint is a reference, not a mandate)
- How `registerRegex()` interacts with the existing `register()` method
- Whether `regexServeMux` needs to embed `dns.ServeMux` or be fully standalone
- Test additions for the new functionality (Wave 0 test gaps)

### Deferred Ideas (OUT OF SCOPE)
- `/pattern/` syntax (Option B from spike blueprint) — only `regex:` prefix
- Runtime limits (compile timeout, pattern length enforcement)
- DI refactoring — Phase 3 only
- Fail-to-start on invalid regex — skip-and-warn approach
- IPv6-only deployments — out of scope
- DNSSEC validation — out of scope
</user_constraints>

# Phase 1: Regex Domainlist - Research

**Researched:** 2026-06-13
**Domain:** DNS query routing with regex pattern matching (Go, miekg/dns)
**Confidence:** HIGH

## Summary

This phase adds regex pattern matching to the bgp-dns domainlist file, enabling pattern-matched domains (not just exact FQDNs) to be intercepted, resolved via upstream DNS, and advertised via BGP. The spike (regexServeMux) has been validated with 9/9 tests passing in a standalone directory.

The core change is replacing the global `dns.HandleFunc(".", proxyQuery)` catch-all with a custom `regexServeMux` that implements the `dns.Handler` interface (`ServeDNS(w, r)`). The mux routes queries by priority: exact > wildcard > regex > catch-all. The existing `cache.register()` method — which currently calls the global `dns.HandleFunc()` — is refactored to call methods on the custom mux instance instead.

**No new external packages are needed.** The only dependency is Go's standard library `regexp` package, which provides `regexp.Compile()` for load-time pattern compilation and `Regexp.MatchString()` for per-query matching.

**Primary recommendation:** Create `internal/dns/serveMux.go` with the `regexServeMux` struct, modify `cache.go` to accept and use the mux in `register()`/`unregister()`/`load()`, and update `main.go` to wire the mux into the `dns.Server` handler field.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Domainlist parsing (`regex:` prefix) | DNS Cache (`cache.load()`) | — | File I/O and line parsing belong to the cache subsystem |
| Regex pattern compilation | DNS Cache (`registerRegex()`) | — | Compiled `*regexp.Regexp` objects stored in the mux, not per-query |
| DNS query routing (exact/wildcard/regex/catch-all) | Custom mux (`regexServeMux.ServeDNS()`) | — | The mux is the single decision point for query dispatch |
| DNS query interception (registered domains) | DNS Cache (`cache.resolve()`) | — | Existing resolve path unchanged; called by mux handlers |
| Upstream proxy (unregistered domains) | Resolver ring (`proxyQuery()`) | — | Existing catch-all proxy path; passed to mux as catchAll handler |
| BGP advertisement | BGP Server (`bgp.Advance/Withdraw`) | — | Unchanged; triggered by cache.upsert() as before |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `regexp` (Go stdlib) | Go 1.24.4 | Compile and match regex patterns at load time | Standard library, no external dependency, thread-safe after compile |
| `github.com/miekg/dns` | v1.1.67 [VERIFIED: go doc] | DNS protocol library; `dns.Handler` interface for custom mux | Existing dependency; `Handler` interface is `ServeDNS(w ResponseWriter, r *Msg)` [VERIFIED: go doc] |
| `github.com/bluele/gcache` | v0.0.2 | LFU cache with eviction callbacks | Existing dependency; no changes needed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/rs/zerolog` | v1.34.0 | Structured logging for invalid regex warnings | `log.L().Warn()` in `load()` when skipping invalid patterns |

**No new external packages needed.** Only Go standard library `regexp` is introduced.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `regexp` (Go stdlib) | stdlib | N/A | N/A | golang/go | OK | Approved — no audit needed |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

> **Note:** This phase introduces zero new external dependencies. The only new import is Go's standard library `regexp` package.

## Architecture Patterns

### System Architecture Diagram

```
Client DNS Query (UDP :5354)
        │
        ▼
┌─────────────────────────────────────────────┐
│           dns.Server                         │
│           Handler: regexServeMux             │
│                                             │
│  ┌──────────┐    ┌──────────┐              │
│  │ Exact    │    │ Wildcard │              │
│  │ map      │    │ slice    │              │
│  └────┬─────┘    └────┬─────┘              │
│       │               │                     │
│  ┌────▼───────────────▼─────┐              │
│  │  regexServeMux.ServeDNS()│              │
│  │  (priority: exact >      │              │
│  │   wildcard > regex >     │              │
│  │   catch-all)             │              │
│  └────┬──────────────────┬───┘              │
│       │                  │                  │
│  ┌────▼─────┐    ┌──────▼──────┐           │
│  │ cache    │    │ cache       │           │
│  │.resolve() │    │.resolve()  │           │
│  │ (exact)  │    │ (regex)    │           │
│  └────┬─────┘    └──────┬──────┘           │
│       │                 │                   │
│       ▼                 ▼                   │
│  ┌─────────────────────────────┐           │
│  │  cache.upsert()             │           │
│  │  → bgp.Advance/Withdraw     │           │
│  └─────────────────────────────┘           │
│                                             │
│  ┌─────────────────────────────┐           │
│  │  catchAll → proxyQuery()    │           │
│  │  (unregistered domains)     │           │
│  └─────────────────────────────┘           │
└─────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/dns/
├── main.go          # dns.Serve(), Shutdown(), global state (_server, _cache, _resolvers)
├── cache.go         # cache struct, register(), unregister(), load(), upsert()
├── cacheEntry.go    # cacheEntry, cacheKey structs and helpers
├── cache_test.go    # existing cache load tests
├── resolve.go       # cache.resolve() — DNS resolution logic
├── resolvers.go     # resolver ring, proxyQuery()
├── loop.go          # cache.refresher loop
├── serveMux.go      # NEW: regexServeMux with priority routing
```

### Pattern: Custom DNS Handler via dns.Handler Interface

The `miekg/dns` library defines a minimal `Handler` interface:

```go
type Handler interface {
    ServeDNS(w ResponseWriter, r *Msg)
}
```

Any struct with a `ServeDNS(w dns.ResponseWriter, r *dns.Msg)` method can be assigned to `dns.Server.Handler`. This is how the custom `regexServeMux` integrates — it replaces the global `dns.DefaultServeMux` without modifying any miekg/dns internals.

**Source:** Verified via `go doc github.com/miekg/dns.Handler` [VERIFIED: go doc]

### Pattern: Load-Time Compilation, Cache at Runtime

```go
// Spike blueprint — compile once, match many
func (c *cache) registerRegex(pattern string) error {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return fmt.Errorf("invalid regex %q: %w", pattern, err)
    }
    c.mu.Lock()
    c.regexPatterns = append(c.regexPatterns, re)
    c.mu.Unlock()
    return nil
}
```

The `regexp.Compile()` function returns a `*regexp.Regexp` that is safe for concurrent use by multiple goroutines. This means:
- Compile happens once during `load()` (single-threaded)
- Per-query matching uses `re.MatchString()` (concurrent-safe, no locking needed in regexp itself)
- The mux's `sync.RWMutex` protects only the handler registration/maps, not the regex objects themselves

**Source:** Go `regexp` documentation [VERIFIED: go doc]

### Pattern: Priority Routing in ServeDNS

```go
func (m *regexServeMux) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    if len(r.Question) == 0 {
        if m.catchAll != nil { m.catchAll(w, r) }
        return
    }
    question := r.Question[0].Name
    if !strings.HasSuffix(question, ".") { question = question + "." }

    m.mu.RLock()
    defer m.mu.RUnlock()

    // 1. Exact match
    if handler, ok := m.exact[strings.TrimSuffix(question, ".")]; ok { handler(w, r); return }

    // 2. Wildcard match
    for _, wc := range m.wildcard {
        if strings.HasSuffix(question, wc.prefix+".") { wc.handler(w, r); return }
    }

    // 3. Regex match
    for _, rh := range m.regex {
        if rh.pattern.MatchString(strings.TrimSuffix(question, ".")) {
            rh.handler(w, r); return
        }
    }

    // 4. Catch-all
    if m.catchAll != nil { m.catchAll(w, r) }
}
```

**Source:** Spike blueprint `.opencode/skills/spike-findings-bgp-dns/references/regex-domainlist.md` [CITED: spike-findings-bgp-dns/references/regex-domainlist.md]

### Anti-Patterns to Avoid

- **Do NOT use global `dns.HandleFunc()` for exact/wildcard/regex handlers.** The current code uses `dns.HandleFunc(cn, ...)` which registers on `dns.DefaultServeMux` (the global mux). This creates a conflict: if the custom mux's `register()` still calls the global `dns.HandleFunc()`, both the global mux AND the custom mux will have handlers, and the `dns.Server` will use whichever is assigned to its `Handler` field. Since we're assigning `regexServeMux` to `Server.Handler`, only the custom mux's handlers will be invoked — the global handlers registered via `dns.HandleFunc()` will be **ignored**. This is the critical bug to avoid.

- **Do NOT compile regex per-query.** Always compile at load time. The `regexp` package is designed for this: compile once, match many. Per-query compilation would be orders of magnitude slower.

- **Do NOT use `regexp.MustCompile()` in `load()`.** It panics on invalid patterns. Use `regexp.Compile()` and handle the error, which allows skipping invalid patterns with a warning (per CONTEXT.md decision).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| DNS query routing | Custom trie/lookup | `regexServeMux` with exact map + wildcard slice + regex slice | miekg/dns `Handler` interface is the standard; custom mux is the proven pattern from spike |
| Regex compilation | Per-query compile | `regexp.Compile()` at load time | Thread-safe compiled `*regexp.Regexp`; O(1) per-query match vs O(n) compile |
| Priority-based matching | Linear scan with if/else | Structured `ServeDNS()` with early returns | Spike-validated pattern; clean separation of concerns |

**Key insight:** The `miekg/dns` library's `ServeMux` uses a crit-trie for efficient domain matching. Our custom `regexServeMux` replaces this with a simpler exact-map + wildcard-slice + regex-slice approach. For the expected scale (<1000 entries), this is simpler and correct. The spike validated this with 9/9 passing tests.

## Runtime State Inventory

> **Not applicable.** This is a greenfield feature (adding regex support), not a rename/refactor/migration phase. No runtime state needs inventory.

## Common Pitfalls

### Pitfall 1: Global vs Custom Mux Confusion
**What goes wrong:** The existing `cache.register()` calls `dns.HandleFunc(cn, func(...) {...})` which registers on the **global** `dns.DefaultServeMux`, not on the custom `regexServeMux`. When the `dns.Server` is configured with `Handler: customMux`, the global handlers are never invoked.

**Why it happens:** The current codebase uses `dns.HandleFunc()` and `dns.HandleRemove()` (global functions) throughout. The refactor to use the custom mux requires changing these calls to `mux.HandleFunc()` and `mux.HandleRemove()` (methods on the custom mux instance).

**How to avoid:** In `cache.register()`, replace `dns.HandleFunc(cn, ...)` with `c.mux.HandleFunc(cn, ...)`. In `cache.unregister()`, replace `dns.HandleRemove(cn)` with `c.mux.HandleRemove(cn)`. The `regexServeMux` must implement `HandleFunc` and `HandleRemove` methods that mirror the global `dns.HandleFunc`/`dns.HandleRemove` behavior but operate on its own internal maps.

**Warning signs:** After implementing, DNS queries for registered domains are not intercepted — they all fall through to `proxyQuery()`.

### Pitfall 2: Trailing Dot Normalization
**What goes wrong:** Domain names in DNS wire format include a trailing dot (e.g., `example.com.`). The domainlist file does NOT have trailing dots (e.g., `example.com`). If the mux compares raw strings without normalizing, exact matches will fail.

**Why it happens:** `dns.CanonicalName()` (used in `cache.register()`) adds the trailing dot. But the `question.Name` from incoming DNS queries also has the trailing dot. The domainlist entries in the file do not. The existing code uses `dns.CanonicalName()` to normalize — the custom mux must do the same.

**How to avoid:** In `register()`, always use `dns.CanonicalName(fqdn)` to normalize the domainlist entry. In `ServeDNS()`, normalize the question name the same way. The spike blueprint handles this with `strings.TrimSuffix(question, ".")` for map lookups.

**Warning signs:** Exact-match domains from the domainlist file never match incoming queries, even though they're registered.

### Pitfall 3: Shutdown Cleanup of Global Handlers
**What goes wrong:** `dns.Shutdown()` in `main.go` calls `dns.HandleRemove(".")` which removes the catch-all from the **global** mux. After replacing the global catch-all with the custom mux, this call becomes a no-op (or removes from the wrong place).

**Why it happens:** The shutdown path assumes the catch-all was registered via `dns.HandleFunc(".", ...)`. The custom mux stores its catch-all separately.

**How to avoid:** Replace `dns.HandleRemove(".")` in `Shutdown()` with `mux.HandleRemoveCatchAll()` (or equivalent). Or simply set `m.catchAll = nil` and remove all exact/wildcard/regex entries. The spike blueprint should be consulted for the exact shutdown cleanup pattern.

**Warning signs:** After shutdown, the global `dns.DefaultServeMux` still has stale handlers, causing unexpected DNS responses on subsequent Serve() calls (re-init).

### Pitfall 4: Concurrency During Handler Registration
**What goes wrong:** If `register()` is called while a DNS query is being served, a race condition can occur between reading the exact map and writing to it.

**Why it happens:** The `regexServeMux` stores exact matches in a `map[string]dns.HandlerFunc`. Maps in Go are not safe for concurrent read/write.

**How to avoid:** The spike blueprint uses `sync.RWMutex` — `ServeDNS()` acquires `RLock()` (read), `register()` acquires `Lock()` (write). This is correct because: (1) `load()` is called from the main goroutine (not concurrent with DNS queries during startup), and (2) the RWMutex protects the internal maps during the brief window of registration.

**Warning signs:** Data race detected by `go test -race`; intermittent panics during domainlist reload.

## Code Examples

### serveMux.go — Full regexServeMux Implementation

```go
package dns

import (
	"regexp"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

type regexServeMux struct {
	mu       sync.RWMutex
	exact    map[string]dns.HandlerFunc
	wildcard []struct {
		prefix  string
		handler dns.HandlerFunc
	}
	regex    []regexHandler
	catchAll dns.HandlerFunc
}

type regexHandler struct {
	pattern *regexp.Regexp
	handler func(dns.ResponseWriter, *dns.Msg)
}

func newRegexServeMux() *regexServeMux {
	return &regexServeMux{
		exact:    make(map[string]dns.HandlerFunc),
		wildcard: make([]struct{ prefix string; handler dns.HandlerFunc }, 0),
		regex:    make([]regexHandler, 0),
	}
}

func (m *regexServeMux) HandleFunc(pattern string, handler dns.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if it's a wildcard pattern
	if strings.HasPrefix(pattern, "*.") {
		m.wildcard = append(m.wildcard, struct {
			prefix  string
			handler dns.HandlerFunc
		}{prefix: pattern[1:], handler: handler})
		return
	}

	// Exact match
	m.exact[dns.CanonicalName(pattern)] = handler
}

func (m *regexServeMux) HandleRegex(pattern string, handler dns.HandlerFunc) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.regex = append(m.regex, regexHandler{pattern: re, handler: handler})
	return nil
}

func (m *regexServeMux) HandleRemove(pattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.HasPrefix(pattern, "*.") {
		// Remove from wildcard slice
		prefix := pattern[1:]
		for i, wc := range m.wildcard {
			if wc.prefix == prefix {
				m.wildcard = append(m.wildcard[:i], m.wildcard[i+1:]...)
				return
			}
		}
		return
	}

	// Remove from exact map
	delete(m.exact, dns.CanonicalName(pattern))
}

func (m *regexServeMux) SetCatchAll(handler dns.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.catchAll = handler
}

func (m *regexServeMux) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		if m.catchAll != nil {
			m.catchAll(w, r)
		}
		return
	}

	question := r.Question[0].Name
	// Normalize: ensure trailing dot for DNS wire format
	if !strings.HasSuffix(question, ".") {
		question = question + "."
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Exact match
	canonical := strings.TrimSuffix(question, ".")
	if handler, ok := m.exact[canonical]; ok {
		handler(w, r)
		return
	}

	// 2. Wildcard match
	for _, wc := range m.wildcard {
		if strings.HasSuffix(canonical, wc.prefix+".") {
			wc.handler(w, r)
			return
		}
	}

	// 3. Regex match
	for _, rh := range m.regex {
		if rh.pattern.MatchString(canonical) {
			rh.handler(w, r)
			return
		}
	}

	// 4. Catch-all
	if m.catchAll != nil {
		m.catchAll(w, r)
	}
}
```

### cache.go — Modified register() and unregister()

```go
// In cache struct, add:
type cache struct {
    // ... existing fields ...
    mux *regexServeMux  // NEW: custom mux instead of global dns mux
}

// Modified register():
func (c *cache) register(fqdn string) error {
    if len(fqdn) < 2 {
        return fmt.Errorf("'%s'. %w", fqdn, EInvalidFQDN)
    }
    cn := dns.CanonicalName(fqdn)
    c.mux.HandleFunc(cn, func(rw dns.ResponseWriter, m *dns.Msg) {
        c.resolve(rw, m, true)
    })
    // ... rest unchanged: initial resolve + cache upsert ...
}

// Modified unregister():
func (c *cache) unregister(fqdn string) error {
    cn := dns.CanonicalName(fqdn)
    c.L().Debug().Msgf("Unregistering %s", cn)
    c.mux.HandleRemove(cn)  // CHANGED: was dns.HandleRemove(cn)
    // ... rest unchanged ...
}
```

### main.go — Modified Serve() and Shutdown()

```go
// In Serve():
func Serve(ctx context.Context) (e error) {
    // ... existing setup ...
    _resolvers = newResolvers(cfg.Dns.Resolvers)
    _cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl,
        newResolvers(cfg.Dns.List.Resolvers), log.L())

    // NEW: create custom mux
    mux := newRegexServeMux()
    _cache.mux = mux  // Pass to cache for register/unregister

    // Set catch-all to proxyQuery
    mux.SetCatchAll(_resolvers.proxyQuery)

    e = _cache.serve(ctx)

    go func(c context.Context) {
        _server = &dns.Server{
            Addr:      fmt.Sprintf("%s:%d", cfg.Dns.Listen.IP.String(), cfg.Dns.Listen.Port),
            Net:       "udp",
            ReusePort: true,
            Handler:   mux,  // CHANGED: was nil (uses DefaultServeMux)
        }
        // ...
        // REMOVED: dns.HandleFunc(".", _resolvers.proxyQuery) — now set via mux.SetCatchAll()
        if err := _server.ListenAndServe(); err != nil {
            log.L().Fatal().Str("m", "dns").Err(err).Msg("Failed to bind DNS resolver")
        }
    }(ctx)
    return e
}

// In Shutdown():
func Shutdown(ctx context.Context) error {
    // CHANGED: was dns.HandleRemove(".")
    // The custom mux's catchAll is cleared by SetCatchAll(nil) or by recreating the mux
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    // ... rest unchanged ...
}
```

### cache.go — Modified load() with regex detection

```go
func (c *cache) load(fn string) error {
    // ... existing file open code ...
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if len(line) == 0 { continue }
        if line[0] == '#' || line[0] == ';' { continue }

        if strings.HasPrefix(line, "regex:") {
            pattern := strings.TrimPrefix(line, "regex:")
            if err := c.registerRegex(pattern); err != nil {
                c.L().Warn().Msgf("Skipping invalid regex pattern %q: %v", pattern, err)
                continue  // CHANGED: was "return err" — skip invalid, continue loading
            }
        } else {
            if err := c.register(line); err != nil {
                return err  // Invalid FQDN still fails
            }
        }
    }
    // ...
}

func (c *cache) registerRegex(pattern string) error {
    return c.mux.HandleRegex(pattern, func(w dns.ResponseWriter, r *dns.Msg) {
        c.resolve(w, r, true)
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `dns.HandleFunc(".", proxyQuery)` — global catch-all | `regexServeMux` with explicit priority routing | This phase | Queries for registered domains bypass proxyQuery entirely; unregistered domains still reach proxyQuery |
| No regex support in domainlist | `regex:` prefix pattern matching | This phase | Operators can use patterns like `regex:([a-z]+)\.internal\.corp` |
| Exact-match only | Exact > wildcard > regex > catch-all priority | This phase | `*.example.com` and regex patterns coexist with exact domains |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `miekg/dns.Server.Handler` accepts any `dns.Handler` implementation | Architecture Patterns | LOW — verified via `go doc` |
| A2 | `regexp.Regexp.MatchString()` is safe for concurrent use after compile | Common Pitfalls #4 | LOW — documented in Go stdlib |
| A3 | The spike blueprint's `regexServeMux` design is correct for integration | Code Examples | MEDIUM — spike was standalone; integration with existing global state may reveal issues |
| A4 | No new external packages needed (only stdlib `regexp`) | Standard Stack | LOW — verified; `regexp` is in Go stdlib |
| A5 | `dns.CanonicalName()` adds trailing dot consistently | Common Pitfalls #2 | MEDIUM — must verify behavior matches what incoming DNS queries provide |

## Open Questions

1. **Should `regexServeMux` embed `dns.ServeMux` for exact/wildcard handling?**
   - What we know: The spike implements a fully standalone `regexServeMux` with its own exact map and wildcard slice.
   - What's unclear: Whether embedding `dns.ServeMux` would simplify the exact/wildcard handling and provide built-in crit-trie optimization.
   - Recommendation: Standalone implementation (as in the spike) is simpler and more explicit. The spike's 9/9 test pass validates this approach.

2. **How does the fswatcher reload path interact with the custom mux?**
   - What we know: `fswatcher/loop.go` calls `dns.Load()` on file change, which calls `cache.load()`.
   - What's unclear: Whether `load()` needs to clear the old mux state before registering new patterns (generation-based eviction already handles cache entries, but does it handle mux handlers?).
   - Recommendation: `load()` should unregister all old patterns before registering new ones. The `evictByGeneration()` path currently calls `unregister()` for each key — but regex patterns don't have FQDN keys to unregister by. Consider tracking registered pattern strings for cleanup.

3. **Does `Shutdown()` need to clear the custom mux's handlers?**
   - What we know: Current `Shutdown()` calls `dns.HandleRemove(".")` to remove the global catch-all.
   - What's unclear: Whether the custom mux's handlers need explicit cleanup, or if recreating the mux on the next `Serve()` call is sufficient.
   - Recommendation: Set `mux.catchAll = nil` and clear all maps/slices in `Shutdown()`. This prevents stale handlers from persisting across restarts.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.24+ | Build | ✓ | 1.26.2 | — |
| `regexp` stdlib | Pattern compilation | ✓ | Go 1.26.2 | — |
| `miekg/dns` v1.1.67 | DNS server | ✓ | In go.mod | — |
| Linux kernel | BGP/NET_ADMIN | ✓ (assumed) | — | Phase blocks on non-Linux |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `stretchr/testify` |
| Config file | `go test` (no separate config) |
| Quick run command | `go test ./internal/dns/ -run TestCache_Load -v` |
| Full suite command | `go test ./... -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REGEX-01 | `regex:` prefix syntax parsed in domainlist | unit | `go test ./internal/dns/ -run TestRegex -v` | ❌ Wave 0 |
| REGEX-02 | Priority: exact > wildcard > regex > catch-all | unit | `go test ./internal/dns/ -run TestMuxPriority -v` | ❌ Wave 0 |
| REGEX-03 | Regex compiled at load time, not per-query | unit | `go test ./internal/dns/ -run TestLoadCompile -v` | ❌ Wave 0 |
| REGEX-04 | Invalid regex skipped with warning | unit | `go test ./internal/dns/ -run TestInvalidRegex -v` | ❌ Wave 0 |
| REGEX-05 | Exact-match entries still work | integration | `go test ./internal/dns/ -run TestExactMatch -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/dns/ -run TestCache -v`
- **Per wave merge:** `go test ./... -v`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/dns/serveMux_test.go` — tests for `regexServeMux` priority routing, HandleFunc, HandleRemove, HandleRegex
- [ ] `internal/dns/serveMux_test.go` — tests for `load()` with `regex:` prefix entries
- [ ] `internal/dns/serveMux_test.go` — tests for invalid regex handling (skip + warn)
- [ ] `internal/dns/serveMux_test.go` — tests for wildcard matching
- [ ] Update `internal/dns/cache_test.go` — existing tests call `c.load()` which now uses `c.mux` — `newTestCache()` must create and inject a `regexServeMux`

## Security Domain

> `security_enforcement` is enabled (absent from config = enabled). However, Phase 1 does not modify security-sensitive code paths.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | Partial | Regex patterns validated at load time via `regexp.Compile()` |
| V6 Cryptography | No | Not applicable |

### Known Threat Patterns for Go DNS

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| ReDoS (regex denial of service) | Availability | Validate at load time; no runtime limits (trust operator, root daemon) |
| Regex injection | Tampering | `regex:` prefix is explicit — no user input in pattern strings (operator-controlled file) |

## Sources

### Primary (HIGH confidence)
- `go doc github.com/miekg/dns.Handler` — Handler interface definition
- `go doc github.com/miekg/dns.ServeMux` — Built-in ServeMux methods
- `go doc github.com/miekg/dns.HandleFunc` — Global HandleFunc registers on DefaultServeMux
- `go doc github.com/miekg/dns.HandleRemove` — Global HandleRemove deregisters from DefaultServeMux
- `go doc github.com/miekg/dns.Server` — Handler field accepts any Handler
- `go doc regexp.Compile` — Standard library regex compilation
- Spike blueprint: `.opencode/skills/spike-findings-bgp-dns/references/regex-domainlist.md` — regexServeMux implementation pattern

### Secondary (MEDIUM confidence)
- `.planning/codebase/ARCHITECTURE.md` — Component responsibilities and data flow
- `.planning/codebase/PATTERNS.md` — Error handling, concurrency, and global state patterns

### Tertiary (LOW confidence)
- `.planning/phases/01-regex-domainlist/01-CONTEXT.md` — User decisions and canonical refs

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — only Go stdlib `regexp` added; miekg/dns interface verified via `go doc`
- Architecture: HIGH — spike blueprint validated with 9/9 tests; codebase architecture well-documented
- Pitfalls: HIGH — global vs custom mux confusion is a well-documented gotcha; trailing dot normalization is a known DNS gotcha

**Research date:** 2026-06-13
**Valid until:** 30 days (stable Go stdlib + miekg/dns interfaces)
