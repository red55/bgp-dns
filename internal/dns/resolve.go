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

	qn := q.Question[0].Name

	if e = c.upsert(qn, q.Question[0].Qtype, a); e != nil {
		c.L().Warn().Err(e).Msg("Failed to update cache")
		return
	}
	if notifyChanged {
		c.notifyChanged(qn)
	}
}
