package dns

import (
	"net"
	"sync/atomic"
	"testing"

	"github.com/miekg/dns"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// initTestLogger is already declared in cache_test.go; we use it directly.

// newTestResolvers builds a *resolvers from a list of UDP address strings.
func newTestResolvers(t *testing.T, addrs []string) *resolvers {
	t.Helper()
	// Ensure logger is initialized (defined in cache_test.go, same package).
	initTestLogger()

	udpAddrs := make([]*net.UDPAddr, len(addrs))
	for i, a := range addrs {
		addr, err := net.ResolveUDPAddr("udp", a)
		if err != nil {
			t.Fatalf("resolve %q: %v", a, err)
		}
		udpAddrs[i] = addr
	}
	return newResolvers(udpAddrs)
}

// fakeDNSServer creates a DNS server that calls the given handler.
// Returns (*dns.Server, listenAddress) — caller must call Shutdown() via t.Cleanup.
func newFakeDNSServer(t *testing.T, handler dns.HandlerFunc) (*dns.Server, string) {
	t.Helper()
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &dns.Server{
		PacketConn: ln,
		Handler:    handler,
		Net:        "udp",
	}
	go srv.ActivateAndServe()
	t.Cleanup(func() { srv.Shutdown() })
	return srv, ln.LocalAddr().String()
}

// makeSuccessMsg builds a DNS response with an A record answer.
func makeSuccessMsg(q *dns.Msg) *dns.Msg {
	r := new(dns.Msg)
	r.SetReply(q)
	r.Authoritative = true
	a := &dns.A{
		Hdr: dns.RR_Header{
			Name:   q.Question[0].Name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		A: net.ParseIP("10.0.0.1").To4(),
	}
	r.Answer = append(r.Answer, a)
	return r
}

// makeErrorResponse builds a DNS response with SERVFAIL.
func makeErrorResponse(q *dns.Msg) *dns.Msg {
	r := new(dns.Msg)
	r.SetReply(q)
	r.Rcode = dns.RcodeServerFailure
	return r
}

// ---------------------------------------------------------------------------
// Test: SingleSuccess
// ---------------------------------------------------------------------------

// TestResolver_SingleSuccess verifies that a single healthy resolver
// returns a valid answer and stays marked okay.
func TestResolver_SingleSuccess(t *testing.T) {
	addr := startFakeServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		w.WriteMsg(makeSuccessMsg(r))
	})

	rs := newTestResolvers(t, []string{addr})
	q := newTestMsg("example.com.", dns.TypeA)

	resp, err := rs.query(q)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer in response")
	}

	// Verify resolver is marked okay.
	rs.m.RLock()
	rs.rs.Do(func(v interface{}) {
		r := v.(*resolver)
		if !r.isOk() {
			t.Error("expected resolver to be marked okay after successful query")
		}
	})
	rs.m.RUnlock()
}

// ---------------------------------------------------------------------------
// Test: Failover
// ---------------------------------------------------------------------------

// TestResolver_Failover verifies that when the first resolver fails,
// the ring rotates and the second resolver is used successfully.
func TestResolver_Failover(t *testing.T) {
	var failFirst atomic.Bool
	failFirst.Store(true)

	// First resolver always fails.
	srv1, addr1 := newFakeDNSServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		w.WriteMsg(makeErrorResponse(r))
	})
	t.Cleanup(func() { srv1.Shutdown() })

	// Second resolver succeeds.
	srv2, addr2 := newFakeDNSServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		w.WriteMsg(makeSuccessMsg(r))
	})
	t.Cleanup(func() { srv2.Shutdown() })

	rs := newTestResolvers(t, []string{addr1, addr2})
	q := newTestMsg("example.com.", dns.TypeA)

	resp, err := rs.query(q)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer in response")
	}

	// Verify ring state: the successful resolver (addr2) should be marked okay,
	// and the failed resolver (addr1) should be marked failed.
	rs.m.RLock()
	var headAddr, nextAddr string
	var headOk, nextOk bool
	rs.rs.Do(func(v interface{}) {
		r := v.(*resolver)
		if r.addr.String() == addr1 {
			headAddr = r.addr.String()
			headOk = r.isOk()
		} else {
			nextAddr = r.addr.String()
			nextOk = r.isOk()
		}
	})
	rs.m.RUnlock()

	if headAddr != addr1 {
		t.Errorf("expected addr1 in ring, got %s", headAddr)
	}
	if headOk {
		t.Error("expected first resolver to be marked failed after query error")
	}
	if nextAddr != addr2 {
		t.Errorf("expected addr2 in ring, got %s", nextAddr)
	}
	if !nextOk {
		t.Error("expected second resolver to be marked okay after successful query")
	}
}

// ---------------------------------------------------------------------------
// Test: Recovery
// ---------------------------------------------------------------------------

// TestResolver_Recovery verifies that a resolver marked as failed
// recovers when it starts responding successfully again.
func TestResolver_Recovery(t *testing.T) {
	var failServer atomic.Bool
	failServer.Store(true)

	// Single fake server that can toggle between failure and success.
	srv, addr := newFakeDNSServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		if failServer.Load() {
			w.WriteMsg(makeErrorResponse(r))
		} else {
			w.WriteMsg(makeSuccessMsg(r))
		}
	})
	t.Cleanup(func() { srv.Shutdown() })

	rs := newTestResolvers(t, []string{addr})
	q := newTestMsg("example.com.", dns.TypeA)

	// First query: server is failing → should get error.
	_, err := rs.query(q)
	if err == nil {
		t.Fatal("expected error from failing server")
	}

	// Verify resolver is marked as failed.
	rs.m.RLock()
	rs.rs.Do(func(v interface{}) {
		r := v.(*resolver)
		if r.isOk() {
			t.Error("expected resolver to be marked failed after query error")
		}
	})
	rs.m.RUnlock()

	// Flip the server to success.
	failServer.Store(false)

	// Second query: server now succeeds → should recover.
	resp, err := rs.query(q)
	if err != nil {
		t.Fatalf("query failed after recovery: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatal("expected answer in response after recovery")
	}

	// Verify resolver is marked okay again.
	rs.m.RLock()
	rs.rs.Do(func(v interface{}) {
		r := v.(*resolver)
		if !r.isOk() {
			t.Error("expected resolver to be marked okay after successful recovery query")
		}
	})
	rs.m.RUnlock()
}

// ---------------------------------------------------------------------------
// Test: AllFail
// ---------------------------------------------------------------------------

// TestResolver_AllFail verifies that when all resolvers fail,
// an error is returned and all are marked as failed.
func TestResolver_AllFail(t *testing.T) {
	srv1, addr1 := newFakeDNSServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		w.WriteMsg(makeErrorResponse(r))
	})
	t.Cleanup(func() { srv1.Shutdown() })

	srv2, addr2 := newFakeDNSServer(t, func(w dns.ResponseWriter, r *dns.Msg) {
		w.WriteMsg(makeErrorResponse(r))
	})
	t.Cleanup(func() { srv2.Shutdown() })

	rs := newTestResolvers(t, []string{addr1, addr2})
	q := newTestMsg("example.com.", dns.TypeA)

	_, err := rs.query(q)
	if err == nil {
		t.Fatal("expected error when all resolvers fail")
	}

	// Verify both resolvers are marked as failed.
	rs.m.RLock()
	count := 0
	rs.rs.Do(func(v interface{}) {
		r := v.(*resolver)
		if !r.isOk() {
			count++
		}
	})
	rs.m.RUnlock()

	if count != 2 {
		t.Errorf("expected 2 failed resolvers, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Convenience: start a server that returns a controlled response
// ---------------------------------------------------------------------------

// startFakeServer creates a fake DNS server that returns a success response
// for every query. Returns the listen address.
func startFakeServer(t *testing.T, handler dns.HandlerFunc) string {
	srv, addr := newFakeDNSServer(t, handler)
	t.Cleanup(func() { srv.Shutdown() })
	return addr
}
