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
	if gw != nil {
		rm.Attributes.Multipath = make([]rtnetlink.NextHop, len(gw))
		for i, g := range gw {
			rm.Attributes.Multipath[i].Gateway = g
		}
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
	log.L().Infof("[Linux] Route %s, next hop %s, metric %d: injecting...", network.String(), gw.String(), metric)

	c, e := rtnetlink.Dial(nil)
	if e != nil {
		log.L().Warnf("[Linux] Route %s, next hop %s, metric %d: Open netlink socket failed: %v.",
			network.String(), gw.String(), metric, e)
		return
	}
	defer c.Close()

	if found := _routeTable.rtFind(network); found != nil {
		idx := slices.IndexFunc(found.gs, func(ip net.IP) bool {
			return ip.Equal(gw)
		})
		if idx != -1 {
			log.L().Infof("[Linux] Route %s, next hop %s, metric %d: already injected.", network.String(), gw.String(), metric)
			return
		}
	}

	kroute := _routeTable.rtAddOrUpdate(network, gw, metric)

	msg := newRouteMessage(network, kroute.gs, metric)
	if len(kroute.gs) == 1 {
		e = c.Route.Add(msg)
	} else {
		e = c.Route.Replace(msg)
	}
	if e != nil {
		var ope syscall.Errno
		if errors.As(e, &ope); ope == syscall.EEXIST {
			log.L().Warnf("[Linux] Route %s, next hop %s, metric %d: already exists, remember it for futher deleteion on exit",
				network.String(), gw.String(), metric)
		} else {
			log.L().Warnf("[Linux] Route %s, next hop %s, metric %d: route inject failed: %v",
				network.String(), gw.String(), metric, e)
			return
		}
	} else {
		log.L().Infof("[Linux] Route %s, next hop %s, metric %d: Injected", network.String(), gw.String(), metric)
	}

}
func withdraw(network *net.IPNet, gw net.IP, metric uint32, del bool) {
	log.L().Infof("[Linux] Route %s, next hop %s, metric %d Removing...", network.String(), gw.String(), metric)
	c, e := rtnetlink.Dial(nil)
	if e != nil {
		log.L().Warnf("[Linux] Route %s, next hop %s, metric %d: open netlink socket failed: %v",
			network.String(), gw.String(), metric, e)
		return
	}
	defer c.Close()
	var rr []rtnetlink.RouteMessage
	if rr, e = c.Route.List(); e != nil {
		log.L().Errorf("[Linux] Route %s, next hop %s, metric %d: route list failed: %v",
			network.String(), gw.String(), metric, e)
		return
	}
	l, _ := network.Mask.Size()
	idx := slices.IndexFunc(rr, func(msg rtnetlink.RouteMessage) bool {
		if msg.Family == unix.AF_INET &&
			msg.DstLength == uint8(l) &&
			msg.Table == unix.RT_TABLE_MAIN &&
			msg.Protocol == unix.RTPROT_BGP &&
			msg.Scope == unix.RT_SCOPE_UNIVERSE &&
			msg.Type == unix.RTN_UNICAST &&
			msg.Attributes.Dst.Equal(network.IP) &&
			msg.Attributes.Table == unix.RT_TABLE_MAIN &&
			msg.Attributes.Priority == metric {

			var b = false
			if len(msg.Attributes.Multipath) > 0 {
				b = slices.IndexFunc(msg.Attributes.Multipath, func(hop rtnetlink.NextHop) bool {
					return hop.Gateway.Equal(gw)
				}) != -1
			} else {
				b = gw.Equal(msg.Attributes.Gateway)
			}

			return b
		}
		return false
	})

	if idx == -1 {
		log.L().Warnf("[Linux] Route %s, next hop %s, metric %d: route not found, ignoring: %v",
			network.String(), gw.String(), metric, e)
		return
	}

	msg := &rr[idx]

	msg.Attributes.Multipath = slices.DeleteFunc(msg.Attributes.Multipath, func(hop rtnetlink.NextHop) bool {
		return hop.Gateway.Equal(gw)
	})

	if len(msg.Attributes.Multipath) == 0 {
		e = c.Route.Delete(msg)
	} else {
		e = c.Route.Replace(msg)
	}

	if e != nil {
		log.L().Warnf("[Linux] Route %s, next hop %s, metric %d: route delete/replace failed: %v",
			network.String(), gw.String(), metric, e)
	} else {
		log.L().Infof("[Linux] Route %s, next hop %s, metric %d: Removed.", network.String(), gw.String(), metric)
	}
}
