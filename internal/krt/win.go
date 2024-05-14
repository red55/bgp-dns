//go:build windows

package krt

import (
	"github.com/red55/bgp-dns/internal/log"
	"net"
)

func advance(net *net.IPNet, gw net.IP, metric uint32) {
	log.L().Panicf("[Win, Not implemented] Injecting kernel route %s, next hop: %s, metric: %d", net.String(), gw.String(), metric)
}

func withdraw(net *net.IPNet, gw net.IP, metric uint32) {
	log.L().Panicf("[Win, Not implemented] Removing kernel route %s, next hop: %s, metric: %d", net.String(), gw.String(), metric)
}
