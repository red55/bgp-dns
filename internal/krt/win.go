//go:build windows

package krt

import (
	"github.com/red55/bgp-dns/internal/log"
	"net"
)

func advance(net *net.IPNet, gw net.IP, metric uint16) {
	log.L().Infof("[Win] Injecting kernel route %s, next hop: %s, metric: %d", net.String(), gw.String(), metric)
}

func withdraw(net *net.IPNet, gw net.IP, metric uint16) {
	log.L().Infof("[Win] Removing kernel route %s, next hop: %s, metric: %d", net.String(), gw.String(), metric)
}
