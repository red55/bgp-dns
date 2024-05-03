package dns

import (
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"time"
)

type Entry struct {
	fqdn   string
	ttl    uint32
	expire time.Time
	ips    []string
	r      *dns.Msg
}

func NewEntry(fqdn string) *Entry {
	r := &Entry{
		fqdn: fqdn}
	r.SetTtl(cfg.AppCfg.Timeouts().DefaultTTL())
	return r
}

func (de *Entry) Fqdn() string {
	return de.fqdn
}

func (de *Entry) IPs() []string {
	return de.ips
}

func (de *Entry) Ttl() uint32 {
	return de.ttl
}

func (de *Entry) SetTtl(ttl uint32) {
	de.ttl = ttl
	de.expire = time.Now().Add(time.Duration(de.ttl) * time.Second)
}

func (de *Entry) Expire() time.Time {
	return de.expire
}

func (de *Entry) String() string {
	return fmt.Sprintf("%s -> (TTL: %d, Expire at: %s) %v", de.fqdn, de.Ttl(), de.Expire(), de.ips)
}
