package dns

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func helperNewARecord(fqdn string, ip net.IP) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(fqdn, dns.TypeA)
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: ip,
	})
	return msg
}

// TestCacheEntry_Ip4s tests that IP4s correctly extracts IPv4 addresses from A records
func TestCacheEntry_Ip4s(t *testing.T) {
	fqdn := "example.com."
	ip := net.ParseIP("192.168.1.1")
	msg := helperNewARecord(fqdn, ip)

	ce := newCacheEntry(msg, time.Duration(60), 1)

	ips := ce.Ip4s()
	if len(ips) != 1 {
		t.Errorf("expected 1 IP, got %d", len(ips))
	}
	if ips[0] != "192.168.1.1" {
		t.Errorf("expected IP 192.168.1.1, got %s", ips[0])
	}
}

// TestCacheEntry_Ip4s_MultipleIPs tests extracting multiple IPs from a single response
func TestCacheEntry_Ip4s_MultipleIPs(t *testing.T) {
	fqdn := "example.com."
	msg := new(dns.Msg)
	msg.SetQuestion(fqdn, dns.TypeA)

	ips := []string{"192.168.1.1", "192.168.1.2", "10.0.0.1"}
	for _, ipStr := range ips {
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   fqdn,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.ParseIP(ipStr),
		})
	}

	ce := newCacheEntry(msg, time.Duration(60), 1)

	resultIPs := ce.Ip4s()
	if len(resultIPs) != len(ips) {
		t.Errorf("expected %d IPs, got %d", len(ips), len(resultIPs))
	}

	// Check all expected IPs are present
	ipMap := make(map[string]bool)
	for _, ip := range resultIPs {
		ipMap[ip] = true
	}
	for _, expectedIP := range ips {
		if !ipMap[expectedIP] {
			t.Errorf("expected IP %s not found in result", expectedIP)
		}
	}
}

// TestCacheEntry_Failures tests the failure counter functionality
func TestCacheEntry_Failures(t *testing.T) {
	fqdn := "example.com."
	ip := net.ParseIP("192.168.1.1")
	msg := helperNewARecord(fqdn, ip)

	ce := newCacheEntry(msg, time.Duration(60), 1)

	if ce.Failures() != 0 {
		t.Errorf("expected 0 failures initially, got %d", ce.Failures())
	}

	ce.IncFailures()
	ce.IncFailures()
	ce.IncFailures()

	if ce.Failures() != 3 {
		t.Errorf("expected 3 failures, got %d", ce.Failures())
	}

	ce.ResetFailures()
	if ce.Failures() != 0 {
		t.Errorf("expected 0 failures after reset, got %d", ce.Failures())
	}
}

// TestCacheEntry_Generation tests the generation counter functionality
func TestCacheEntry_Generation(t *testing.T) {
	fqdn := "example.com."
	ip := net.ParseIP("192.168.1.1")
	msg := helperNewARecord(fqdn, ip)

	ce := newCacheEntry(msg, time.Duration(60), 5)

	if ce.generation() != 5 {
		t.Errorf("expected generation 5, got %d", ce.generation())
	}

	ce.setGeneration(10)
	if ce.generation() != 10 {
		t.Errorf("expected generation 10, got %d", ce.generation())
	}
}

// TestCacheKey_String tests the cache key string representation
func TestCacheKey_String(t *testing.T) {
	key := newCacheKey("example.com.", dns.TypeA)
	expected := "example.com.:A"
	if key.String() != expected {
		t.Errorf("expected key string %q, got %q", expected, key.String())
	}

	keyHTTPS := newCacheKey("example.com.", dns.TypeHTTPS)
	expectedHTTPS := "example.com.:HTTPS"
	if keyHTTPS.String() != expectedHTTPS {
		t.Errorf("expected key string %q, got %q", expectedHTTPS, keyHTTPS.String())
	}
}

// TestCacheKey_Equals tests the cache key equality
func TestCacheKey_Equals(t *testing.T) {
	key1 := newCacheKey("example.com.", dns.TypeA)
	key2 := newCacheKey("example.com.", dns.TypeA)
	key3 := newCacheKey("other.com.", dns.TypeA)
	key4 := newCacheKey("example.com.", dns.TypeHTTPS)

	if !key1.Equals(key2) {
		t.Error("expected keys to be equal")
	}
	if key1.Equals(key3) {
		t.Error("expected keys to be different (different FQDN)")
	}
	if key1.Equals(key4) {
		t.Error("expected keys to be different (different QType)")
	}
}

// TestNewCacheEntry_TTL tests that TTL is correctly set from the DNS message
func TestNewCacheEntry_TTL(t *testing.T) {
	fqdn := "example.com."
	msg := helperNewARecord(fqdn, net.ParseIP("192.168.1.1"))

	ce := newCacheEntry(msg, time.Duration(60), 1)

	// TTL should be at least the minimum TTL from the message (300)
	if ce.ttl < 300 {
		t.Errorf("expected TTL >= 300, got %d", ce.ttl)
	}

	// Expiration should be roughly now + TTL
	expectedExpiry := time.Now().Add(time.Duration(ce.ttl) * time.Second)
	diff := ce.expiration.Sub(expectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expiration time is off by more than 1 second: expected ~%v, got %v", expectedExpiry, ce.expiration)
	}
}

// TestCacheEntry_Ip4s_EmptyAnswer tests IP extraction when there are no A records
func TestCacheEntry_Ip4s_EmptyAnswer(t *testing.T) {
	fqdn := "example.com."
	msg := new(dns.Msg)
	msg.SetQuestion(fqdn, dns.TypeA)
	// No answer records

	ce := newCacheEntry(msg, time.Duration(60), 1)

	ips := ce.Ip4s()
	if len(ips) != 0 {
		t.Errorf("expected 0 IPs for empty answer, got %d", len(ips))
	}
}

// TestCacheEntry_Ip6s tests that IP6s correctly extracts IPv6 addresses from AAAA records
func TestCacheEntry_Ip6s(t *testing.T) {
	fqdn := "example.com."
	msg := new(dns.Msg)
	msg.SetQuestion(fqdn, dns.TypeAAAA)
	msg.Answer = append(msg.Answer, &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		AAAA: net.ParseIP("2001:db8::1"),
	})

	ce := newCacheEntry(msg, time.Duration(60), 1)

	ips := ce.Ip6s()
	if len(ips) != 1 {
		t.Errorf("expected 1 IPv6 address, got %d", len(ips))
	}
	if ips[0] != "2001:db8::1" {
		t.Errorf("expected IP 2001:db8::1, got %s", ips[0])
	}
}

// TestCacheEntry_Ip6s_NoAAAA tests that IP6s returns empty for non-AAAA records
func TestCacheEntry_Ip6s_NoAAAA(t *testing.T) {
	fqdn := "example.com."
	msg := helperNewARecord(fqdn, net.ParseIP("192.168.1.1"))

	ce := newCacheEntry(msg, time.Duration(60), 1)

	ips := ce.Ip6s()
	if len(ips) != 0 {
		t.Errorf("expected 0 IPv6 addresses for A record, got %d", len(ips))
	}
}

// TestMinTtl_UsesHighestRecordTTLWithConfiguredMinimum verifies that minTtl
// returns the highest TTL from the answer records unless the configured
// minimum TTL is higher, in which case the configured minimum is used.
func TestMinTtl_UsesHighestRecordTTLWithConfiguredMinimum(t *testing.T) {
	fqdn := "example.com."
	msg := new(dns.Msg)
	msg.SetQuestion(fqdn, dns.TypeA)

	// Add records with different TTLs.
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    100,
		},
		A: net.ParseIP("192.168.1.1"),
	})
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   fqdn,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    200,
		},
		A: net.ParseIP("192.168.1.2"),
	})

	// When the configured minimum is below all record TTLs, the highest record
	// TTL is used.
	result := minTtl(msg, time.Duration(60))
	if result != 200 {
		t.Errorf("expected minTtl to be 200 (highest record TTL when above configured minimum), got %d", result)
	}

	// When the configured minimum is greater than all record TTLs, it overrides
	// the record TTLs.
	result = minTtl(msg, time.Duration(300))
	if result != 300 {
		t.Errorf("expected minTtl to be 300 (configured minimum override), got %d", result)
	}
}

// TestNewCacheEntry_UpdateTtl tests that TTL can be updated
func TestNewCacheEntry_UpdateTtl(t *testing.T) {
	fqdn := "example.com."
	msg := helperNewARecord(fqdn, net.ParseIP("192.168.1.1"))

	ce := newCacheEntry(msg, time.Duration(60), 1)
	oldTtl := ce.ttl
	oldExpiration := ce.expiration

	// Update with a different min TTL
	ce.updateTtl(time.Duration(120))

	// TTL should still be 300 (from the record) since 300 > 120
	if ce.ttl != oldTtl {
		t.Errorf("TTL should remain based on record TTL when above min, was %d, now %d", oldTtl, ce.ttl)
	}

	// Expiration should have been updated (should be very close to now + TTL)
	newExpectedExpiry := time.Now().Add(time.Duration(ce.ttl) * time.Second)
	diff := ce.expiration.Sub(newExpectedExpiry)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expiration time should be refreshed after updateTtl: expected ~%v, got %v (diff: %v)", newExpectedExpiry, ce.expiration, diff)
	}

	// Expiration should be different from the old one (refreshed to current time)
	if !ce.expiration.After(oldExpiration) || ce.expiration.Sub(oldExpiration) > 2*time.Second {
		t.Errorf("expiration should be refreshed from current time within a reasonable range: old=%v, new=%v, delta=%v", oldExpiration, ce.expiration, ce.expiration.Sub(oldExpiration))
	}
}
