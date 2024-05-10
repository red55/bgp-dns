package krt

import (
	"github.com/red55/bgp-dns/internal/log"
	"net"
)

func Init() {
	log.L().Info("Kernel route manager -> init")

	go loop(cmdChannel)
}

func Advance(network *net.IPNet, gw net.IP, metric uint16) {
	cmdChannel <- &routeOp{
		op: opAdvance,
		r: &route{
			network: network,
			gateway: gw,
			metric:  metric,
		},
	}
}

func Withdraw(network *net.IPNet, gw net.IP, metric uint16) {
	cmdChannel <- &routeOp{
		op: opWithdraw,
		r: &route{
			network: network,
			gateway: gw,
			metric:  metric,
		},
	}
}

func Deinit() {
	log.L().Info("Kernel route manager -> deinit")

	cmdChannel <- &routeOp{
		op: opQuit,
		r:  nil,
	}
}
