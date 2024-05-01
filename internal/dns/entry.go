package dns

import (
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"time"
)

type Entry struct {
	fqdn   string
	ttl    time.Duration
	expire time.Time
	ips    []string
	r      *dns.Msg
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
	de.expire = time.Now().Add(ttl)
}

func (de *Entry) Expire() time.Time {
	return de.expire
}

func (de *Entry) String() string {
	return fmt.Sprintf("%s -> (TTL: %s, Expire at: %s) %v", de.fqdn, de.Ttl(), de.Expire(), de.ips)
}
