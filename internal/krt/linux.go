//go:build linux

package krt

import (
	"github.com/red55/bgp-dns-peer/internal/log"
	"net"
)

func Advance(net *net.IPNet, gw net.IP, metric uint32) {
	log.L().Infof("Injecting kernel route %s, next hop: %s, metric: %d", net.String(), gw.String(), metric)
}

func Withdraw(net *net.IPNet, gw net.IP, metric uint32) {
	log.L().Infof("Removing kernel route %s, next hop: %s, metric: %d", net.String(), gw.String(), metric)
}
