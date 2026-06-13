package dns

import (
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

// testResponseWriter is a minimal dns.ResponseWriter mock for unit tests.
type testResponseWriter struct {
	msg *dns.Msg
}

func (w *testResponseWriter) LocalAddr() net.Addr                         { return &net.UDPAddr{} }
func (w *testResponseWriter) RemoteAddr() net.Addr                        { return &net.UDPAddr{} }
func (w *testResponseWriter) WriteMsg(r *dns.Msg) error                   { w.msg = r; return nil }
func (w *testResponseWriter) Write([]byte) (int, error)                   { return 0, nil }
func (w *testResponseWriter) Close() error                                { return nil }
func (w *testResponseWriter) TsigStatus() error                           { return nil }
func (w *testResponseWriter) TsigTimersOnly(bool)                         {}
func (w *testResponseWriter) Hijack()                                     {}
func (w *testResponseWriter) Network() string                             { return "udp" }
func (w *testResponseWriter) CloseNotify() <-chan bool                    { return nil }
func (w *testResponseWriter) Requested() string                           { return "" }

// newTestMsg creates a DNS query message for the given FQDN and type.
func newTestMsg(fqdn string, qtype uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(fqdn, qtype)
	return m
}

// newTestMsgNoQuestion creates a DNS message with no questions.
func newTestMsgNoQuestion() *dns.Msg {
	return new(dns.Msg)
}

func TestServeMux_ExactMatch(t *testing.T) {
	mux := newRegexServeMux()
	called := false
	mux.HandleFunc("example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("example.com.", dns.TypeA))
	assert.True(t, called, "exact handler should be called for matching domain")
}

func TestServeMux_WildcardMatch(t *testing.T) {
	mux := newRegexServeMux()
	called := false
	mux.HandleFunc("*.example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("foo.example.com.", dns.TypeA))
	assert.True(t, called, "wildcard handler should match subdomains")
}

func TestServeMux_RegexMatch(t *testing.T) {
	mux := newRegexServeMux()
	called := false
	err := mux.HandleRegex("([a-z]+)\\.internal\\.corp", func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})
	assert.NoError(t, err, "valid regex pattern should compile without error")

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("foo.internal.corp.", dns.TypeA))
	assert.True(t, called, "regex handler should match pattern")
}

func TestServeMux_CatchAll(t *testing.T) {
	mux := newRegexServeMux()
	called := false
	mux.SetCatchAll(func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("unknown.com.", dns.TypeA))
	assert.True(t, called, "catch-all handler should be invoked for unmatched domains")
}

func TestServeMux_PriorityExactOverWildcard(t *testing.T) {
	mux := newRegexServeMux()
	exactCalled := false
	wildcardCalled := false

	mux.HandleFunc("example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		exactCalled = true
	})
	mux.HandleFunc("*.example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		wildcardCalled = true
	})

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("example.com.", dns.TypeA))

	assert.True(t, exactCalled, "exact handler should take priority over wildcard")
	assert.False(t, wildcardCalled, "wildcard handler should NOT be called when exact matches")
}

func TestServeMux_PriorityWildcardOverRegex(t *testing.T) {
	mux := newRegexServeMux()
	wildcardCalled := false
	regexCalled := false

	mux.HandleFunc("*.corp", func(w dns.ResponseWriter, r *dns.Msg) {
		wildcardCalled = true
	})
	mux.HandleRegex("([a-z]+)\\.corp", func(w dns.ResponseWriter, r *dns.Msg) {
		regexCalled = true
	})

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("foo.corp.", dns.TypeA))

	assert.True(t, wildcardCalled, "wildcard should take priority over regex")
	assert.False(t, regexCalled, "regex handler should NOT be called when wildcard matches")
}

func TestServeMux_PriorityRegexOverCatchAll(t *testing.T) {
	mux := newRegexServeMux()
	regexCalled := false
	catchAllCalled := false

	mux.HandleRegex("([a-z]+)\\.test", func(w dns.ResponseWriter, r *dns.Msg) {
		regexCalled = true
	})
	mux.SetCatchAll(func(w dns.ResponseWriter, r *dns.Msg) {
		catchAllCalled = true
	})

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("foo.test.", dns.TypeA))

	assert.True(t, regexCalled, "regex should take priority over catch-all")
	assert.False(t, catchAllCalled, "catch-all should NOT be called when regex matches")
}

func TestServeMux_EmptyQuestion(t *testing.T) {
	mux := newRegexServeMux()
	called := false
	mux.SetCatchAll(func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsgNoQuestion())
	assert.True(t, called, "catch-all should be invoked for messages with no questions")
}

func TestServeMux_HandleRemoveExact(t *testing.T) {
	mux := newRegexServeMux()
	called := false
	mux.HandleFunc("example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})

	// Remove the exact handler
	mux.HandleRemove("example.com")

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("example.com.", dns.TypeA))
	assert.False(t, called, "exact handler should be removed and not called")
}

func TestServeMux_HandleRemoveWildcard(t *testing.T) {
	mux := newRegexServeMux()
	called := false
	mux.HandleFunc("*.example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})

	// Remove the wildcard handler (HandleRemove strips "*." prefix)
	mux.HandleRemove("*.example.com")

	w := &testResponseWriter{}
	mux.ServeDNS(w, newTestMsg("foo.example.com.", dns.TypeA))
	assert.False(t, called, "wildcard handler should be removed and not called")
}

func TestServeMux_HandleRegexInvalid(t *testing.T) {
	mux := newRegexServeMux()
	err := mux.HandleRegex("[invalid", func(w dns.ResponseWriter, r *dns.Msg) {})

	assert.Error(t, err, "invalid regex pattern should return an error")
	assert.Contains(t, err.Error(), "invalid regex", "error message should describe the invalid pattern")
}

func TestServeMux_Clear(t *testing.T) {
	mux := newRegexServeMux()

	exactCalled := false
	wildcardCalled := false
	regexCalled := false
	catchAllCalled := false

	mux.HandleFunc("example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		exactCalled = true
	})
	mux.HandleFunc("*.example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		wildcardCalled = true
	})
	mux.HandleRegex("([a-z]+)\\.test", func(w dns.ResponseWriter, r *dns.Msg) {
		regexCalled = true
	})
	mux.SetCatchAll(func(w dns.ResponseWriter, r *dns.Msg) {
		catchAllCalled = true
	})

	// Clear all handlers
	mux.clear()

	w := &testResponseWriter{}

	// None of the handlers should fire after clear
	mux.ServeDNS(w, newTestMsg("example.com.", dns.TypeA))
	assert.False(t, exactCalled, "exact handler should be cleared")

	mux.ServeDNS(w, newTestMsg("foo.example.com.", dns.TypeA))
	assert.False(t, wildcardCalled, "wildcard handler should be cleared")

	mux.ServeDNS(w, newTestMsg("foo.test.", dns.TypeA))
	assert.False(t, regexCalled, "regex handler should be cleared")

	mux.ServeDNS(w, newTestMsg("unknown.com.", dns.TypeA))
	assert.False(t, catchAllCalled, "catch-all handler should be cleared")
}
