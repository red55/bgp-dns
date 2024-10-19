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
	log.Log
	m  sync.RWMutex
	rs *ring.Ring
}

func newResolvers(c []*net.UDPAddr) *resolvers {
	r := &resolvers{
		Log: log.NewLog(log.L(), "resolvers"),
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
		rs.L().Debug().Msgf("Using DNS %v for %s", srv.addr, q.Question[0].Name)

		if a, e := dns.Exchange(q, srv.addr.String()); e == nil {
			rs.L().Trace().Msgf("Got answer %d", len(a.Answer))
			srv.ok = true
			return a, nil
		} else {
			srv.ok = false
			rs.L().Error().Err(e).Msgf("queryDns failed for %v", q.Question)

			rs.rs = rs.rs.Next()

			if head == rs.rs {
				rs.L().Error().Msg("All DNS Servers doesn't respond")

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
					rs.L().Error().Msgf("DNS op %s failed with %s on destination %s", opError.Op, opError.Error(),
						opError.Addr)
					//return nil, cause
				default:
					var rCode = -1
					if a != nil {
						rCode = a.Rcode
					}
					rs.L().Error().Err(errors.Join(fmt.Errorf("failed to dail %s, Rcode: %x", srv.addr, rCode), e))
					//return nil, errors.Join(fmt.Errorf("failed to dail %s, Rcode: %x", srv.addr, rCode), e)
				}
			}
		}
	}
}

func (rs *resolvers) ResolveA(fqdn string) {

}
func (rs *resolvers) proxyQuery(w dns.ResponseWriter, rq *dns.Msg) {
	rs.L().Debug().Msgf("Proxying request %s(%d) from: %s", rq.Question[0].Name, rq.Question[0].Qtype, w.RemoteAddr().String())

	if r, e := rs.query(rq); e != nil {
		rs.L().Error().Msgf("Forwarding response to upstream responder failed %v", e)
	} else {
		if e = w.WriteMsg(r); e != nil {
			rs.L().Error().Msgf("Failed to write response to client %s, %v", w.RemoteAddr(), e)
		}
	}
}
