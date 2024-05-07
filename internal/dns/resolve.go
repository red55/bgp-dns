package dns

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/log"
	"net"
	"os"
)

const dot = string('.')

type errNXName struct{}

func (e *errNXName) Error() string {
	return "NX Name Error"
}

func resolveOnConfigChange() {

	_resolvers.setResolvers(cfg.AppCfg.Resolvers())
	_defaultResolvers.setResolvers(cfg.AppCfg.DefaultResolvers())

	CacheClear()
	for _, n := range cfg.AppCfg.Names() {
		CacheEnqueue(n)
	}
}

func (r *resolversT) queryDns(q *dns.Msg) (*dns.Msg, error) {
	r.m.RLock()
	defer r.m.RUnlock()

	if r.resolvers == nil || r.resolvers.Len() < 1 {
		return nil, fmt.Errorf("resolvers are empty, cannot resolve")
	}

	head := r.resolvers
	for {
		srv := r.resolvers.Value.(*resovlerT)
		log.L().Debugf("Using DNS server %v", srv)

		if a, e := dns.Exchange(q, srv.addr.String()); e == nil {
			srv.ok = true
			return a, nil
		} else {
			srv.ok = false
			log.L().Debugf("queryDns failed for %v: %v", q.Question, e)

			r.resolvers = r.resolvers.Next()

			if head == r.resolvers {
				log.L().Errorf("All DNS Servers didn't answer")

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
					if a != nil {
						rCode = a.Rcode
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

	if r, e := _resolvers.queryDns(q); e != nil {
		return e
	} else {
		if r.Rcode == dns.RcodeSuccess {
			de.r = r
			de.ips = make([]string, 0, len(r.Answer))
			for _, rr := range r.Answer {
				if a, ok := rr.(*dns.A); ok {
					ttl := a.Hdr.Ttl

					if ttl == 0 {
						de.SetTtl(cfg.AppCfg.Timeouts().DefaultTTL())
					} else {
						de.SetTtl(ttl)
					}

					de.ips = append(de.ips, a.A.String())
				}
			}
			log.L().Debugf("Resolved: %v", de)
		} else {
			return fmt.Errorf("DNS server answered bad RCode %d, %w", r.Rcode, &errNXName{})
		}

	}
	return nil
}

func resolve(entry *Entry) error {
	if entry == nil || len(entry.Fqdn()) < 1 {
		// obliviously we need to return special error class and ignore it in the caller, but now just
		// behave as it normal.
		return nil
	}
	previps := make([]string, len(entry.ips))
	copy(previps, entry.ips)
	e := Resolve(entry)

	cache.updateNextRefresh(true)

	if e == nil {
		fireCallbacks(entry.Fqdn(), previps, entry.ips)
		return nil
	}

	if errors.Is(e, &errNXName{}) {
		log.L().Debugf("Remove from cache %s as it is NXDOMAIN",
			entry.Fqdn())
		e = cache.remove(entry.Fqdn())
	}

	return e
}
