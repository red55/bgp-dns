package main

import (
	"fmt"
	"github.com/red55/bgp-dns-peer/internal/dns"
	"github.com/red55/bgp-dns-peer/internal/log"
	"net"
	"slices"
	"sync"
	"time"
)

type Cache struct {
	m            sync.RWMutex
	entries      []*dns.Entry
	next2Refresh *dns.Entry

	c chan string
}

func (c *Cache) String() string {
	c.m.RLock()
	defer c.m.RUnlock()
	var ret string
	for _, e := range c.entries {
		ret += fmt.Sprintf("\t%s\n", e)
	}

	return ret
}

var cache Cache

func updater(c chan struct{}) {

	for {
		cache.m.RLock()
		var ttl time.Duration
		if len(cache.entries) > 0 {
			ttl = cache.next2Refresh.Expire().Sub(time.Now())
			log.L().Infof("Updater will sleep for %s until %s for %s", ttl, cache.next2Refresh.Expire(), cache.next2Refresh.Fqdn())
		} else {
			ttl = 10 * time.Second
			log.L().Infof("Updater will sleep for %s until %s for <empty>", ttl, time.Now().Add(ttl))
		}
		cache.m.RUnlock()

		select {
		case c <- struct{}{}:
			break
		case <-time.After(ttl):
			cache.m.RLock()
			entry := cache.next2Refresh
			cache.m.RUnlock()

			if _, e := resolve(entry); e != nil {
				log.L().Panicf("unable to resolve cache entry (%e)", e)
			}
		}
	}

}

func resolve(de *dns.Entry) (*dns.Entry, error) {
	if entry, e := dns.Resolve(de); e == nil {

		cache.m.Lock()
		defer cache.m.Unlock()

		cache.next2Refresh = slices.MinFunc(cache.entries, func(a, b *dns.Entry) int {
			if a.Expire().Before(b.Expire()) {
				return -1
			} else if a.Expire().Equal(b.Expire()) {
				return 0
			} else {
				return 1
			}
		})
		/*
			slices.SortFunc(cache.entries, func(a, b *dns.Entry) int {
				if a.Expire().Before(b.Expire()) {
					return -1
				} else if a.Expire().Equal(b.Expire()) {
					return 0
				} else {
					return 1
				}
			})*/

		return entry, nil
	} else {
		if de == nil {
			log.L().Debugf("resolve failed for <nil>: %v", e)
		} else {
			log.L().Debugf("resolve failed for %s: %v", de.Fqdn(), e)
		}

		return nil, e
	}
}
func resolver(c chan string) {
	for v := range c {
		var de *dns.Entry = nil

		cache.m.RLock()
		found := slices.IndexFunc(cache.entries, func(de *dns.Entry) bool { return de.Fqdn() == v })
		if found > -1 {
			de = cache.entries[found]
		} else {
			de = dns.NewEntry(v)
			cache.entries = append(cache.entries, de)
		}
		cache.m.RUnlock()

		if de, e := resolve(de); e == nil {
			log.L().Debugf("Resolved: %v\n", de)
		} else {
			log.L().Debugf("Resolve failed for %s: %v", v, e)
		}
	}
}

func main() {
	log.Init()
	defer log.Deinit()

	resolvers := []*net.UDPAddr{
		{
			IP:   net.ParseIP("1.1.1.1"),
			Port: 53,
		},
		{
			IP:   net.ParseIP("8.8.8.8"),
			Port: 53,
		},
	}

	dns.SetResolvers(resolvers)

	cache = Cache{
		c: make(chan string, 10),
	}

	c := make(chan struct{})
	go updater(c)

	go resolver(cache.c)

	// cache.c <- "ya.ru"
	// cache.c <- "dns.ru"
	cache.c <- "tt.csi.group"
	cache.c <- "tt2.csi.group"

	time.Sleep(120 * time.Second)

	c <- struct{}{}

	log.L().Info("Waiting for termination signal")
	time.Sleep(120 * time.Second)
}
