package dns
import (
	"github.com/miekg/dns"
	"slices"
)

func (c *cache) resolve(w dns.ResponseWriter, q *dns.Msg)  {
	var a *dns.Msg
	var e error

	if a, e = c.rs.query(q); e != nil {
		c.L().Error().Err(e)
		return
	}

	if w != nil {
		if e = w.WriteMsg(a); e != nil {
			c.L().Error().Err(e)
			return
		}
	}

	i := slices.IndexFunc(a.Answer, func(rr dns.RR) bool {
		return rr.Header().Rrtype == dns.TypeA
	})
	qn := q.Question[0].Name

	if i > -1 {
		if e = c.upsert(qn, a); e != nil {
			c.L().Warn().Err(e)
			return
		}
	} else {
		c.L().Trace().Msgf("Empty Answer for %s, RCode: %d", qn, a.Rcode)
		if c.has(qn) {
			if e = c.unregister(qn); e!=nil {
				c.L().Error().Err(e).Msgf("Failed to unregister %s from resolve", qn)
			}
		} else {
			c.L().Trace().Msgf("%s not in cache, ignore...", qn)
		}
	}
}