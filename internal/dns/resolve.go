package dns

import (
	"errors"
	"github.com/miekg/dns"
)

func (c *cache) resolve(w dns.ResponseWriter, q *dns.Msg, notifyChanged bool) {
	var a *dns.Msg
	var e error

	if a, e = c.rs.query(q); e != nil && !errors.Is(e, ErrEmptyAnswer) || a == nil {
		c.L().Error().Err(e).Msg("Failed to query Resolvers")
		return
	}

	if w != nil {
		if e = w.WriteMsg(a); e != nil {
			c.L().Error().Err(e).Msg("Failed to write answer to client")
			return
		}
	}
	/*
		i := slices.IndexFunc(a.Answer, func(rr dns.RR) bool {
			return rr.Header().Rrtype == dns.TypeA
		})
	*/

	qn := q.Question[0].Name

	// if i > -1 {
	if e = c.upsert(qn, a); e != nil {
		c.L().Warn().Err(e).Msg("Failed to update cache")
		return
	}
	if notifyChanged {
		c.notifyChanged(qn)
	}
	/*
		} else {
			c.L().Trace().Msgf("Not an A Answer for %s, RCode: %d", qn, a.Rcode)

			ce := c.get(qn)
			if ce != nil {
				c.L().Warn().Msgf("Increasing falure count for %s", qn)
				ce.IncFailures()
			} else {
				c.L().Trace().Msgf("%s not in cache, ignore...", qn)
			}
		}

	*/
}
