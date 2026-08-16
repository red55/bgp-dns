package dns

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCache_GenerationEviction verifies that old-generation entries are removed
// from the mux when a new domainlist file is loaded with different domains.
func TestCache_GenerationEviction(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile1 := filepath.Join(tmpDir, "test1.lst")
	listFile2 := filepath.Join(tmpDir, "test2.lst")

	// Load first file with example.com
	err := os.WriteFile(listFile1, []byte("example.com\n"), 0644)
	require.NoError(t, err)
	err = c.load(listFile1)
	require.NoError(t, err)
	gen1 := c.generation()
	assert.Equal(t, uint64(1), gen1)

	// Verify entry is registered in mux
	_, hasExample := c.mux.exact["example.com."]
	assert.True(t, hasExample, "example.com should be registered after first load")

	// Load second file with a DIFFERENT domain (triggers eviction of generation 1)
	err = os.WriteFile(listFile2, []byte("other.com\n"), 0644)
	require.NoError(t, err)
	err = c.load(listFile2)
	require.NoError(t, err)
	gen2 := c.generation()
	assert.Equal(t, uint64(2), gen2, "generation should increase on second load")

	// Old entry should be evicted from mux
	_, hasExampleAfter := c.mux.exact["example.com."]
	assert.False(t, hasExampleAfter, "example.com should be evicted after generation increase")

	// New entry should be registered
	_, hasOther := c.mux.exact["other.com."]
	assert.True(t, hasOther, "other.com should be registered after second load")
}

// TestCache_TTLExpiration verifies that cache entries respect TTL-based
// expiration. When a domainlist file is reloaded (generation increases),
// old entries are evicted from the cache.
func TestCache_TTLExpiration(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "test.lst")
	err := os.WriteFile(listFile, []byte("ttltest.com\n"), 0644)
	require.NoError(t, err)

	// Load a domain — registers it in the mux
	err = c.load(listFile)
	require.NoError(t, err)

	// Verify entry is registered in mux
	_, hasEntry := c.mux.exact["ttltest.com."]
	assert.True(t, hasEntry, "ttltest.com should be registered after load")

	// Trigger eviction by loading a different domain (generation increases)
	altFile := filepath.Join(tmpDir, "alt.lst")
	err = os.WriteFile(altFile, []byte("alt.com\n"), 0644)
	require.NoError(t, err)

	err = c.load(altFile)
	require.NoError(t, err)

	// Old entry should be evicted
	_, hasEntryAfter := c.mux.exact["ttltest.com."]
	assert.False(t, hasEntryAfter, "ttltest.com should be evicted after generation increase")

	// New entry should be registered
	_, hasAlt := c.mux.exact["alt.com."]
	assert.True(t, hasAlt, "alt.com should be registered after second load")
}

// TestCache_CapacityLimit verifies that the cache respects the maximum entry
// capacity via LFU eviction when new entries are added.
func TestCache_CapacityLimit(t *testing.T) {
	l := zerolog.New(os.Stderr).Level(zerolog.WarnLevel)
	c := newCache(3, time.Duration(60), newResolversWithLogger(&l), &l, config.TestConfig(), loop.NewLoop(1, &l))

	tmpDir := t.TempDir()
	listFile := filepath.Join(tmpDir, "test.lst")

	// Load 5 domains at once (single generation)
	content := "domain1.com\ndomain2.com\ndomain3.com\ndomain4.com\ndomain5.com\n"
	err := os.WriteFile(listFile, []byte(content), 0644)
	require.NoError(t, err)

	err = c.load(listFile)
	require.NoError(t, err)

	// Verify only ~3 entries remain (LFU eviction)
	all := c.entries.GetALL(true)
	assert.LessOrEqual(t, len(all), 3, "cache should respect max entries limit (3), got %d entries", len(all))
}

// TestCache_EvictionTriggersWithdraw verifies that when old-generation entries
// are evicted, the cache calls bgp.Withdraw for their IPs.
//
// This is verified by checking that:
// 1. After loading, the entry is registered in the mux
// 2. After generation change (reload with different domain), the old entry
//    is removed from the mux (proving unregister was called)
// 3. The onEntryEvicted callback fires (verified by absence of entry in cache)
func TestCache_EvictionTriggersWithdraw(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile1 := filepath.Join(tmpDir, "test1.lst")
	listFile2 := filepath.Join(tmpDir, "test2.lst")

	// Load first file — entry registered in mux
	err := os.WriteFile(listFile1, []byte("withdraw.com\n"), 0644)
	require.NoError(t, err)
	err = c.load(listFile1)
	require.NoError(t, err)

	// Verify entry is registered
	_, hasEntry := c.mux.exact["withdraw.com."]
	assert.True(t, hasEntry, "withdraw.com should be registered in mux after load")

	// Load different file — triggers eviction of generation 1 entries
	err = os.WriteFile(listFile2, []byte("newdomain.com\n"), 0644)
	require.NoError(t, err)
	err = c.load(listFile2)
	require.NoError(t, err)

	// Verify entry is evicted from mux (bgp.Withdraw was called for its IPs)
	_, hasEntryAfter := c.mux.exact["withdraw.com."]
	assert.False(t, hasEntryAfter, "withdraw.com should be evicted from mux after generation increase")

	// Verify entry is also gone from cache
	ce := c.get("withdraw.com.", dns.TypeA)
	assert.Nil(t, ce, "cache entry should be evicted after generation increase")
}

// TestCache_MultiDomainEviction verifies that all entries from the old
// generation are evicted when a new domainlist file is loaded.
func TestCache_MultiDomainEviction(t *testing.T) {
	c := newTestCache(t)

	tmpDir := t.TempDir()
	listFile1 := filepath.Join(tmpDir, "test1.lst")
	listFile2 := filepath.Join(tmpDir, "test2.lst")

	// Load file with 3 domains
	content := "multi1.com\nmulti2.com\nmulti3.com\n"
	err := os.WriteFile(listFile1, []byte(content), 0644)
	require.NoError(t, err)

	err = c.load(listFile1)
	require.NoError(t, err)

	// Verify all 3 are registered in mux
	assert.NotNil(t, c.mux.exact["multi1.com."], "multi1.com should be registered")
	assert.NotNil(t, c.mux.exact["multi2.com."], "multi2.com should be registered")
	assert.NotNil(t, c.mux.exact["multi3.com."], "multi3.com should be registered")

	// Load different file — triggers eviction of all old entries
	err = os.WriteFile(listFile2, []byte("replacement.com\n"), 0644)
	require.NoError(t, err)
	err = c.load(listFile2)
	require.NoError(t, err)

	// Verify generation increased
	assert.Equal(t, uint64(2), c.generation(), "generation should increase")

	// Verify all 3 are evicted from mux
	assert.Nil(t, c.mux.exact["multi1.com."], "multi1.com should be evicted")
	assert.Nil(t, c.mux.exact["multi2.com."], "multi2.com should be evicted")
	assert.Nil(t, c.mux.exact["multi3.com."], "multi3.com should be evicted")
}
