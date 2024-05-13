//go:build linux

package krt

import (
	"errors"
	"github.com/jsimonetti/rtnetlink"
	"github.com/red55/bgp-dns/internal/log"
	"golang.org/x/sys/unix"
	"net"
	"slices"
	"syscall"
)

func newRouteMessage(network *net.IPNet, gw []net.IP, metric uint32) *rtnetlink.RouteMessage {
	length, _ := network.Mask.Size()

	rm := &rtnetlink.RouteMessage{
		Family:    unix.AF_INET,
		DstLength: uint8(length),
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  unix.RTPROT_BGP,
		Scope:     unix.RT_SCOPE_UNIVERSE,
		Type:      unix.RTN_UNICAST,
		Attributes: rtnetlink.RouteAttributes{
			Dst: network.IP,
			//Gateway:  gw,
			Table:    unix.RT_TABLE_MAIN,
			Priority: metric,
		},
	}
	rm.Attributes.Multipath = make([]rtnetlink.NextHop, len(gw))
	for i, g := range gw {
		rm.Attributes.Multipath[i].Gateway = g
	}
	return rm
}
func newRouteMessage2(network *net.IPNet, gw []net.IP, metric uint32) *rtnetlink.RouteMessage {
	rm := newRouteMessage(network, gw, metric)
	rm.Attributes.Gateway = gw[0]
	rm.Attributes.Multipath = make([]rtnetlink.NextHop, 0)

	return rm
}

func advance(network *net.IPNet, gw net.IP, metric uint32) {
	log.L().Infof("[Linux] Injecting kernel route %s, next hop: %s, metric: %d", network.String(), gw.String(), metric)

	c, e := rtnetlink.Dial(nil)
	if e != nil {
		log.L().Warnf("Open netlink socket failed: %v", e)
		return
	}
	defer c.Close()

	if found := routeTable.rtFind(network, gw, metric); found != nil {
		idx := slices.IndexFunc(found.gs, func(ip net.IP) bool {
			return ip.Equal(gw)
		})
		if idx != -1 {
			log.L().Infof("Route %s, next hop %s, metric: %d already injected", network.String(), gw.String(), metric)
			return
		}
	}

	kroute := routeTable.rtAddOrUpdate(network, gw, metric)

	msg := newRouteMessage(network, kroute.gs, metric)
	if len(kroute.gs) == 1 {
		e = c.Route.Add(msg)
	} else {
		e = c.Route.Replace(msg)
	}
	if e != nil {
		var ope syscall.Errno
		if errors.As(e, &ope); ope == syscall.EEXIST {
			log.L().Warnf("Route already exists, remember it for futher deleteion on exit")
		} else {
			log.L().Warnf("Route inject failed: %v", e)
			return
		}
	}
}
func withdraw(network *net.IPNet, gw net.IP, metric uint32, del bool) {
	log.L().Infof("[Linux] Removing kernel route %s, next hop: %s, metric: %d", network.String(), gw.String(), metric)
	c, e := rtnetlink.Dial(nil)
	if e != nil {
		log.L().Warnf("Open netlink socket failed: %v", e)
		return
	}
	defer c.Close()
	rm := newRouteMessage2(network, []net.IP{gw}, metric)
	if del {
		e = c.Route.Delete(rm)
	} else {
		e = c.Route.Replace(rm)
	}

	if e != nil {
		log.L().Warnf("Route delete/replace failed: %v", e)
	}
}
