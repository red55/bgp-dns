package dns

import (
	"errors"
	"fmt"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Entry struct {
	fqdn  string
	ttl   time.Duration
	exire time.Time
	ips   []string
}

func NewEntry(fqdn string) *Entry {
	r := &Entry{
		fqdn: fqdn}
	r.SetTtl(cfg.AppCfg.Timeouts().TTL())
	return r
}

func (de *Entry) Fqdn() string {
	return de.fqdn
}

func (de *Entry) IPs() []string {
	return de.ips
}

func (de *Entry) Ttl() time.Duration {
	return de.ttl
}

func (de *Entry) SetTtl(ttl time.Duration) {
	de.ttl = ttl
	de.exire = time.Now().Add(ttl)
}

func (de *Entry) Expire() time.Time {
	return de.exire
}

func (de *Entry) String() string {
	return fmt.Sprintf("%s -> (TTL: %s, Expire at: %s) %v", de.fqdn, de.Ttl(), de.Expire(), de.ips)
}

type resolversT struct {
	m         sync.RWMutex
	resolvers []*net.UDPAddr
}

var _resolvers resolversT

func init() {

}

func SetResolvers(resolvers []*net.UDPAddr) {
	_resolvers.m.Lock()
	defer _resolvers.m.Unlock()

	_resolvers.resolvers = resolvers
}

const dot = string('.')

func Resolve(p *Entry) (*Entry, error) {

	if p == nil {
		return nil, errors.New("nil entry")
	}

	if p.fqdn == "" {
		return p, errors.New("missing fqdn")
	}

	if p.fqdn[len(p.fqdn)-1:] != dot {
		p.fqdn = p.fqdn + dot
	}

	m := new(dns.Msg)
	m.SetQuestion(p.fqdn, dns.TypeA)
	var e error

	_resolvers.m.RLock()
	defer _resolvers.m.RUnlock()

	for _, srv := range _resolvers.resolvers {
		var r *dns.Msg
		if r, e = dns.Exchange(m, net.JoinHostPort(srv.IP.String(), strconv.Itoa(srv.Port))); e == nil && r.Rcode == dns.RcodeSuccess {

			p.ips = make([]string, 0, len(r.Answer))

			for _, rr := range r.Answer {
				if a, ok := rr.(*dns.A); ok {
					ttl := time.Duration(a.Hdr.Ttl) * time.Second

					if ttl == 0 {
						p.SetTtl(cfg.AppCfg.Timeouts().TTL())
					} else {
						p.SetTtl(ttl)
					}

					p.ips = append(p.ips, a.A.String())
				}
			}

			return p, nil

		} else {

			if errors.Is(e, os.ErrDeadlineExceeded) {
				log.L().Errorf("DNS operation failed with %s", e.Error())
				continue
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
				continue
			default:
				var rCode int = -1
				if r != nil {
					rCode = r.Rcode
				}
				return nil, errors.Join(fmt.Errorf("failed to dail %s, Rcode: %x", srv.IP.String(), rCode), e)
			}
		}
	}

	return nil, fmt.Errorf("failed to resolve %s", p.Fqdn())
}
