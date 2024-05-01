package dns

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/log"
	"net"
	"os"
	"time"
)

const dot = string('.')

func queryDns(q *dns.Msg) (*dns.Msg, error) {
	_resolvers.m.RLock()
	defer _resolvers.m.RUnlock()

	if _resolvers.current == nil || _resolvers.resolvers == nil {
		return nil, fmt.Errorf("resolvers are empty, cannot resolve")
	}

	head := _resolvers.current
	for {
		srv := _resolvers.current.Value.(*resovlerT)
		log.L().Debugf("Using DNS server %v", srv)

		if r, e := dns.Exchange(q, srv.addr.String()); e == nil {
			srv.ok = true
			return r, nil
		} else {
			srv.ok = false
			log.L().Debugf("queryDns failed for %v: %v", q.Question, e)

			_resolvers.current = _resolvers.current.Next()

			if head == _resolvers.current {
				log.L().Errorf("tried all DNS servers, set current entry TTL to %s: %v")

				if errors.Is(e, os.ErrDeadlineExceeded) {
					return nil, errors.Join(fmt.Errorf("DNS operation for %v failed ", q.Question), e)
				}

				cause := e
				if unwrap, ok := cause.(interface{ Unwrap() error }); ok {
					cause = unwrap.Unwrap()
				}

				var opError *net.OpError

				switch {
				case errors.As(cause, &opError):
					log.L().Errorf("DNS operation %s failed with %s on destination %s", opError.Op, opError.Error(),
						opError.Addr)
					return nil, cause
				default:
					var rCode = -1
					if r != nil {
						rCode = r.Rcode
					}
					return nil, errors.Join(fmt.Errorf("failed to dail %s, Rcode: %x", srv.addr, rCode), e)
				}
			}
		}
	}
}

func Resolve(de *Entry) error {
	if de == nil {
		return errors.New("nil entry")
	}

	if de.fqdn == "" {
		return errors.New("missing fqdn")
	}

	if de.fqdn[len(de.fqdn)-1:] != dot {
		de.fqdn = de.fqdn + dot
	}

	q := new(dns.Msg)
	q.SetQuestion(de.fqdn, dns.TypeA)

	if r, e := queryDns(q); e != nil {
		return e
	} else { // r.Rcode == dns.RcodeSuccess
		de.r = r
		ips := make([]string, 0, len(r.Answer))
		for _, rr := range r.Answer {
			if a, ok := rr.(*dns.A); ok {
				ttl := time.Duration(a.Hdr.Ttl) * time.Second

				if ttl == 0 {
					de.SetTtl(cfg.AppCfg.Timeouts().TTL())
				} else {
					de.SetTtl(ttl)
				}

				ips = append(ips, a.A.String())
			}

			de.ips = ips
			log.L().Debugf("Resolved: %v", de)
		}
	}
	return nil
}

func resolve(de *Entry) error {
	if e := Resolve(de); e == nil {
		var found = cache.findLeastTTLCacheEntry()
		log.L().Debugf("Found least TTL cache entry: %v", found)
		cache.setNextRefreshEntry(found)
		return nil
	} else {
		log.L().Debugf("resolve failed for %s: %v", de.Fqdn(), e)
		return e
	}
}

func resolver(c chan string) {
	for v := range c {
		var de *Entry = nil

		if found := cache.findCacheEntry(v); found == nil {
			de = cache.addCacheEntry(v)
		}

		if e := resolve(de); e != nil {
			log.L().Debugf("Resolve failed for %s: %v", v, e)
		}
	}
}
