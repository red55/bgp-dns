package krt

import (
	"github.com/red55/bgp-dns/internal/log"
	"net"
	"slices"
)

type op int

const (
	opAdvance op = iota
	opWithdraw
	opQuit
)

type route struct {
	network *net.IPNet
	gateway net.IP
	metric  uint32
}
type routeOp struct {
	op op
	r  *route
}

var cmdChannel = make(chan *routeOp, 10)

func loop(c chan *routeOp) {
	for o := range c {
		switch o.op {
		case opAdvance:
			advance(o.r.network, o.r.gateway, o.r.metric)
			break
		case opWithdraw:
			found := routeTable.rtFind(o.r.network, o.r.gateway, o.r.metric)
			if found == nil {
				log.L().Infof("Route %s, next hop %s, metric: %d was not injected. Ignoring..",
					o.r.network.String(), o.r.gateway.String(), o.r.metric)
				continue
			}
			found.gs = slices.DeleteFunc(found.gs, func(ip net.IP) bool {
				return ip.Equal(o.r.gateway)
			})
			routeTable.rtRemove(found.n, o.r.gateway, found.m)
			withdraw(found.n, o.r.gateway, found.m, len(found.gs) == 0)
		case opQuit:
			//routeTable.m.Lock()
			for _, v := range routeTable.routes {
				l := len(v.gs) - 1
				for i, g := range v.gs {
					withdraw(v.n, g, v.m, i == l)
				}
				clear(v.gs)
			}
			clear(routeTable.routes)

			/*for len(routeTable.routes) > 0 {
				r := routeTable.routes
				withdraw(r.n, r.gs, r.m)
			}*/

			//routeTable.m.Unlock()
		default:
			return
		}
	}
}
