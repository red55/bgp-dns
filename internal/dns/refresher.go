package dns

import (
	"github.com/red55/bgp-dns-peer/internal/log"
	"time"
)

func refresher(c chan struct{}) {
	for {
		cache.m.RLock()
		var ttl time.Duration
		if cache.next2Refresh == nil {
			ttl = 10 * time.Second
			log.L().Infof("Refresher will sleep for %s until %s for <empty>", ttl, time.Now().Add(ttl))
		} else {
			ttl = cache.next2Refresh.Expire().Sub(time.Now())
			if ttl < 0 {
				log.L().Errorf("Looks like %v already expired. Missed time %s.", cache.next2Refresh, ttl)
				ttl = 0
			}
			log.L().Infof("Refresher will sleep for %s until %s for %s", ttl, cache.next2Refresh.Expire(),
				cache.next2Refresh.Fqdn())
		}
		cache.m.RUnlock()

		select {
		case <-c:
			return
		case <-time.After(ttl):
			cache.m.RLock()
			entry := cache.next2Refresh
			cache.m.RUnlock()
			if entry == nil || len(entry.Fqdn()) < 1 {
				continue
			}
			if e := resolve(entry); cache.next2Refresh != nil && e != nil {
				log.L().Panicf("unable to resolve cache entry (%e)", e)
			}
		}
	}
}
