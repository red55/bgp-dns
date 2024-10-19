package dns

import (
	"context"
	"errors"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"time"
)


func (c *cache) loop(ctx context.Context) {
	c.wg.Add(1)
	defer c.wg.Done()

	cfg := ctx.Value("cfg").(*config.AppCfg)
	L:
	for {
		var sleepUntil time.Time
		now := time.Now()

		all := c.entries.GetALL(true)

		if len(all) > 0 {
			sleepUntil = all[c.entries.Keys(true)[0]].(*cacheEntry).expiration
		} else {
			sleepUntil = time.Now().Add(cfg.Dns.Cache.MinTtl * time.Second)
		}

		for k,v := range all {
			ce := v.(*cacheEntry)

			c.L().Trace().Msgf("%s, ttl:%d, expire: %s", k.(string), ce.ttl, ce.expiration.Format(time.RFC3339))
			if ce.expiration.Before(now) {
				q := new(dns.Msg)
				cn := dns.CanonicalName(k.(string))
				q.SetQuestion(cn, dns.TypeA)
				c.L().Debug().Msgf("Resolving cached %s", k.(string))
				// resolve will call cache.upsert on resolved IPs
				c.resolve(nil, q, false)
				c.L().Trace().Msgf("New %s, ttl:%d, expire: %s", k.(string), ce.ttl,ce.expiration.Format(time.RFC3339))
			}

			if sleepUntil.After(ce.expiration) {
				sleepUntil = ce.expiration
			}

		}
		if sleepUntil.Sub(now) < cfg.Dns.Cache.MinTtl * time.Second {
			sleepUntil = now.Add(cfg.Dns.Cache.MinTtl * time.Second)
		}

		c.L().Info().Msgf("DSN Refresher will sleep until %s for %d seconds", sleepUntil.Format(time.RFC3339),
			sleepUntil.Sub(now) / time.Second)
		timeout, cancelTimeout := context.WithDeadline(ctx, sleepUntil)

		select {
		case o := <- c.ChanOp():
			cancelTimeout()
			c.HandleOp(o)
			continue
		case <- timeout.Done():
			cancelTimeout()
			continue
		case <- ctx.Done():
			cancelTimeout()
			if !errors.Is(ctx.Err(), context.Canceled) {
				log.L().Error().Err(ctx.Err())
			}
			break L
		}
	}
}