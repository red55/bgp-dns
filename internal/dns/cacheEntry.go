package dns
import 	(
	"errors"
	"github.com/miekg/dns"
	"sync/atomic"
	"time"
)

type cacheEntry struct {
	gen atomic.Uint64
	ttl time.Duration
	answer 	*dns.Msg
}
var (
	ENotAddressAnswer = errors.New("not an A/AAAA answer")
)

func minTtl (m *dns.Msg, minTtl time.Duration) (r time.Duration) {
	for _, rr := range m.Answer {
		if r < time.Duration(rr.Header().Ttl) {
			r = time.Duration(rr.Header().Ttl)
		}
	}
	if r < minTtl {
		r = minTtl
	}

	return r
}

func newCacheEntry(m *dns.Msg, mTtl time.Duration, gen uint64) *cacheEntry{
	ce := new(cacheEntry)
	ce.answer = m
	ce.ttl = minTtl(m, mTtl) * time.Second
	ce.setGeneration(gen)

	return ce;
}

func (ce *cacheEntry) generation() uint64 {
	return (&ce.gen).Load()
}

func (ce *cacheEntry) setGeneration(gen uint64)  {
	(&ce.gen).Store(gen)
}


func (ce *cacheEntry) Ip4s() (ips []string)  {
	ips = make([]string, 0, len(ce.answer.Answer))
	for _, rr := range ce.answer.Answer {
		if a, ok := rr.(*dns.A); ok {
			ips = append(ips, a.A.String())
		}
	}
	return ips
}

func (ce *cacheEntry) Ip6s() (ips []string)  {
	ips = make([]string, 0, len(ce.answer.Answer))
	for _, rr := range ce.answer.Answer {
		if a, ok := rr.(*dns.AAAA); ok {
			ips = append(ips, a.AAAA.String())
		}
	}
	return ips
}


