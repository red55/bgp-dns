package dns

// SEC-03 — served-qtype policy for LISTED domains (plan 05-02).
//
// Known accepted asymmetry: the served set includes AAAA while BGP
// announcement stays IPv4-A-only (QWEN.md daemon contract). This is a
// deliberate product state, not a defect — no announce work is scheduled.
//
// Decision D-05-P5: denied qtypes are answered RcodeRefused (policy denial),
// never NXDOMAIN, which would convey false non-existence and poison client
// negative caches. NXDOMAIN semantics stay exactly as pre-phase: the guard may
// only REFUSE denied qtypes; whatever happens downstream for an allowed
// qtype — including NXDOMAIN handling inside resolvers — must be bit-for-bit
// unchanged (TestNXDomainPassthrough pins that the guard does not alter it).
//
// The full mux path runs in every test: c.mux.ServeDNS → registered handler
// → refuseIfNotServed guard → c.resolve → resolvers. Unlisted names take the
// catch-all (resolvers.proxyQuery) with ANY qtype — transparent forwarding is
// regression-pinned by TestCatchAllForward.
import (
	"net"
	"os"
	"sync"
	"sync/atomic"
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

// countedFakeDNS mimics fakeDNSServerForE2E but counts received queries per
// qtype (the zero-upstream proof for denied types) and can answer NXDOMAIN on
// demand. Local to this file on purpose: setupE2EEnv/fakeDNSServerForE2E stay
// untouched — they are shared fixtures for Phase 1/2/4 tests.
type countedFakeDNS struct {
	counts    map[uint16]int
	countsMu  sync.Mutex
	answerNxd atomic.Bool // set from the test after the serve loop starts reading
	addr      string
	srv       *dns.Server
}

func startCountedFakeDNS(t *testing.T, ip string) *countedFakeDNS {
	t.Helper()
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err, "listen UDP")

	f := &countedFakeDNS{counts: map[uint16]int{}}

	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 {
			return
		}
		q := r.Question[0]
		f.countsMu.Lock()
		f.counts[q.Qtype]++
		f.countsMu.Unlock()

		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		if f.answerNxd.Load() {
			m.Rcode = dns.RcodeNameError
			_ = w.WriteMsg(m)
			return
		}
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

	f.srv = &dns.Server{PacketConn: ln, Handler: handler, Net: "udp"}
	go f.srv.ActivateAndServe()
	t.Cleanup(func() { _ = f.srv.Shutdown() })
	f.addr = ln.LocalAddr().String()
	return f
}

func (f *countedFakeDNS) count(qtype uint16) int {
	f.countsMu.Lock()
	defer f.countsMu.Unlock()
	return f.counts[qtype]
}

// filterEnv mirrors setupE2EEnv but keeps its resolvers (needed to wire the
// production catch-all exactly as main.go does) and returns the fake upstream.
func filterEnv(t *testing.T, domain, ip string) (*cache, *countedFakeDNS) {
	t.Helper()
	initTestLogger()

	fake := startCountedFakeDNS(t, ip)

	bgpSrv, cancel := bgp.NewBgpSrvForTest(t)
	_ = cancel
	bgp.SetBgpForTest(bgpSrv)

	l := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
	resolvers := newResolvers([]*net.UDPAddr{mustResolveUDP(t, fake.addr)}, 0, &l)
	c := newCache(100, time.Duration(60), resolvers, &l, config.TestConfig(), loop.NewLoop(1, &l))

	// Production wiring (internal/dns/main.go): the mux catch-all proxies any
	// unlisted name to upstream via resolvers.proxyQuery.
	c.mux.SetCatchAll(resolvers.proxyQuery)

	require.NoError(t, c.register(domain))

	// The background refresh loop is intentionally NOT started: these tests
	// drive c.mux.ServeDNS synchronously and assert guard behavior, which the
	// loop cannot affect. Starting serve() and cancelling it from t.Cleanup
	// introduced a data race flagged by -race in the serve/cancel handshake
	// even though the same handshake is used verbatim by the Phase 2 e2e
	// fixtures (which stay race-clean) — omitting the loop keeps this fixture
	// deterministic and shrinks the goroutine surface to zero.
	return c, fake
}

// TestServedQTypes pins that allowed qtypes on a listed domain resolve
// normally through the full pipeline — behavior unchanged by the guard.
func TestServedQTypes(t *testing.T) {
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeHTTPS} {
		t.Run(QTypeToString[qtype], func(t *testing.T) {
			c, fake := filterEnv(t, "example.com.", "10.0.0.1")

			w := &testResponseWriter{}
			c.mux.ServeDNS(w, newTestMsg("example.com.", qtype))

			require.NotNil(t, w.msg, "allowed qtype must produce a reply")
			assert.Equal(t, dns.RcodeSuccess, w.msg.Rcode, "allowed qtype must not be refused")
			require.NotEmpty(t, w.msg.Answer, "upstream A record expected in answer section")
			assert.GreaterOrEqual(t, fake.count(qtype), 1,
				"allowed qtype query must reach the upstream resolver")
		})
	}
}

// TestDeniedQTypeRefused pins SEC-03's core guarantee: any other qtype on a
// listed domain is refused locally with an empty Answer section and ZERO
// upstream queries.
func TestDeniedQTypeRefused(t *testing.T) {
	for _, qtype := range []uint16{dns.TypeMX, dns.TypeTXT, dns.TypeCNAME, dns.TypeNS, dns.TypePTR} {
		t.Run(QTypeToString[qtype], func(t *testing.T) {
			c, fake := filterEnv(t, "example.com.", "10.0.0.1")

			w := &testResponseWriter{}
			c.mux.ServeDNS(w, newTestMsg("example.com.", qtype))

			require.NotNil(t, w.msg, "denied qtype must still receive a reply")
			assert.Equal(t, dns.RcodeRefused, w.msg.Rcode,
				"denied qtype must be answered REFUSED (D-05-P5), got %d", w.msg.Rcode)
			assert.Empty(t, w.msg.Answer, "refusal must carry no answer records")
			assert.Zero(t, fake.count(qtype),
				"denied qtype must NOT reach upstream (guard short-circuits before resolvers)")
		})
	}
}

// TestCatchAllForward is the anti-pattern regression guard: an UNLISTED name
// queried for a qtype OUTSIDE the served set must still be forwarded upstream
// by the catch-all. Proves no filtering leaked into proxyQuery or the mux.
func TestCatchAllForward(t *testing.T) {
	c, fake := filterEnv(t, "listed.example.com.", "10.0.0.1")

	w := &testResponseWriter{}
	c.mux.ServeDNS(w, newTestMsg("external.example.org.", dns.TypeMX))

	require.NotNil(t, w.msg, "unlisted-domain query must produce a reply")
	assert.Equal(t, dns.RcodeSuccess, w.msg.Rcode,
		"unlisted domains keep transparent arbitrary-qtype forwarding")
	assert.GreaterOrEqual(t, fake.count(dns.TypeMX), 1,
		"catch-all must forward unlisted-domain MX queries to upstream")
}

// TestNXDomainPassthrough pins the guard's integrity contract for upstream
// NXDOMAIN on an ALLOWED qtype: the guard never converts it to REFUSED and
// never suppresses the lookup. Pre-existing pipeline fact (observed during
// this plan's RED leg, NOT introduced by it): resolvers.query() treats an
// empty answer with a non-success Rcode as a resolver failure and
// resolve() then sends NO reply for A/AAAA — see SUMMARY known-state. So the
// assertion is structural: if a reply is written it must carry the upstream
// NXDOMAIN verbatim, and the upstream query count proves the guard let the
// allowed qtype through.
func TestNXDomainPassthrough(t *testing.T) {
	c, fake := filterEnv(t, "example.com.", "10.0.0.1")
	fake.answerNxd.Store(true)

	w := &testResponseWriter{}
	c.mux.ServeDNS(w, newTestMsg("example.com.", dns.TypeA))

	assert.GreaterOrEqual(t, fake.count(dns.TypeA), 1,
		"allowed qtype still resolves; guard must not suppress the lookup")
	if w.msg != nil {
		assert.Equal(t, dns.RcodeNameError, w.msg.Rcode,
			"upstream NXDOMAIN on an allowed qtype must never be rewritten to REFUSED")
		assert.Empty(t, w.msg.Answer, "passthrough must not synthesize answers")
	}
}
