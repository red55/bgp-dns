package dns

import (
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/log"
)

func proxyQuery(w dns.ResponseWriter, rq *dns.Msg) {
	log.L().Debugf("Proxying request %v from: %s", rq, w.RemoteAddr().String())

	if r, e := _defaultResolvers.queryDns(rq); e != nil {
		log.L().Errorf("Forwarding response to upstream responder failed %v", e)
	} else {
		if e = w.WriteMsg(r); e != nil {
			log.L().Errorf("Failed to write response to client %s, %v", w.RemoteAddr(), e)
		}
	}

}

func respond(w dns.ResponseWriter, rq *dns.Msg) {
	var e error
	log.L().Debugf("Got DNS request from: %s, %v", w.RemoteAddr().String(), rq)

	for _, q := range rq.Question {
		switch q.Qtype {
		case dns.TypeA:
			var de *Entry
			if de, e = _cache.add(q.Name); e != nil {
				log.L().Errorf("Error resolving %s - %s", q.Name, e.Error())
			} else {
				var r = new(dns.Msg)
				r.SetReply(rq)
				r.Answer = de.r.Answer

				log.L().Debugf("Answering to %s: %s", w.RemoteAddr(), r)
				if e = w.WriteMsg(r); e != nil {
					log.L().Errorf("Failed to write response to client %s, %v", w.RemoteAddr(), e)
				}
			}
		default:
			//proxy to any resolver
			proxyQuery(w, rq)
		}
	}
}
