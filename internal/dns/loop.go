package dns

import (
	"context"
	"errors"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"math"
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
			ce := all[c.entries.Keys(true)[0]].(*cacheEntry)
			sleepUntil = time.Unix(0, ce.expiration.Load())
		} else {
			sleepUntil = time.Now().Add(cfg.Dns.Cache.MinTtl * time.Second)
		}

		for k, v := range all {
			ce := v.(*cacheEntry)
			ck := k.(cacheKey)
			exp := time.Unix(0, ce.expiration.Load())
			if exp.Before(now) {
				q := new(dns.Msg)
				cn := dns.CanonicalName(ck.fqdn)
				q.SetQuestion(cn, ck.qtype)
				c.L().Debug().Msgf("Resolving cached %s (%s)", ck.fqdn, dns.TypeToString[ck.qtype])
				// resolve will call cache.upsert on resolved IPs
				c.resolve(nil, q, false)
				ceExp := time.Unix(0, ce.expiration.Load())
				c.L().Info().Msgf("Resolved cached %s (%s), ttl:%d, expire: %s",
					ck.fqdn, dns.TypeToString[ck.qtype], ce.ttl.Load(), ceExp.Format(time.RFC3339))
			}

			ceExp := time.Unix(0, ce.expiration.Load())
			if sleepUntil.After(ceExp) {
				c.L().Trace().Msgf("Sleep until %s is less than %s (%s)", sleepUntil.Format(time.RFC3339),
					ceExp.Format(time.RFC3339), ck.fqdn)
				sleepUntil = ceExp
			}

		}
		c.L().Trace().Msgf("Calculated sleep until and now difference is %d sec",
			time.Duration(math.Abs(float64(sleepUntil.Sub(now))))/time.Second)
		if time.Duration(math.Abs(float64(sleepUntil.Sub(now)))) < cfg.Dns.Cache.MinTtl*time.Second {
			sleepUntil = now.Add(cfg.Dns.Cache.MinTtl * time.Second)
		}

		c.L().Info().Msgf("DNS Refresher will sleep until %s for %d seconds", sleepUntil.Format(time.RFC3339),
			sleepUntil.Sub(now)/time.Second)
		timeout, cancelTimeout := context.WithDeadline(ctx, sleepUntil)

		select {
		case o := <-c.ChanOp():
			cancelTimeout()
			c.HandleOp(o)
			continue
		case <-timeout.Done():
			cancelTimeout()
			continue
		case <-ctx.Done():
			cancelTimeout()
			if !errors.Is(ctx.Err(), context.Canceled) {
				log.L().Error().Err(ctx.Err())
			}
			break L
		}
	}
}
