package dns

import (
	"container/ring"
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/sourcegraph/conc/iter"
	"net"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
)

var (
	ErrNoResolvers = errors.New("no resolvers available")
	ErrEmptyAnswer = errors.New("empty answer from resolver")
)

type resolver struct {
	addr *net.UDPAddr
	okay atomic.Bool
}

func (r *resolver) fail() {
	r.okay.Store(false)
}

func (r *resolver) ok() {
	r.okay.Store(true)
}
func (r *resolver) isOk() bool {
	return r.okay.Load()
}

func (r *resolver) String() string {
	return fmt.Sprintf("%s (failed: %t)", r.addr.String(), r.isOk())
}

func newResolver(a *net.UDPAddr) (new *resolver) {
	new = &resolver{
		addr: a,
	}
	new.ok()
	return new
}

type resolvers struct {
	log.Log
	m  sync.RWMutex
	rs *ring.Ring
}

func newResolvers(c []*net.UDPAddr, logger *zerolog.Logger) *resolvers {
	r := &resolvers{
		Log: log.NewLog(logger, "resolvers"),
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
		rs.rs.Value = newResolver(*a)
		rs.rs = rs.rs.Next()
	})
}

func (rs *resolvers) query(q *dns.Msg) (*dns.Msg, error) {
	rs.m.RLock()
	defer rs.m.RUnlock()

	if rs.rs == nil || rs.rs.Len() < 1 {
		return nil, ErrNoResolvers
	}

	head := rs.rs
	for {
		srv := rs.rs.Value.(*resolver)
		rs.L().Debug().Msgf("Using DNS %v for %s (%s)",
			srv.addr, q.Question[0].Name, dns.TypeToString[q.Question[0].Qtype])

		if a, e := dns.Exchange(q, srv.addr.String()); e == nil && len(a.Answer) > 0 {
			rs.L().Trace().Msgf("Got answer %s", a.Answer[0].String())
			srv.ok()
			return a, nil
		} else {
			if e == nil {
				rs.L().Warn().Msgf("%s: We got answer using: %s, rcode: %s, qtype: %s",
					q.Question[0].Name, srv.addr.String(), dns.RcodeToString[a.Rcode], dns.TypeToString[q.Question[0].Qtype])
			} else {
				rs.L().Warn().Msgf("%s: error %v, using: %s", q.Question[0].Name, e, srv.addr.String())
			}

			if a != nil && a.Rcode == dns.RcodeSuccess {
				srv.ok()
				rs.L().Warn().Msgf("%s: We got answer using: %s, but it's empty so return it as is (%s)",
					q.Question[0].Name, srv.addr.String(), dns.RcodeToString[a.Rcode])
				return a, e
			}

			srv.fail()
			if e == nil && len(a.Answer) == 0 {
				rs.L().Warn().Msgf("%s: %s empty answer, using: %s",
					q.Question[0].Name, dns.TypeToString[q.Question[0].Qtype], srv.addr.String())
			} else {
				rs.L().Error().Err(e).Msgf("queryDns failed for %v", q.Question)
			}
			rs.rs = rs.rs.Next()

			if head == rs.rs {
				rs.L().Error().Msg("All DNS Servers didn't respond")

				if q.Question[0].Qtype == dns.TypeAAAA || q.Question[0].Qtype == dns.TypeA {
					return a, errors.Join(fmt.Errorf("no %s records on DNS servers for %v",
						dns.TypeToString[q.Question[0].Qtype], q.Question), e)
				} else {
					return a, e
				}

				/*
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
				*/
			}
		}
	}
}

func (rs *resolvers) proxyQuery(w dns.ResponseWriter, rq *dns.Msg) {
	rs.L().Debug().Msgf("Proxying request %s(%d) from: %s", rq.Question[0].Name,
		rq.Question[0].Qtype, w.RemoteAddr().String())

	if r, e := rs.query(rq); e != nil && !errors.Is(e, ErrEmptyAnswer) {
		rs.L().Error().Msgf("Forwarding response to upstream responder failed %v", e)
	} else {
		if e = w.WriteMsg(r); e != nil {
			rs.L().Error().Msgf("Failed to write response to client %s, %v", w.RemoteAddr(), e)
		}
	}
}
