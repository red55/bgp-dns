package bgp

import (
	"context"
	"errors"
	"fmt"
	bgpapi "github.com/osrg/gobgp/v3/api"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/log"
	apb "google.golang.org/protobuf/types/known/anypb"
	"net"
)

type operation int

const (
	opAdd operation = iota
	opRemove
	opQuit
)

var v4Family = &bgpapi.Family{
	Afi:  bgpapi.Family_AFI_IP,
	Safi: bgpapi.Family_SAFI_UNICAST,
}

type bgpOp struct {
	op     operation
	prefix *bgpapi.IPAddressPrefix
}

var chanRefresher chan *bgpOp

func Add(ip net.IP, length uint32) {
	chanRefresher <- &bgpOp{
		op: opAdd,
		prefix: &bgpapi.IPAddressPrefix{
			PrefixLen: length,
			Prefix:    ip.String(),
		},
	}
}
func Remove(ip net.IP) {
	chanRefresher <- &bgpOp{
		op: opRemove,
		prefix: &bgpapi.IPAddressPrefix{
			Prefix: ip.String(),
		},
	}
}
func add(prefix *bgpapi.IPAddressPrefix) error {
	if prefix == nil {
		return fmt.Errorf("prefix is nil")
	}
	nlri, _ := apb.New(prefix)

	a1, _ := apb.New(&bgpapi.OriginAttribute{
		Origin: 0,
	})
	a2, _ := apb.New(&bgpapi.NextHopAttribute{
		NextHop: cfg.AppCfg.Bgp.Id,
	})
	a3, _ := apb.New(&bgpapi.AsPathAttribute{
		Segments: []*bgpapi.AsSegment{
			{
				Type:    2,
				Numbers: []uint32{cfg.AppCfg.Bgp.Asn},
			},
		},
	})
	/*
		a4, _ := apb.New(&bgpapi.CommunitiesAttribute{
			Communities: comms,
		})*/
	attrs := []*apb.Any{a1, a2, a3 /*, a4*/}

	if _, e := server.AddPath(context.Background(), &bgpapi.AddPathRequest{
		Path: &bgpapi.Path{
			Family: &bgpapi.Family{Afi: bgpapi.Family_AFI_IP, Safi: bgpapi.Family_SAFI_UNICAST},
			Nlri:   nlri,
			Pattrs: attrs,
		},
	}); e != nil {
		return fmt.Errorf("unable to add path: %v, %w", prefix, e)
	}

	return nil
}

func find(prefix *bgpapi.IPAddressPrefix) (*bgpapi.IPAddressPrefix, error) {
	var found = false
	if prefix == nil {
		e := errors.New("prefix is nil")
		log.L().Warnf(e.Error())
		return nil, e
	}
	if e := server.ListPath(context.Background(), &bgpapi.ListPathRequest{
		Family: v4Family,
		Prefixes: []*bgpapi.TableLookupPrefix{
			{
				Prefix: prefix.Prefix,
			},
		},
	}, func(dst *bgpapi.Destination) {
		if dst.Prefix == prefix.Prefix {
			found = true
		}
	}); e != nil {
		return nil, e
	}

	if found {
		return prefix, nil
	} else {
		return nil, nil
	}
}

func remove(prefix *bgpapi.IPAddressPrefix) error {
	if prefix == nil {
		return fmt.Errorf("prefix is nil")
	}

	found, _ := find(prefix)
	if found == nil {
		return nil
	}
	return fmt.Errorf("prefix %s is not found", prefix.String())
}
