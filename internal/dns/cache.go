package dns

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bluele/gcache"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/bgp"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/red55/bgp-dns/internal/utils"
	"github.com/rs/zerolog"
)

type cache struct {
	loop.Loop
	log.Log
	m       sync.RWMutex
	wg      sync.WaitGroup
	entries gcache.Cache
	cancel  context.CancelFunc
	rs      *resolvers
	minTtl  time.Duration
	gen     atomic.Uint64
	mux     *regexServeMux
}

func newCache(max int, minTtl time.Duration, rs *resolvers, l *zerolog.Logger) (r *cache) {
	r = &cache{
		Loop:   loop.NewLoop(1, l),
		Log:    log.NewLog(l, "dns"),
		cancel: nil,
		rs:     rs,
		minTtl: minTtl,
		gen:    atomic.Uint64{},
	}
	r.entries = gcache.New(max).LFU().EvictedFunc(r.onEntryEvicted).Build()
	r.mux = newRegexServeMux()

	return
}

func (c *cache) onEntryEvicted(k interface{}, v interface{}) {
	ck := k.(cacheKey)
	c.L().Debug().Msgf("Evicting %s", ck)
	if e := bgp.Withdraw(v.(*cacheEntry).Ip4s()); e != nil {
		c.L().Error().Err(e).Msgf("Failed to withdraw IPs for %s", ck)
	}
}

func (c *cache) generation() uint64 {
	return (&c.gen).Load()
}

func (c *cache) increaseGeneration() uint64 {
	return (&c.gen).Add(1)
}

func (c *cache) serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	if nil != c.cancel {
		c.cancel()
	}
	c.cancel = cancel

	go c.loop(ctx)

	return nil
}

func (c *cache) shutdown() error {
	if nil != c.cancel {
		c.cancel()
	}
	c.wg.Wait()

	return nil
}

func (c *cache) upsert(fqdn string, qtype uint16, answer *dns.Msg) error {
	c.L().Trace().Msgf("-> upsert(%s)", fqdn)
	defer c.L().Trace().Msgf("<- upsert(%s)", fqdn)

	var cn = dns.CanonicalName(fqdn)
	var ce = c.get(cn, qtype)
	var gen = c.generation()
	var prevIps []string
	if ce == nil {
		ce = newCacheEntry(answer, c.minTtl, gen)
	} else {
		prevIps = ce.Ip4s()
		ce.answer = answer
		ce.gen.Store(gen)
		ce.updateTtl(c.minTtl)
		ce.ResetFailures()
	}

	var ips = ce.Ip4s()
	var gone = utils.Difference(prevIps, ips)
	var arrived = utils.Difference(ips, prevIps)

	_ = bgp.Advance(arrived)
	_ = bgp.Withdraw(gone)

	if e := c.entries.Set(newCacheKey(cn, qtype), ce); e != nil {
		c.L().Error().Err(e)
		return e
	}
	return nil

}

func (c *cache) findKeysByGeneration(gen uint64) []string {
	// GetALL returns a map with a copy of cache contents
	all := c.entries.GetALL(true)
	r := make([]string, 0, len(all)/2)
	for k, v := range all {
		ce := v.(*cacheEntry)
		ceGen := (&ce.gen).Load()
		if ceGen <= gen {
			r = append(r, k.(string))
		}
	}

	return r
}

var requestTypes = []uint16{dns.TypeA, dns.TypeHTTPS}

func (c *cache) register(fqdn string) error {
	return c.registerOn(fqdn, c.mux, true)
}

func (c *cache) registerOn(fqdn string, targetMux *regexServeMux, doLookup bool) error {
	if len(fqdn) < 2 {
		return fmt.Errorf("'%s'. %w", fqdn, EInvalidFQDN)
	}
	cn := dns.CanonicalName(fqdn)
	targetMux.HandleFunc(cn, func(rw dns.ResponseWriter, m *dns.Msg) {
		c.resolve(rw, m, true)
	})
	if doLookup {
		q := new(dns.Msg)
		for _, t := range requestTypes {
			q.SetQuestion(cn, t)
			c.resolve(nil, q, false)
		}
	}
	return nil
}

func (c *cache) SetMux(m *regexServeMux) {
	c.mux = m
}

func (c *cache) registerRegex(pattern string) error {
	return c.mux.HandleRegex(pattern, func(w dns.ResponseWriter, r *dns.Msg) {
		c.resolve(w, r, true)
	})
}

func (c *cache) registerRegexOn(fqdn string, targetMux *regexServeMux) error {
	pattern := strings.TrimPrefix(fqdn, "regex:")
	return targetMux.HandleRegex(pattern, func(w dns.ResponseWriter, r *dns.Msg) {
		c.resolve(w, r, true)
	})
}

func (c *cache) get(fqdn string, qtype uint16) *cacheEntry {
	ce, _ := c.entries.Get(newCacheKey(fqdn, qtype))
	if ce == nil {
		return nil
	}
	return ce.(*cacheEntry)
}

func (c *cache) fail(fqdn string, qtype uint16) {
	ce := c.get(fqdn, qtype)

	if ce != nil {
		ce.IncFailures()
	}
}

func (c *cache) unregister(fqdn string) error {
	if len(fqdn) < 2 {
		return fmt.Errorf("'%s'. %w", fqdn, EInvalidFQDN)
	}
	cn := dns.CanonicalName(fqdn)
	c.L().Debug().Msgf("Unregistering %s", cn)
	c.mux.HandleRemove(cn)

	var kr []cacheKey
	for _, k := range c.entries.Keys(true) {
		s := k.(cacheKey)
		if strings.HasPrefix(s.fqdn, cn) {
			kr = append(kr, s)
		}
	}

	for _, k := range kr {
		c.L().Trace().Msgf("Removing cache entry %s", k)
		_ = c.entries.Remove(k)
	}

	return nil
}

func (c *cache) load(fn string) error {
	f, e := os.Open(fn)
	if e != nil {
		return e
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.L().Warn().Msgf("Failed to close file: %v", err)
		}
	}(f)

	// SAFE RELOAD: increment generation BEFORE loading entries so new
	// entries are tagged with the current generation and won't be
	// evicted by the old-generation sweep.
	oldGen := c.generation()
	_ = c.increaseGeneration()

	// Build on a temporary mux first; swap only on success.
	tempMux := newRegexServeMux()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' || line[0] == ';' {
			continue
		}

		if strings.HasPrefix(line, "regex:") {
			if e = c.registerRegexOn(line, tempMux); e != nil {
				c.L().Warn().Msgf("Skipping invalid regex pattern %q: %v", strings.TrimPrefix(line, "regex:"), e)
				continue
			}
		} else {
			if e = c.registerOn(line, tempMux, false); e != nil {
				return e
			}
		}
	}

	// All registrations succeeded — atomically swap mux
	c.mux = tempMux

	return c.evictByGeneration(oldGen)
}

func (c *cache) evictByGeneration(gen uint64) error {
	c.L().Debug().Msgf("Evicting generation %d...", gen)
	defer c.L().Debug().Msgf("Evicting generation %d done.", gen)
	keys := c.findKeysByGeneration(gen)

	for _, k := range keys {
		if e := c.unregister(k); e != nil {
			c.L().Error().Err(e).Msgf("Failed to unregister by generation")
		}
	}

	return nil
}

func (c *cache) notifyChanged(cn string) {
	_ = c.Operation(func() error {
		c.L().Debug().Msgf("Signaling cache changed for %s", cn)
		return nil
	}, false)
}

func (c *cache) dump(callback func(qtype uint16, fqdn string, fails uint64, ips []string, ttl time.Duration, expiration time.Time, gen uint64) error) error {
	if callback == nil {
		return errors.New("callback function is nil")
	}

	all := c.entries.GetALL(true)
	for k, v := range all {
		ce := v.(*cacheEntry)
		key := k.(cacheKey)
		if e := callback(key.qtype, key.fqdn, ce.Failures(), ce.Ip4s(),
			time.Duration(ce.ttl.Load()), time.Unix(0, ce.expiration.Load()), ce.gen.Load()); e != nil {
			return e
		}
	}
	return nil
}

func (c *cache) clear() uint64 {
	c.L().Debug().Msg("Clearing all cache entries")
	defer c.L().Debug().Msg("Clearing all cache entries done")

	all := c.entries.GetALL(true)
	for k := range all {
		key, ok := k.(cacheKey)
		if !ok {
			c.L().Error().Msgf("Failed to clear cache entry: unexpected key type %T", k)
			continue
		}

		if e := c.unregister(key.fqdn); e != nil {
			c.L().Error().Err(e).Msg("Failed to unregister cache entry during clear")
		}
	}

	return uint64(len(all))
}
