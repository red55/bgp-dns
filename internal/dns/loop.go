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
	_wg.Add(1)
	defer _wg.Done()

	cfg := ctx.Value("cfg").(*config.AppCfg)
	L:
	for {
		all := _cache.entries.GetALL(true)
		var sleepUntil time.Time
		if len(all) > 0 {
			sleepUntil = all[_cache.entries.Keys(true)[0]].(*cacheEntry).expiration
		} else {
			sleepUntil = time.Now().Add(cfg.Dns.Cache.MinTtl * time.Second)
		}

		now := time.Now()

		for k,v := range all {
			ce := v.(*cacheEntry)
			if sleepUntil.After(ce.expiration) {
				sleepUntil = ce.expiration
			}
			log.L().Trace().Msgf("%s, ttl:%d, expire: %s", k.(string), ce.ttl, ce.expiration.String())
			if ce.expiration.Before(now) {
				q := new(dns.Msg)
				cn := dns.CanonicalName(k.(string))
				q.SetQuestion(cn, dns.TypeA)
				log.L().Debug().Msgf("Resolving cached %s", k.(string))
				// resolve will call cache.upsert on resolved IPs
				resolve(nil, q)
				log.L().Trace().Msgf("%s, ttl:%d, expire: %s", k.(string), ce.ttl,ce.expiration.String())
			}

		}
		if sleepUntil.Sub(now) < cfg.Dns.Cache.MinTtl * time.Second {
			sleepUntil = now.Add(cfg.Dns.Cache.MinTtl * time.Second)
		}
		log.L().Info().Msgf("DSN Refresher will sleep until %s for %d seconds", sleepUntil.Format(time.RFC3339),
			sleepUntil.Sub(now) / time.Second)
		timeout, cancelTimeout := context.WithDeadline(ctx, sleepUntil)

		select {
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