package dns

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/bgp"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// testContext returns a context with a minimal test config embedded.
func testContext() context.Context {
	return context.WithValue(context.Background(), config.ConfigKey{}, config.TestConfig())
}

// fakeDNSServerForE2E creates a UDP DNS server that responds with an A record
// for ANY query. Returns the listen address.
func fakeDNSServerForE2E(t *testing.T, ip string) string {
	t.Helper()
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err, "listen UDP")

	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 {
			return
		}
		q := r.Question[0]
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP(ip).To4(),
		}
		m.Answer = append(m.Answer, a)
		_ = w.WriteMsg(m)
	})

	srv := &dns.Server{
		PacketConn: ln,
		Handler:    handler,
		Net:        "udp",
	}
	go srv.ActivateAndServe()
	t.Cleanup(func() { srv.Shutdown() })

	return ln.LocalAddr().String()
}

// setupE2EEnv creates the full E2E test environment:
// - fake DNS server responding for any query with the given IP
// - bgpSrv assigned to global _bgp for reference counting
// - cache with resolver pointing to fake DNS server
// - domain registered in cache mux (triggers auto-lookup for A + HTTPS)
// Returns (cache, fakeAddr, cleanupFunc).
// Tests must NOT use t.Parallel() — this modifies global _bgp.
func setupE2EEnv(t *testing.T, domain, ip string) *cache {
	t.Helper()
	initTestLogger()

	// 1. Create fake DNS server (responds to any query type)
	fakeAddr := fakeDNSServerForE2E(t, ip)

	// 2. Create bgpSrv with real loop goroutine for reference counting
	bgpSrv, cancel := bgp.NewBgpSrvForTest(t)
	_ = cancel
	bgp.SetBgpForTest(bgpSrv)

	// 3. Create cache with resolver pointing to fake DNS server
	l := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
	resolvers := newResolvers([]*net.UDPAddr{mustResolveUDP(t, fakeAddr)}, 0, &l)
	c := newCache(100, time.Duration(60), resolvers, &l, config.TestConfig(), loop.NewLoop(1, &l))

	// 4. Register domain in cache mux (triggers auto-lookup for A + HTTPS)
	require.NoError(t, c.register(domain))

	// 5. Start cache serve loop
	require.NoError(t, c.serve(testContext()))

	return c
}

// ---------------------------------------------------------------------------
// Test 1: DNS Query → Cache Resolution → BGP Advance
// ---------------------------------------------------------------------------

func TestE2E_DnsQueryToCacheToBgp(t *testing.T) {
	// Setup: fake DNS server + cache + bgpSrv
	c := setupE2EEnv(t, "example.com.", "10.0.0.1")
	t.Cleanup(func() { c.shutdown() })

	// Construct DNS query and call cache.resolve() directly
	msg := newTestMsg("example.com.", dns.TypeA)
	w := &testResponseWriter{}
	c.resolve(w, msg, false)

	// Verify: ResponseWriter received answer with IP 10.0.0.1
	assert.NotNil(t, w.msg, "response writer should have received a message")
	assert.Equal(t, dns.TypeA, w.msg.Question[0].Qtype, "response question type should be A")
	require.Len(t, w.msg.Answer, 1, "response should have exactly 1 answer")
	a, ok := w.msg.Answer[0].(*dns.A)
	require.True(t, ok, "answer should be an A record")
	assert.Equal(t, "10.0.0.1", a.A.String(), "A record should contain IP 10.0.0.1")

	// Verify: bgp.Advance() was called with ["10.0.0.1"]
	// Note: register() triggers auto-lookup for both A and HTTPS types,
	// creating two cache entries for the same domain. Each entry advances
	// the IP, so the reference count is 2.
	refCount := bgp.GetBgpRefCounter()
	require.Contains(t, refCount, "10.0.0.1", "IP 10.0.0.1 should be tracked in BGP")
	assert.Equal(t, uint64(2), refCount["10.0.0.1"], "reference count should be 2 (A + HTTPS entries)")
}

// ---------------------------------------------------------------------------
// Test 2: Multi-Domain IP Sharing (reference counting)
// ---------------------------------------------------------------------------

func TestE2E_MultiDomainIPSharing(t *testing.T) {
	initTestLogger()

	// 1. Create fake DNS server returning same IP for any query
	fakeAddr := fakeDNSServerForE2E(t, "10.0.0.1")

	// 2. Create bgpSrv for reference counting
	bgpSrv, cancel := bgp.NewBgpSrvForTest(t)
	_ = cancel
	bgp.SetBgpForTest(bgpSrv)

	// 3. Create cache with resolver pointing to fake DNS server
	l := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
	resolvers := newResolvers([]*net.UDPAddr{mustResolveUDP(t, fakeAddr)}, 0, &l)
	c := newCache(100, time.Duration(60), resolvers, &l, config.TestConfig(), loop.NewLoop(1, &l))

	// 4. Register both domains (each triggers auto-lookup for A + HTTPS)
	require.NoError(t, c.register("example.com."))
	require.NoError(t, c.register("test.com."))

	// 5. Start cache serve loop
	require.NoError(t, c.serve(testContext()))
	t.Cleanup(func() { c.shutdown() })

	// 6. At this point, register() created 4 cache entries (2 domains × 2 query types),
	//    all with IP 10.0.0.1. The reference count is 4.
	//    We verify the existing state directly.

	// Verify: IP is tracked (from auto-lookup creating A + HTTPS entries per domain)
	refCount := bgp.GetBgpRefCounter()
	assert.Contains(t, refCount, "10.0.0.1", "IP 10.0.0.1 should be tracked in BGP")
	assert.Equal(t, uint64(4), refCount["10.0.0.1"], "4 entries × 1 IP = ref count 4")

	// 7. Withdraw example.com (simulate domainlist change)
	//    This removes both A and HTTPS entries for example.com.
	require.NoError(t, c.unregister("example.com."))

	// 8. Verify: IP still tracked (test.com's 2 entries still reference it)
	refCount = bgp.GetBgpRefCounter()
	assert.Contains(t, refCount, "10.0.0.1", "IP 10.0.0.1 should still be tracked after example.com unregistered")
	assert.Equal(t, uint64(2), refCount["10.0.0.1"], "test.com's 2 entries still reference the IP")

	// 9. Withdraw test.com
	require.NoError(t, c.unregister("test.com."))

	// 10. Verify: IP removed (no more references)
	refCount = bgp.GetBgpRefCounter()
	assert.NotContains(t, refCount, "10.0.0.1", "IP 10.0.0.1 should be removed after all domains unregistered")
}

// ---------------------------------------------------------------------------
// Test 3: Cache Hit Returns Answer
// ---------------------------------------------------------------------------

func TestE2E_CacheHitReturnsAnswer(t *testing.T) {
	// Setup: fake DNS server + cache + bgpSrv
	c := setupE2EEnv(t, "example.com.", "10.0.0.1")
	t.Cleanup(func() { c.shutdown() })

	// First query: cache miss → hits fake DNS server → caches result
	msg1 := newTestMsg("example.com.", dns.TypeA)
	w1 := &testResponseWriter{}
	c.resolve(w1, msg1, false)

	// Verify first query response
	require.NotNil(t, w1.msg, "first query should return a response")
	require.Len(t, w1.msg.Answer, 1, "first query should have 1 answer")
	a1, ok := w1.msg.Answer[0].(*dns.A)
	require.True(t, ok, "answer should be an A record")
	assert.Equal(t, "10.0.0.1", a1.A.String(), "first query should resolve to 10.0.0.1")

	// Second query: cache hit → returns cached answer
	msg2 := newTestMsg("example.com.", dns.TypeA)
	w2 := &testResponseWriter{}
	c.resolve(w2, msg2, false)

	// Verify second query returns same answer
	require.NotNil(t, w2.msg, "second query should return a response")
	require.Len(t, w2.msg.Answer, 1, "second query should have 1 answer")
	a2, ok := w2.msg.Answer[0].(*dns.A)
	require.True(t, ok, "answer should be an A record")
	assert.Equal(t, "10.0.0.1", a2.A.String(), "second query should return cached IP 10.0.0.1")

	// Verify cache entry exists
	ce := c.get("example.com.", dns.TypeA)
	assert.NotNil(t, ce, "cache entry should exist after first query")
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func mustResolveUDP(t *testing.T, addr string) *net.UDPAddr {
	t.Helper()
	u, err := net.ResolveUDPAddr("udp", addr)
	require.NoError(t, err, "resolve UDP addr %q", addr)
	return u
}

func buildAnswerMsg(fqdn, ip string) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(fqdn, dns.TypeA)
	m.Authoritative = true
	a := &dns.A{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		A: net.ParseIP(ip).To4(),
	}
	m.Answer = append(m.Answer, a)
	return m
}
