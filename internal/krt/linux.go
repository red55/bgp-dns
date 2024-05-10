//go:build linux

package krt

import (
	"errors"
	"github.com/jsimonetti/rtnetlink"
	"github.com/red55/bgp-dns/internal/log"
	"golang.org/x/sys/unix"
	"net"
	"syscall"
)

func newRouteMessage(network *net.IPNet, gw net.IP, metric uint16) *rtnetlink.RouteMessage {
	length, _ := network.Mask.Size()

	return &rtnetlink.RouteMessage{
		Family:    unix.AF_INET,
		DstLength: uint8(length),
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  unix.RTPROT_BGP,
		Scope:     unix.RT_SCOPE_UNIVERSE,
		Type:      unix.RTN_UNICAST,
		Attributes: rtnetlink.RouteAttributes{
			Dst:     network.IP,
			Gateway: gw,
			Table:   unix.RT_TABLE_MAIN,
		},
	}
}

func advance(network *net.IPNet, gw net.IP, metric uint16) {
	log.L().Infof("[Linux] Injecting kernel route %s, next hop: %s, metric: %d", network.String(), gw.String(), metric)

	c, e := rtnetlink.Dial(nil)
	if e != nil {
		log.L().Warnf("Open netlink socket failed: %v", e)
		return
	}
	defer c.Close()

	if i, _ := routeTable.rtFind(network, gw, metric); i != -1 {
		log.L().Infof("Route %s, next hop %s, metric: %d already injected", network.String(), gw.String(), metric)
		return
	}
	msg := newRouteMessage(network, gw, metric)
	if e := c.Route.Add(msg); e != nil {
		//ope := &netlink.OpError{}
		var ope syscall.Errno
		if errors.As(e, &ope); ope == syscall.EEXIST {
			log.L().Warnf("Route already exists, remember it for futher deleteion on exit")

		} else {
			log.L().Warnf("Route inject failed: %v", e)
			return
		}
	}
	routeTable.rtAdd(network, gw, metric)
}

func withdraw(net *net.IPNet, gw net.IP, metric uint16) {
	log.L().Infof("[Linux] Removing kernel route %s, next hop: %s, metric: %d", net.String(), gw.String(), metric)
	c, e := rtnetlink.Dial(nil)
	if e != nil {
		log.L().Warnf("Open netlink socket failed: %v", e)
		return
	}
	defer c.Close()
	if i, _ := routeTable.rtFind(net, gw, metric); i == -1 {
		log.L().Infof("Route %s, next hop %s, metric: %d was not injected. Ignoring..", net.String(), gw.String(), metric)
		return
	}
	if e := c.Route.Delete(newRouteMessage(net, gw, metric)); e != nil {
		log.L().Warnf("Route delete failed: %v", e)
		return
	}

	routeTable.rtRemove(net, gw, metric)
}
