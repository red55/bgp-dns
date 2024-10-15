package bgp

import (
	"github.com/cornelk/hashmap"
	"github.com/red55/bgp-dns/internal/log"
	"sync/atomic"
)

var _ipRefCounter = hashmap.New[string, *atomic.Uint64]()

func Advance(ips []string) error {
	for _, ip := range ips {
		counter := new(atomic.Uint64)
		refs, _ := _ipRefCounter.GetOrInsert(ip, counter)
		c := refs.Add(1)
		if  c == 1 {
			log.L().Debug().Msgf("Advance IPs: %v", ip)
		} else {
			log.L().Debug().Msgf("No need to change BGP, %v(%d)", ip, c)
		}
	}

	return nil
}

func Withdraw(ips []string) error {
	for _, ip := range ips {
		if refs, exists := _ipRefCounter.Get(ip); exists {
			c := refs.Add(^uint64(0))
			if c < 1 {
				log.L().Debug().Msgf("Withdraw IPs: %v", ip)
				_ipRefCounter.Del(ip)
			} else {
				log.L().Debug().Msgf("No need to change BGP, %v(%d)", ip, c)
			}
		}
	}
	return nil
}