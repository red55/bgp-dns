package krt

import "net"

type op int

const (
	opAdvance op = iota
	opWithdraw
	opQuit
)

type route struct {
	network *net.IPNet
	gateway net.IP
	metric  uint16
}
type routeOp struct {
	op op
	r  *route
}

var cmdChannel = make(chan *routeOp)

func loop(c chan *routeOp) {
	for o := range c {
		switch o.op {
		case opAdvance:
			advance(o.r.network, o.r.gateway, o.r.metric)
		case opWithdraw:
			withdraw(o.r.network, o.r.gateway, o.r.metric)
		case opQuit:
		default:
			routeTable.m.Lock()
			for len(routeTable.routes) > 0 {
				r := routeTable.routes[0]
				withdraw(r.n, r.g, r.m)
			}
			routeTable.routes = make([]*krtEntry, 0)
			routeTable.m.Unlock()
			break
		}
	}
}
