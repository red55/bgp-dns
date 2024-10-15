package dns

import (
    "container/ring"
    "errors"
    "fmt"
    "github.com/miekg/dns"
    "github.com/red55/bgp-dns/internal/log"
    "github.com/sourcegraph/conc/iter"
    "net"
    "os"
    "sync"
)

type resolver struct {
	addr *net.UDPAddr
	ok   bool
}

type resolvers struct {
	m  sync.RWMutex
	rs *ring.Ring
}

func newResolvers(c []*net.UDPAddr) *resolvers {
	r := &resolvers{
	}
	r.setResolvers(c)

	return r
}
func (rs *resolvers) setResolvers(c []*net.UDPAddr) {
	rs.m.Lock()
	defer rs.m.Unlock()

	l := len(c)
	rs.rs = ring.New(l)

	iter.ForEach(c, func(a **net.UDPAddr) {
		rs.rs.Value = &resolver{
            addr: *a,
            ok:   true,
        }
		rs.rs = rs.rs.Next()
	})
}

func (rs *resolvers) query(q *dns.Msg) (*dns.Msg, error) {
	rs.m.RLock()
	defer rs.m.RUnlock()

	if rs.rs == nil || rs.rs.Len() < 1 {
		return nil, fmt.Errorf("resolvers are empty, cannot resolve")
	}

	head := rs.rs
	for {
		srv := rs.rs.Value.(*resolver)
		log.L().Debug().Msgf("Using DNS server %v", srv.addr)

		if a, e := dns.Exchange(q, srv.addr.String()); e == nil {
			srv.ok = true
			return a, nil
		} else {
			srv.ok = false
			log.L().Error().Err(e).Msgf("queryDns failed for %v", q.Question)

			rs.rs = rs.rs.Next()

			if head == rs.rs {
				log.L().Error().Msg("All DNS Servers doesn't respond")

				if errors.Is(e, os.ErrDeadlineExceeded) {
					return nil, errors.Join(fmt.Errorf("DNS op for %v failed ", q.Question), e)
				}

				cause := e
				if unwrap, ok := cause.(interface{ Unwrap() error }); ok {
					cause = unwrap.Unwrap()
				}

				var opError *net.OpError

				switch {
				case errors.As(cause, &opError):
					log.L().Error().Msgf("DNS op %s failed with %s on destination %s", opError.Op, opError.Error(),
						opError.Addr)
					//return nil, cause
				default:
					var rCode = -1
					if a != nil {
						rCode = a.Rcode
					}
					log.L().Error().Err(errors.Join(fmt.Errorf("failed to dail %s, Rcode: %x", srv.addr, rCode), e))
					//return nil, errors.Join(fmt.Errorf("failed to dail %s, Rcode: %x", srv.addr, rCode), e)
				}
			}
		}
	}
}

func (rs *resolvers) ResolveA(fqdn string) {

}
func proxyQuery(w dns.ResponseWriter, rq *dns.Msg) {
	log.L().Debug().Msgf("Proxying request %v from: %s", rq, w.RemoteAddr().String())

	if r, e := _resolvers.query(rq); e != nil {
		log.L().Error().Msgf("Forwarding response to upstream responder failed %v", e)
	} else {
		if e = w.WriteMsg(r); e != nil {
			log.L().Error().Msgf("Failed to write response to client %s, %v", w.RemoteAddr(), e)
		}
	}
}
