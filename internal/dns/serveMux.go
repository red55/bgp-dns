package dns

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// regexHandler holds a pre-compiled regex pattern and its handler function.
type regexHandler struct {
	pattern *regexp.Regexp
	handler func(dns.ResponseWriter, *dns.Msg)
}

// regexServeMux is a DNS handler multiplexer that supports exact, wildcard,
// regex, and catch-all routing with priority: exact > wildcard > regex > catch-all.
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

// newRegexServeMux creates and returns a fully initialized regexServeMux.
func newRegexServeMux() *regexServeMux {
	return &regexServeMux{
		exact:    make(map[string]dns.HandlerFunc),
		wildcard: make([]struct{ prefix string; handler dns.HandlerFunc }, 0),
		regex:    make([]regexHandler, 0),
	}
}

// HandleFunc registers a handler for an exact domain or a wildcard pattern.
// Wildcard patterns start with "*." (e.g., "*.example.com").
// Exact patterns are stored using dns.CanonicalName as the key.
func (m *regexServeMux) HandleFunc(pattern string, handler dns.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.HasPrefix(pattern, "*.") {
		m.wildcard = append(m.wildcard, struct{ prefix string; handler dns.HandlerFunc }{
			prefix:  pattern[2:],
			handler: handler,
		})
		return
	}

	m.exact[dns.CanonicalName(pattern)] = handler
}

// HandleRegex registers a handler for a regex pattern.
// The pattern is compiled with regexp.Compile (not MustCompile) — invalid
// patterns return an error instead of panicking.
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

// HandleRemove removes a previously registered handler.
// For wildcard patterns (starting with "*."), it strips the "*." prefix
// (2 characters) and matches against the stored prefix.
// For exact patterns, it removes by the given key.
func (m *regexServeMux) HandleRemove(pattern string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.HasPrefix(pattern, "*.") {
		prefix := pattern[2:]
		for i, wc := range m.wildcard {
			if wc.prefix == prefix {
				m.wildcard = append(m.wildcard[:i], m.wildcard[i+1:]...)
				return
			}
		}
		return
	}

	delete(m.exact, pattern)
}

// SetCatchAll sets the fallback handler that is invoked when no other
// pattern matches.
func (m *regexServeMux) SetCatchAll(handler dns.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.catchAll = handler
}

// clear resets all handler registrations. Called during domainlist reloads.
func (m *regexServeMux) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.exact = make(map[string]dns.HandlerFunc)
	m.wildcard = m.wildcard[:0]
	m.regex = m.regex[:0]
	m.catchAll = nil
}

// ServeDNS routes an incoming DNS query through the priority-based matching
// pipeline: exact > wildcard > regex > catch-all.
func (m *regexServeMux) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		if m.catchAll != nil {
			m.catchAll(w, r)
		}
		return
	}

	question := r.Question[0].Name
	if !strings.HasSuffix(question, ".") {
		question = question + "."
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	canonical := strings.TrimSuffix(question, ".")

	// 1. Exact match
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
