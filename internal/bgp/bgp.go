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
	"net/netip"
	"slices"
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
func newPath(prefix *bgpapi.IPAddressPrefix) *bgpapi.Path {
	nlri, _ := apb.New(prefix)

	a1, _ := apb.New(&bgpapi.OriginAttribute{
		Origin: 1, // IGP
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
	attrs := []*apb.Any{a1, a2, a3}
	return &bgpapi.Path{
		Family: v4Family,
		Nlri:   nlri,
		Pattrs: attrs,
	}
}
func add(prefix *bgpapi.IPAddressPrefix) error {
	if prefix == nil {
		return fmt.Errorf("prefix is nil")
	}

	log.L().Infof("Adding prefix: %s", prefix.String())
	if _, e := server.AddPath(context.Background(), &bgpapi.AddPathRequest{
		Path: newPath(prefix),
	}); e != nil {
		return fmt.Errorf("unable to add path: %v, %w", prefix, e)
	}

	return nil
}

func find(prefixes []*bgpapi.IPAddressPrefix) (found *bgpapi.IPAddressPrefix, e error) {
	found = nil
	if prefixes == nil {
		e = errors.New("prefix is nil")
		log.L().Warnf(e.Error())
		return found, e
	}

	tl := make([]*bgpapi.TableLookupPrefix, len(prefixes))
	for i, p := range prefixes {
		tl[i] = &bgpapi.TableLookupPrefix{
			Prefix: fmt.Sprintf("%s/%d", p.Prefix, p.PrefixLen),
			Type:   bgpapi.TableLookupPrefix_SHORTER,
		}
	}

	if e := server.ListPath(context.Background(), &bgpapi.ListPathRequest{
		TableType: bgpapi.TableType_GLOBAL,
		Family:    v4Family,
		Prefixes:  tl,
	}, func(dst *bgpapi.Destination) {
		p1, _ := netip.ParsePrefix(dst.Prefix)
		slices.IndexFunc(prefixes, func(prefix *bgpapi.IPAddressPrefix) bool {
			p2, _ := netip.ParsePrefix(fmt.Sprintf("%s/%d", prefix.Prefix, prefix.PrefixLen))

			if p1.Overlaps(p2) {
				found = prefix
				return true
			} else {
				return false
			}
		})
	}); e != nil {
		return found, e
	}

	return found, nil
}

func remove(prefix *bgpapi.IPAddressPrefix) error {
	if prefix == nil {
		return fmt.Errorf("prefix is nil")
	}

	found, _ := find([]*bgpapi.IPAddressPrefix{prefix})
	if found != nil {
		e := server.DeletePath(context.Background(), &bgpapi.DeletePathRequest{
			Path: newPath(prefix),
		})
		return e
	}
	return fmt.Errorf("prefix %s is not found", prefix.String())
}
