package dns

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/beevik/prefixtree/v2"
	"github.com/bluele/gcache"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/bgp"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/red55/bgp-dns/internal/utils"
	"github.com/rs/zerolog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type entries *prefixtree.Tree[cacheEntry]

type cache struct {
	loop.Loop
	log.Log
	m sync.RWMutex
	wg sync.WaitGroup
	pref entries
	entries gcache.Cache
	cancel context.CancelFunc
	rs *resolvers
	minTtl time.Duration
	gen 	atomic.Uint64
}

func newCache(max int, minTtl time.Duration, rs *resolvers, l *zerolog.Logger) (r *cache) {
	r = &cache{
		Loop: loop.NewLoop(1),
		Log: log.NewLog(l, "dns"),
		pref: prefixtree.New[cacheEntry](),
		cancel: nil,
		rs:     rs,
		minTtl: minTtl,
		gen:    atomic.Uint64{},
	}
	r.entries = gcache.New(max).LFU().EvictedFunc(r.onEntryEvicted).Build()

	return
}

func (c *cache) onEntryEvicted(k interface{}, v interface{}) {
	c.L().Debug().Msgf("Evicting %s", k.(string))
	if e := bgp.Withdraw(v.(*cacheEntry).Ip4s()); e != nil {
		c.L().Error().Err(e).Msgf("Failed to withdraw IPs for %s", k.(string))
	}
}

func (c *cache) generation() uint64 {
	return (&c.gen).Load()
}

func (c *cache) increaseGeneration () uint64 {
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

func (c *cache) upsert(fqdn string, answer *dns.Msg) error {

	var ce *cacheEntry
	var cn = dns.CanonicalName(fqdn)
	if t, e := c.entries.Get(cn); t == nil && !errors.Is(e, gcache.KeyNotFoundError) {
		return e
	} else if t != nil {
		ce = t.(*cacheEntry)
	}
	var gen = c.generation()
	var prevIps [] string
	if ce == nil {
		ce = newCacheEntry(answer, c.minTtl, gen)
	} else {
		prevIps = ce.Ip4s()
		ce.answer = answer
		ce.gen.Store(gen)
		ce.updateTtl(c.minTtl)
	}

	var ips = ce.Ip4s()
	var gone = utils.Difference(prevIps, ips)
	var arrived = utils.Difference(ips, prevIps)

	_ = bgp.Advance(arrived)
	_ = bgp.Withdraw(gone)

	if e := c.entries.Set(fqdn, ce); e != nil {
		c.L().Error().Err(e)
		return e
	}
	return c.Operation(func () error {
		c.L().Debug().Msgf("Signaling cache changed for %s", cn)
		return nil
	}, false)

}

func (c* cache) findKeysByGeneration(gen uint64) []string {
	// GetALL returns a map with a copy of cache contents
	all := c.entries.GetALL(true)
	r := make([]string, 0, len(all) / 2)
	for k,v := range all {
		ce := v.(*cacheEntry)
		ceGen := (&ce.gen).Load()
		if ceGen <= gen {
			r = append(r, k.(string))
		}
	}

	return r
}

func (c *cache) has(k string) bool{
	return c.entries.Has(k)
}

func (c* cache) register(fqdn string) error {
	if len (fqdn) < 2 {
		return fmt.Errorf("'%s'. %w", fqdn, EInvalidFQDN)
	}
	cn := dns.CanonicalName(fqdn)
	dns.HandleFunc(dns.CanonicalName(fqdn), c.resolve)

	q := new(dns.Msg)
	q.SetQuestion(cn, dns.TypeA)
	// resolve will call cache.upsert on resolved IPs
	c.resolve(nil, q)

	return nil
}

func (c*cache) unregister(fqdn string) error {
	if len (fqdn) < 2 {
		return fmt.Errorf("'%s'. %w", fqdn, EInvalidFQDN)
	}
	cn := dns.CanonicalName(fqdn)
	c.L().Debug().Msgf("Unregistering %s", cn)
	dns.HandleRemove(cn)

	var kr [] string
	for _, k := range c.entries.Keys(true) {
		s := k.(string)
		if strings.HasSuffix(s, cn) {
			kr = append(kr, s)
		}
	}

	for _, k := range kr {
		c.L().Trace().Msgf("Removing cache entry %s", k)
		_ = c.entries.Remove(k);
	}

	return nil
}

func (c* cache) load(fn string) error {
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

	scanner := bufio.NewScanner(f)
	oldGeneration := c.generation()
	_ = c.increaseGeneration()

	for scanner.Scan() {
		fqdn := strings.TrimSpace(scanner.Text())
		if len(fqdn) == 0 {
			continue
		}
		if fqdn[0] == '#' || fqdn[0] == ';'{
			continue
		}
		if e = c.register(fqdn); e != nil {
			return e
		}

	}

	return c.evictByGeneration(oldGeneration);
}

func (c *cache) evictByGeneration(gen uint64) error {
	c.L().Debug().Msgf("Evicting generation %d...", gen)
	defer c.L().Debug().Msgf("Evicting generation %d done.", gen)
	keys := c.findKeysByGeneration(gen)

	for _, k := range keys {
		if e := c.unregister(k); e != nil{
			c.L().Error().Err(e).Msgf("Failed to unregister by generation")
		}
	}

	return nil
}