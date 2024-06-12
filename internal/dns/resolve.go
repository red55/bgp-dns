package dns

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/fswatcher"
	"github.com/red55/bgp-dns/internal/log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
)

const (
	dot = string('.')
)

type errNXName struct{}

func (e *errNXName) Error() string {
	return "NX Name Error"
}

func onDirChange(ev fsnotify.Event) {
	var e error
	path, _ := filepath.Abs(ev.Name)
	isLst, _ := filepath.Match(filepath.Join(cfg.AppCfg.DomainListsFolder(), "*.lst"), path)

	if (ev.Has(fsnotify.Write) || ev.Has(fsnotify.Create)) && isLst {
		path, e = filepath.Abs(path)

		if e != nil {
			log.L().Fatalf("Unable to get absolute path of %s", e)
		}
		Load([]string{path})
	}
}

func resolveOnConfigChange() {

	_resolvers.setResolvers(cfg.AppCfg.Resolvers())
	_defaultResolvers.setResolvers(cfg.AppCfg.DefaultResolvers())

	// fswatcher.StopWatcher()
	CacheClear()

	if e := fswatcher.AddWatcher(onDirChange); e != nil {
		log.L().Fatal(e)
	}

	// Simulate config folder change
	config := cfg.AppCfg.DomainListsFolder()
	if dir, e := os.ReadDir(config); e != nil {
		log.L().Fatal(e)
	} else {
		for _, d := range dir {
			if !d.IsDir() && filepath.Ext(d.Name()) == ".lst" {
				fswatcher.TriggerCreate(filepath.Join(config, d.Name()))
			}
		}
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
		srv := r.resolvers.Value.(*resolverT)
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
					return nil, errors.Join(fmt.Errorf("DNS op for %v failed ", q.Question), e)
				}

				cause := e
				if unwrap, ok := cause.(interface{ Unwrap() error }); ok {
					cause = unwrap.Unwrap()
				}

				var opError *net.OpError

				switch {
				case errors.As(cause, &opError):
					log.L().Errorf("DNS op %s failed with %s on destination %s", opError.Op, opError.Error(),
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

func Resolve(entry /*in, out*/ *Entry) ([]string, error) {
	if entry == nil {
		return nil, errors.New("nil entry")
	}

	if entry.fqdn == "" {
		return nil, errors.New("missing fqdn")
	}

	if entry.fqdn[len(entry.fqdn)-1:] != dot {
		entry.fqdn = entry.fqdn + dot
	}
	prevIps := make([]string, len(entry.ips))
	copy(prevIps, entry.ips)

	q := new(dns.Msg)
	q.SetQuestion(entry.fqdn, dns.TypeA)

	if r, e := _resolvers.queryDns(q); e != nil {
		return prevIps, e
	} else {
		if r.Rcode == dns.RcodeSuccess && r.Answer != nil {
			entry.r = r
			entry.ips = make([]string, 0, len(r.Answer))
			ttl := uint32(0)
			for _, rr := range r.Answer {
				if a, ok := rr.(*dns.A); ok {
					if ttl < a.Hdr.Ttl {
						ttl = a.Hdr.Ttl
					}
					entry.ips = append(entry.ips, a.A.String())
				}
			}
			if ttl < cfg.AppCfg.Timeouts().TtlForZero() {
				jitter := uint32(rand.Int31n(int32(cfg.AppCfg.Timeouts().Ttl4ZeroJitter)))
				log.L().Debugf("Entry %s has ttl %d, so adjust it to default %d with jitter %d",
					entry.Fqdn(),
					ttl,
					cfg.AppCfg.Timeouts().TtlForZero(),
					jitter,
				)
				ttl = cfg.AppCfg.Timeouts().TtlForZero() - jitter
			}
			entry.SetTtl(ttl)

			log.L().Debugf("Resolved: name %s will expire at %s (%d), previous ips: %v",
				entry.Fqdn(),
				entry.Expire().String(),
				entry.Ttl(),
				prevIps)

		} else {
			if r.Rcode != dns.RcodeSuccess {
				return prevIps, fmt.Errorf("DNS server answered bad RCode %d, %w ", r.Rcode, &errNXName{})
			}
			log.L().Debugf("DNS query didn't return any RRs so set default timeout to retry later")
			entry.SetTtl(cfg.AppCfg.Timeouts().DefaultTTL())
		}

	}
	return prevIps, nil
}

func resolve(entry *Entry) error {
	if entry == nil || len(entry.Fqdn()) < 1 {
		// obliviously we need to return special error class and ignore it in the caller, but now just
		// behave as it normal.
		return nil
	}
	previps, e := Resolve(entry)

	_cache.updateNextRefresh(true)

	if e == nil {
		fireCallbacks(entry.Fqdn(), previps, entry.ips)
		return nil
	}

	if errors.Is(e, &errNXName{}) {
		log.L().Debugf("Remove from cache %s as it is NXDOMAIN",
			entry.Fqdn())
		e = _cache.remove(entry.Fqdn())
	}

	return e
}
