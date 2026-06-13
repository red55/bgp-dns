package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// regexHandler wraps a compiled regex with its DNS handler function
type regexHandler struct {
	pattern *regexp.Regexp
	handler func(dns.ResponseWriter, *dns.Msg)
}

// regexServeMux extends dns.ServeMux with regex pattern support
type regexServeMux struct {
	mu        sync.RWMutex
	exact     map[string]dns.HandlerFunc      // exact FQDN handlers (used by existing domainlist entries)
	wildcard  []struct{ prefix string; handler dns.HandlerFunc } // wildcard handlers (*.example.com)
	regex     []regexHandler                   // regex pattern handlers
	catchAll  dns.HandlerFunc                  // fallback handler
}

func newRegexServeMux() *regexServeMux {
	return &regexServeMux{
		exact:    make(map[string]dns.HandlerFunc),
		wildcard: []struct{ prefix string; handler dns.HandlerFunc }{},
		regex:    []regexHandler{},
	}
}

// HandleRegex registers a regex pattern handler
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

// HandleFunc registers an exact FQDN handler (same as dns.HandleFunc)
func (m *regexServeMux) HandleFunc(pattern string, handler dns.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.HasSuffix(pattern, ".") {
		pattern = strings.TrimSuffix(pattern, ".")
	}

	// Check for wildcard pattern (*.example.com)
	if strings.HasPrefix(pattern, "*.") {
		wildcard := strings.TrimPrefix(pattern, "*.")
		m.wildcard = append(m.wildcard, struct{ prefix string; handler dns.HandlerFunc }{
			prefix: wildcard,
			handler: handler,
		})
		return
	}

	// Exact match
	m.exact[pattern] = handler
}

// HandleFunc sets the catch-all handler
func (m *regexServeMux) HandleFuncCatchAll(handler dns.HandlerFunc) {
	m.catchAll = handler
}

// ServeDNS routes incoming DNS queries to the appropriate handler
func (m *regexServeMux) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		if m.catchAll != nil {
			m.catchAll(w, r)
		}
		return
	}

	question := r.Question[0].Name

	// Normalize: ensure trailing dot
	if !strings.HasSuffix(question, ".") {
		question = question + "."
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Exact match (highest priority)
	if handler, ok := m.exact[strings.TrimSuffix(question, ".")]; ok {
		handler(w, r)
		return
	}

	// 2. Wildcard match
	for _, wc := range m.wildcard {
		if strings.HasSuffix(question, wc.prefix+".") {
			wc.handler(w, r)
			return
		}
	}

	// 3. Regex match (iterate in registration order)
	for _, rh := range m.regex {
		if rh.pattern.MatchString(strings.TrimSuffix(question, ".")) {
			rh.handler(w, r)
			return
		}
	}

	// 4. Catch-all (lowest priority)
	if m.catchAll != nil {
		m.catchAll(w, r)
	}
}

// --- Demo: DNS server with regex domainlist support ---

func main() {
	mux := newRegexServeMux()

	// Simulate domainlist entries (exact matches from file)
	mux.HandleFunc("cloudflare.com.", func(w dns.ResponseWriter, r *dns.Msg) {
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		rr, _ := dns.NewRR("cloudflare.com. 300 IN A 104.16.1.1")
		msg.Answer = append(msg.Answer, rr)
		w.WriteMsg(&msg)
	})

	mux.HandleFunc("google.com.", func(w dns.ResponseWriter, r *dns.Msg) {
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		rr, _ := dns.NewRR("google.com. 300 IN A 142.250.80.46")
		msg.Answer = append(msg.Answer, rr)
		w.WriteMsg(&msg)
	})

	// Regex pattern: intercept *.internal.corp and *.dev.corp
	// Matches any subdomain of internal.corp or dev.corp
	mux.HandleRegex(`^([a-z0-9-]+\.)?(internal|dev)\.corp$`, func(w dns.ResponseWriter, r *dns.Msg) {
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		rr, _ := dns.NewRR(fmt.Sprintf("%s 300 IN A 192.168.1.100", r.Question[0].Name))
		msg.Answer = append(msg.Answer, rr)
		w.WriteMsg(&msg)
	})

	// Regex pattern: intercept *.app.* domains (any subdomain of any TLD starting with "app.")
	mux.HandleRegex(`^app\.[a-z0-9-]+\.[a-z]{2,}$`, func(w dns.ResponseWriter, r *dns.Msg) {
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		rr, _ := dns.NewRR(fmt.Sprintf("%s 300 IN A 192.168.2.200", r.Question[0].Name))
		msg.Answer = append(msg.Answer, rr)
		w.WriteMsg(&msg)
	})

	// Catch-all: proxy to upstream for unmatched queries
	mux.HandleFuncCatchAll(func(w dns.ResponseWriter, r *dns.Msg) {
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Rcode = dns.RcodeNameError
		w.WriteMsg(&msg)
	})

	// Start DNS server
	server := &dns.Server{
		Addr:    "127.0.0.1:5553",
		Net:     "udp",
		Handler: mux,
	}

	fmt.Println("DNS server with regex domainlist support listening on 127.0.0.1:5553")
	fmt.Println()

	go func() {
		if err := server.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test queries
	client := new(dns.Client)
	tests := []struct {
		name    string
		query   string
		wantIP  string
		wantRcode string
	}{
		// Exact match (existing domainlist behavior)
		{"Exact: cloudflare.com", "cloudflare.com.", "104.16.1.1", ""},
		{"Exact: google.com", "google.com.", "142.250.80.46", ""},

		// Regex match: *.internal.corp
		{"Regex: api.internal.corp", "api.internal.corp.", "192.168.1.100", ""},
		{"Regex: db.internal.corp", "db.internal.corp.", "192.168.1.100", ""},
		{"Regex: internal.corp", "internal.corp.", "192.168.1.100", ""},

		// Regex match: *.dev.corp
		{"Regex: staging.dev.corp", "staging.dev.corp.", "192.168.1.100", ""},

		// Regex match: app.*.*
		{"Regex: app.prod.com", "app.prod.com.", "192.168.2.200", ""},
		{"Regex: app.staging.io", "app.staging.io.", "192.168.2.200", ""},

		// No match (catch-all NXDOMAIN)
		{"No match: unknown.com", "unknown.com.", "", "NXDOMAIN"},
	}

	fmt.Println("=== Test Results ===")
	passed := 0
	failed := 0

	for _, tt := range tests {
		m := new(dns.Msg)
		m.SetQuestion(tt.query, dns.TypeA)

		resp, _, err := client.Exchange(m, "127.0.0.1:5553")
		if err != nil {
			fmt.Printf("FAIL: %-30s error: %v\n", tt.name, err)
			failed++
			continue
		}

		if tt.wantIP != "" {
			for _, ans := range resp.Answer {
				if a, ok := ans.(*dns.A); ok {
					if a.A.String() == tt.wantIP {
						fmt.Printf("PASS: %-30s -> %s\n", tt.name, a.A.String())
						passed++
						goto next
					}
				}
			}
			fmt.Printf("FAIL: %-30s expected %s, got answer but no match\n", tt.name, tt.wantIP)
			failed++
		} else if tt.wantRcode != "" {
			if dns.RcodeToString[resp.Rcode] == tt.wantRcode {
				fmt.Printf("PASS: %-30s -> %s\n", tt.name, dns.RcodeToString[resp.Rcode])
				passed++
			} else {
				fmt.Printf("FAIL: %-30s expected %s, got %s\n", tt.name, tt.wantRcode, dns.RcodeToString[resp.Rcode])
				failed++
			}
		} else {
			fmt.Printf("FAIL: %-30s no expected result defined\n", tt.name)
			failed++
		}
	next:
	}

	fmt.Println()
	fmt.Printf("Results: %d passed, %d failed out of %d tests\n", passed, failed, len(tests))

	// Edge case: show regex compilation is done at registration time (not per-query)
	fmt.Println()
	fmt.Println("=== Performance Note ===")
	fmt.Println("Regex patterns are compiled once at registration time.")
	fmt.Println("Per-query cost is just regexp.MatchString() — O(n) where n = query length.")
	fmt.Println("For 1000 regex patterns, worst case is 1000 MatchString calls per query.")
	fmt.Println("Mitigation: compile patterns eagerly, cache compiled regex objects.")

	server.Shutdown()
	fmt.Printf("\nOverall: %s\n", map[bool]string{true: "ALL TESTS PASSED", false: "SOME TESTS FAILED"}[failed == 0])

	if failed > 0 {
		os.Exit(1)
	}
}
