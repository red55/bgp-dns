# Regex Domainlist Support

## Requirements

- Must support mixing exact domain entries and regex patterns in the same list file
- Regex patterns must be pre-compiled and cached for performance (no per-query compilation)
- Must not degrade performance for existing exact-match entries

## How to Build It

### 1. Create the regexServeMux

Replace the global `dns.HandleFunc(".", proxyQuery)` catch-all in `internal/dns/main.go` with a custom `regexServeMux` that supports:
- Exact FQDN matching (existing domainlist entries)
- Wildcard matching (`*.example.com`)
- Regex pattern matching (new feature)
- Catch-all fallback (proxy to upstream)

Priority order: **exact > wildcard > regex > catch-all**

```go
type regexServeMux struct {
    mu       sync.RWMutex
    exact    map[string]dns.HandlerFunc
    wildcard []struct{ prefix string; handler dns.HandlerFunc }
    regex    []regexHandler
    catchAll dns.HandlerFunc
}

type regexHandler struct {
    pattern *regexp.Regexp
    handler func(dns.ResponseWriter, *dns.Msg)
}
```

### 2. Add HandleRegex method

```go
func (m *regexServeMux) HandleRegex(pattern string, handler dns.HandlerFunc) error {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return fmt.Errorf("invalid regex %q: %w", pattern, err)
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    m.regex = append(m.regex, regexHandler{pattern: re, handler: handler})
    return nil
}
```

### 3. Wire up ServeDNS with priority routing

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

### 4. Modify domainlist loader (internal/dns/cache.go:load)

Detect regex patterns in the domainlist file. Two syntax options:

**Option A — `regex:` prefix:**
```
# Exact domain
cloudflare.com
# Regex pattern
regex:([a-z]+)\.internal\.corp
```

**Option B — `/pattern/` syntax:**
```
# Exact domain
cloudflare.com
# Regex pattern
/([a-z]+)\.internal\.corp/
```

When loading, parse each line:
```go
func (c *cache) load(fn string) error {
    // ... existing file open code ...
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if len(line) == 0 || line[0] == '#' || line[0] == ';' { continue }

        if strings.HasPrefix(line, "regex:") {
            pattern := strings.TrimPrefix(line, "regex:")
            if err := c.registerRegex(pattern); err != nil { return err }
        } else if strings.HasPrefix(line, "/") && strings.HasSuffix(line, "/") {
            pattern := line[1 : len(line)-1]
            if err := c.registerRegex(pattern); err != nil { return err }
        } else {
            if err := c.register(line); err != nil { return err }
        }
    }
    // ...
}
```

### 5. Add registerRegex to cache

```go
func (c *cache) registerRegex(pattern string) error {
    re, err := regexp.Compile(pattern)
    if err != nil {
        return fmt.Errorf("invalid regex %q: %w", pattern, err)
    }
    // Store compiled regex for later matching
    c.mu.Lock()
    c.regexPatterns = append(c.regexPatterns, re)
    c.mu.Unlock()
    return nil
}
```

### 6. Replace the DNS server handler in main.go

```go
// In internal/dns/main.go, replace:
// dns.HandleFunc(".", _resolvers.proxyQuery)

// With:
mux := newRegexServeMux()
mux.HandleFuncCatchAll(_resolvers.proxyQuery)
_server = &dns.Server{
    Addr:    fmt.Sprintf("%s:%d", cfg.Dns.Listen.IP.String(), cfg.Dns.Listen.Port),
    Net:     "udp",
    Handler: mux,
}
```

The `register()` function (which calls `dns.HandleFunc(cn, ...)`) should instead call `mux.HandleFunc(cn, ...)` to register exact/wildcard handlers.

## What to Avoid

- **Per-query regex compilation** — always compile at load time, cache the `*regexp.Regexp` object
- **Regex before exact match** — exact FQDN entries from domainlist must take priority over regex patterns to avoid accidental matches
- **Unbounded regex patterns** — validate regex syntax at load time and reject invalid patterns immediately (don't defer to query time)
- **Replacing all existing dns.HandleFunc calls** — only the catch-all (`"."`) handler needs to be replaced. Exact match handlers from `register()` stay as-is but go through the mux instead.

## Constraints

- `miekg/dns` does not support regex natively — a custom ServeMux wrapper is required
- Regex compilation must happen at domainlist load time, not per-query
- For 1000 regex patterns, worst case is 1000 `MatchString()` calls per query (O(n) per pattern)
- Thread safety: use `sync.RWMutex` to protect concurrent handler registration and query routing
- The cache's `register()` and `unregister()` methods must be updated to work with the new mux instead of global dns.HandleFunc

## Origin

Synthesized from spikes: 001
Source files available in: sources/001-regex-dns-intercept/
