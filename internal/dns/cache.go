package dns

import (
	"context"
	"errors"
	"github.com/beevik/prefixtree/v2"
	"github.com/bluele/gcache"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/bgp"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/utils"

	"sync"
	"sync/atomic"
	"time"

)

type entries *prefixtree.Tree[cacheEntry]

type cache struct {
	m sync.RWMutex
	wg sync.WaitGroup
	pref entries
	entries gcache.Cache
	cancel context.CancelFunc
	rs *resolvers
	minTtl time.Duration
	gen 	atomic.Uint64
}

func newCache(max int, minTtl time.Duration, rs *resolvers) *cache{
	return &cache{
		pref: prefixtree.New[cacheEntry](),
		entries: gcache.New(max).LFU().EvictedFunc(func(k interface{}, v interface{}) {
			log.L().Debug().Msgf("Evicting %s", k.(string))
			if e := bgp.Withdraw(v.(*cacheEntry).Ip4s()); e != nil {
				log.L().Error().Err(e).Msgf("Failed to withdraw IPs for %s", k.(string))
			}
		}).Build(),
		cancel: nil,
		rs:     rs,
		minTtl: minTtl,
		gen:    atomic.Uint64{},
	}
}

func (c *cache) generation() uint64 {
	return (&c.gen).Load()
}

func (c *cache) increaseGeneration () uint64 {
	return (&c.gen).Add(1)
}


func (c *cache) Serve(ctx context.Context) error {
	c.m.Lock()
	defer c.m.Unlock()
	ctx, cancel := context.WithCancel(ctx)

	if nil != c.cancel {
		c.cancel()
	}
	c.cancel = cancel

	go c.loop(ctx)

	return nil
}

func (c *cache) Shutdown(fn string) error {
	c.cancel()
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

	var prevIps [] string
	if ce == nil {
		ce = newCacheEntry(answer, c.minTtl, c.generation())
	} else {
		prevIps = ce.Ip4s()
		ce.answer = answer
		ce.updateTtl(c.minTtl)
	}

	var ips = ce.Ip4s()
	var gone = utils.Difference(prevIps, ips)
	var arrived = utils.Difference(ips, prevIps)

	_ = bgp.Advance(arrived)
	_ = bgp.Withdraw(gone)

	return c.entries.Set(fqdn, ce)

}

func (c* cache) findKeysByGeneration(gen uint64) []string {
	// GetALL returns a map with a copy of cache contents
	all := c.entries.GetALL(true)
	r := make([]string, 0, len(all) / 2)
	for k,v := range all {
		ce := v.(*cacheEntry)
		ceGen := (&ce.gen).Load()
		if ceGen == gen {
			r = append(r, k.(string))
		}
	}

	return r
}

func (c *cache) has(k string) bool{
	return c.entries.Has(k)
}


