package dns

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"sync/atomic"
	"time"
)

type cacheEntry struct {
	gen        atomic.Uint64
	ttl        time.Duration
	answer     *dns.Msg
	expiration time.Time
	failures   atomic.Uint64
}

var (
	ENotAddressAnswer = errors.New("not an A/AAAA answer")
)

func minTtl(m *dns.Msg, minTtl time.Duration) (r time.Duration) {
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

func newCacheEntry(m *dns.Msg, mTtl time.Duration, gen uint64) *cacheEntry {
	ce := new(cacheEntry)
	ce.answer = m
	ce.updateTtl(mTtl)
	ce.setGeneration(gen)
	ce.failures.Store(0)
	return ce
}

func (ce *cacheEntry) updateTtl(mTtlSeconds time.Duration) {
	mTtlSeconds = minTtl(ce.answer, mTtlSeconds)
	ce.ttl = mTtlSeconds
	ce.expiration = time.Now().Add(ce.ttl * time.Second)
}

func (ce *cacheEntry) generation() uint64 {
	return (&ce.gen).Load()
}

func (ce *cacheEntry) setGeneration(gen uint64) {
	(&ce.gen).Store(gen)
}

func (ce *cacheEntry) Ip4s() (ips []string) {
	ips = make([]string, 0, len(ce.answer.Answer))
	for _, rr := range ce.answer.Answer {
		if a, ok := rr.(*dns.A); ok {
			ips = append(ips, a.A.String())
		}
	}
	return ips
}

func (ce *cacheEntry) Ip6s() (ips []string) {
	ips = make([]string, 0, len(ce.answer.Answer))
	for _, rr := range ce.answer.Answer {
		if a, ok := rr.(*dns.AAAA); ok {
			ips = append(ips, a.AAAA.String())
		}
	}
	return ips
}

func (ce *cacheEntry) IncFailures() uint64 {
	return ce.failures.Add(1)
}

func (ce *cacheEntry) Failures() uint64 {
	return ce.failures.Load()
}
func (ce *cacheEntry) ResetFailures() {
	ce.failures.Store(0)
}
func (ce *cacheEntry) String() string {
	var fqdn string
	if len(ce.answer.Question) < 1 {
		fqdn = "not an A/AAAA answer"
	}
	fqdn = ce.answer.Question[0].Name
	return fmt.Sprintf("fqdn: %s, gen: %d, ttl: %v, expiration: %v, failures: %d",
		fqdn, ce.generation(), ce.ttl, ce.expiration.Format(time.RFC3339), ce.Failures())
}
