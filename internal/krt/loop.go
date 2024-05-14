package krt

import (
	"github.com/red55/bgp-dns/internal/log"
	"net"
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

var _cmdChannel = make(chan *routeOp, 10)

func loop(c chan *routeOp) {
	_wg.Add(1)
	defer _wg.Done()
	for o := range c {
		switch o.op {
		case opAdvance:
			advance(o.r.network, o.r.gateway, o.r.metric)
			break
		case opWithdraw:
			withdraw(o.r.network, o.r.gateway, o.r.metric)
		case opQuit:
			//_routeTable.m.Lock()
			for _, v := range _routeTable.routes {
				for _, g := range v.gs {
					rtnlWithdraw(v.n, g, v.m)
				}
				clear(v.gs)
			}
			clear(_routeTable.routes)
			return
			/*for len(_routeTable.routes) > 0 {
				r := _routeTable.routes
				withdraw(r.n, r.gs, r.m)
			}*/

			//_routeTable.m.Unlock()
		default:
			log.L().Errorf("Krt loop got unknwon command: %v", o)
			return
		}
	}
}
