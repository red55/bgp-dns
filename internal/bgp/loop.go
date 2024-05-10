package bgp

import (
	bgpapi "github.com/osrg/gobgp/v3/api"
	"github.com/red55/bgp-dns/internal/log"
)

type op int

const (
	opAdd op = iota
	opRemove
	opQuit
)

var v4Family = &bgpapi.Family{
	Afi:  bgpapi.Family_AFI_IP,
	Safi: bgpapi.Family_SAFI_UNICAST,
}

type bgpOp struct {
	op     op
	prefix *bgpapi.IPAddressPrefix
}

var cmdChannel = make(chan *bgpOp)

func loop(ch chan *bgpOp) {
	for op := range ch {
		switch op.op {
		case opAdd:
			if e := add(op.prefix); e != nil {
				log.L().Errorf("Failed to add prefix: %v", e)
			}
		case opRemove:
			if e := remove(op.prefix); e != nil {
				log.L().Errorf("Failed to remove prefix: %v", e)
			}
		case opQuit:
			return
		}
	}
}
