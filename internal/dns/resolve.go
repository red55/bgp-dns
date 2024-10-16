package dns
import (
	"github.com/red55/bgp-dns/internal/log"
	"github.com/miekg/dns"
	"slices"
)

func resolve(w dns.ResponseWriter, q *dns.Msg)  {
	var a *dns.Msg
	var e error

	if a, e = _cache.rs.query(q); e != nil {
		log.L().Error().Err(e)
		return
	}

	if w != nil {
		if e = w.WriteMsg(a); e != nil {
			log.L().Error().Err(e)
			return
		}
	}

	i := slices.IndexFunc(a.Answer, func(rr dns.RR) bool {
		return rr.Header().Rrtype == dns.TypeA
	})
	qn := q.Question[0].Name

	if i > -1 {
		if e = _cache.upsert(qn, a); e != nil {
			log.L().Warn().Err(e)
			return
		}
	} else {
		log.L().Trace().Msgf("Empty Answer for %s, RCode: %d", qn, a.Rcode)
		if _cache.has(qn) {
			Unregister(qn)
		} else {
			log.L().Trace().Msgf("%s not in cache, ignore...", qn)
		}
	}
//else {

//	}
}