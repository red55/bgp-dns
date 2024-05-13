package krt

import (
	"github.com/red55/bgp-dns/internal/log"
	"net"
	"sync"
)

var _wg sync.WaitGroup

func Init() {
	log.L().Info("Kernel route manager -> init")

	go loop(_cmdChannel)
}

func Advance(network *net.IPNet, gw net.IP, metric uint32) {
	go func() {
		_cmdChannel <- &routeOp{
			op: opAdvance,
			r: &route{
				network: network,
				gateway: gw,
				metric:  metric,
			},
		}
	}()
}

func Withdraw(network *net.IPNet, gw net.IP, metric uint32) {

	go func() {
		_cmdChannel <- &routeOp{
			op: opWithdraw,
			r: &route{
				network: network,
				gateway: gw,
				metric:  metric,
			},
		}
	}()
}

func Deinit() {
	log.L().Infof("Krt->Deinit(): Enter")
	defer log.L().Infof("Krt->Deinit(): Done")

	_cmdChannel <- &routeOp{
		op: opQuit,
		r:  nil,
	}

	_wg.Wait()
}
