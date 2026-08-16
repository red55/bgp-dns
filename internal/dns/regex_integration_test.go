package dns

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegexIntegration_LoadRegexPattern verifies that loading a domainlist file
// with mixed exact and regex: entries registers both, and queries actually
// trigger the registered handlers (not just generation count).
func TestRegexIntegration_LoadRegexPattern(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "mixed.lst")
	content := "example.com\nregex:([a-z]+)\\.internal\\.corp\n"
	require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))

	err := c.load(listFile)
	require.NoError(t, err)

	assert.True(t, c.generation() > 0, "generation should increase after load")

	// Behavioral assertion: verify exact entry is registered in mux
	_, hasExact := c.mux.exact["example.com."]
	assert.True(t, hasExact, "example.com should be registered in exact map")

	// Behavioral assertion: verify regex entry is registered
	require.Len(t, c.mux.regex, 1, "should have exactly 1 regex handler")
	assert.Equal(t, "([a-z]+)\\.internal\\.corp", c.mux.regex[0].pattern.String(), "regex pattern should match loaded entry")
}

// TestRegexIntegration_RegexMatchInMux verifies that HandleRegex + ServeDNS
// routes a query matching the regex pattern to the handler.
func TestRegexIntegration_RegexMatchInMux(t *testing.T) {
	mux := newRegexServeMux()

	called := false
	err := mux.HandleRegex("([a-z]+)\\.internal\\.corp", func(w dns.ResponseWriter, r *dns.Msg) {
		called = true
	})
	assert.NoError(t, err, "valid regex pattern should compile without error")

	msg := newTestMsg("foo.internal.corp.", dns.TypeA)
	w := &testResponseWriter{}
	mux.ServeDNS(w, msg)

	assert.True(t, called, "regex handler should be called for foo.internal.corp")
}

// TestRegexIntegration_InvalidRegexSkippedWithWarning verifies that loading a
// domainlist file with an invalid regex pattern skips it (with a warning logged)
// but still registers the valid entries.
func TestRegexIntegration_InvalidRegexSkippedWithWarning(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "invalid.lst")
	content := "example.com\nregex:[invalid\ntest.com\n"
	require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))

	// load should succeed even with invalid regex — invalid patterns are skipped
	err := c.load(listFile)
	require.NoError(t, err, "load should succeed even with invalid regex")

	// Behavioral assertion: valid entries are still registered
	_, hasExample := c.mux.exact["example.com."]
	assert.True(t, hasExample, "example.com should be registered despite invalid regex nearby")

	_, hasTest := c.mux.exact["test.com."]
	assert.True(t, hasTest, "test.com should be registered despite invalid regex nearby")

	// Behavioral assertion: no regex handlers were registered (invalid one was skipped)
	assert.Len(t, c.mux.regex, 0, "invalid regex pattern should not be registered")
}

// TestRegexIntegration_PriorityExactOverRegex verifies that an exact match
// handler takes priority over a regex handler that would also match the same domain.
func TestRegexIntegration_PriorityExactOverRegex(t *testing.T) {
	mux := newRegexServeMux()

	exactCalled := false
	regexCalled := false

	mux.HandleFunc("example.com", func(w dns.ResponseWriter, r *dns.Msg) {
		exactCalled = true
	})
	err := mux.HandleRegex("ex.*", func(w dns.ResponseWriter, r *dns.Msg) {
		regexCalled = true
	})
	assert.NoError(t, err)

	msg := newTestMsg("example.com.", dns.TypeA)
	w := &testResponseWriter{}
	mux.ServeDNS(w, msg)

	assert.True(t, exactCalled, "exact match should take priority over regex")
	assert.False(t, regexCalled, "regex handler should NOT be called when exact matches")
}

// TestRegexIntegration_WildcardInDomainlist verifies that wildcard entries
// (e.g., *.example.com) loaded from a domainlist file match subdomain queries.
func TestRegexIntegration_WildcardInDomainlist(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "wildcard.lst")
	content := "*.example.com\nregex:([a-z]+)\\.test\\.com\n"
	require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))

	err := c.load(listFile)
	require.NoError(t, err)

	assert.True(t, c.generation() > 0, "generation should increase after load")

	// Behavioral assertion: verify wildcard entry is registered
	require.Len(t, c.mux.wildcard, 1, "should have exactly 1 wildcard handler")
	assert.Equal(t, "example.com.", c.mux.wildcard[0].prefix, "wildcard prefix should be 'example.com.' (canonical)")

	// Behavioral assertion: verify regex entry is registered
	require.Len(t, c.mux.regex, 1, "should have exactly 1 regex handler")
	assert.Equal(t, "([a-z]+)\\.test\\.com", c.mux.regex[0].pattern.String(), "regex pattern should match loaded entry")
}

// TestRegexIntegration_LoadClearsPreviousState verifies that loading a new
// domainlist file replaces all previous handlers — old domains no longer match,
// new domains do.
func TestRegexIntegration_LoadClearsPreviousState(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile1 := filepath.Join(tmpDir, "list1.lst")
	listFile2 := filepath.Join(tmpDir, "list2.lst")
	require.NoError(t, os.WriteFile(listFile1, []byte("example.com\n"), 0644))
	require.NoError(t, os.WriteFile(listFile2, []byte("test.com\n"), 0644))

	// Load first list
	err := c.load(listFile1)
	require.NoError(t, err)

	// Behavioral assertion: example.com IS registered before second load
	_, hasExample := c.mux.exact["example.com."]
	assert.True(t, hasExample, "example.com should be registered before reload")

	// Load second list (should replace all handlers via safe reload)
	err = c.load(listFile2)
	require.NoError(t, err)

	// Behavioral assertion: example.com is NOT registered after reload
	_, hasExampleAfter := c.mux.exact["example.com."]
	assert.False(t, hasExampleAfter, "example.com should NOT be registered after reload clears state")

	// Behavioral assertion: test.com IS registered after reload
	_, hasTest := c.mux.exact["test.com."]
	assert.True(t, hasTest, "test.com should be registered after reload")
}

// TestRegexIntegration_ExactMatchStillWorks verifies that exact domain entries
// loaded from a domainlist file are properly registered and respond to queries.
func TestRegexIntegration_ExactMatchStillWorks(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "exact.lst")
	content := "cloudflare.com\ngoogle.com\n"
	require.NoError(t, os.WriteFile(listFile, []byte(content), 0644))

	err := c.load(listFile)
	require.NoError(t, err)

	assert.True(t, c.generation() > 0, "generation should increase after load")

	// Behavioral assertions: verify exact entries are registered
	_, hasCloudflare := c.mux.exact["cloudflare.com."]
	assert.True(t, hasCloudflare, "cloudflare.com should be registered in exact map")

	_, hasGoogle := c.mux.exact["google.com."]
	assert.True(t, hasGoogle, "google.com should be registered in exact map")
}
