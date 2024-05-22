package bgp

import (
	"context"
	"errors"
	"fmt"
	bgpapi "github.com/osrg/gobgp/v3/api"
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/krt"
	"github.com/red55/bgp-dns/internal/log"
	"google.golang.org/protobuf/types/known/anypb"
	"net"
	"net/netip"
	"slices"
)

func newBgpPath(prefix *bgpapi.IPAddressPrefix) *bgpapi.Path {
	nlri, _ := anypb.New(prefix)

	a1, _ := anypb.New(&bgpapi.OriginAttribute{
		Origin: 1, // IGP
	})
	a2, _ := anypb.New(&bgpapi.NextHopAttribute{
		NextHop: cfg.AppCfg.Routing().Bgp().Id(),
	})
	a3, _ := anypb.New(&bgpapi.AsPathAttribute{
		Segments: []*bgpapi.AsSegment{
			{
				Type:    2,
				Numbers: []uint32{cfg.AppCfg.Routing().Bgp().Asn()},
			},
		},
	})
	attrs := []*anypb.Any{a1, a2, a3}
	return &bgpapi.Path{
		Family: _v4Family,
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
		Path: newBgpPath(prefix),
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
		Family:    _v4Family,
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
			Path: newBgpPath(prefix),
		})
		return e
	}
	return fmt.Errorf("prefix %s is not found", prefix.String())
}

func onBgpEvent(event *bgpapi.WatchEventResponse) {
	if p := event.GetPeer(); p != nil && p.Type == bgpapi.WatchEventResponse_PeerEvent_STATE {
		log.L().Infof("PeerEvent_STATE: %v", p)
	} else if t := event.GetTable(); t != nil {
		onBgpRoutingTableChange(t)
	}
}

func onBgpRoutingTableChange(event *bgpapi.WatchEventResponse_TableEvent) {
	for _, p := range event.Paths {
		sPath := path2String(p)
		log.L().Infof("[BGP] Path: %s", sPath)

		var idx = slices.IndexFunc(p.Pattrs, func(a *anypb.Any) bool {
			return a.TypeUrl == "type.googleapis.com/apipb.CommunitiesAttribute"
		})

		if idx == -1 {
			log.L().Debugf("Skiping path %s as it doesn't have community attr", sPath)
			continue
		}

		var cs bgpapi.CommunitiesAttribute
		if e := extractAttr(p.GetPattrs(), &cs); e != nil {
			log.L().Panicf("Failed to unmarshall communities: %v", e)
		}

		comms := cfg.AppCfg.Routing().Kernel().Inject().Communities()
		communitiesMatched := false
		for _, c := range comms {
			comm := parseCommunity(c)
			if comm != 0 && slices.Index(cs.Communities, comm) != -1 {
				communitiesMatched = true
				break
			}
		}

		if !communitiesMatched {
			log.L().Debugf("Skiping path %s as it's not marked with communities %v", sPath, comms)
			continue
		}

		var prefix bgpapi.IPAddressPrefix
		if e := a2s(p.GetNlri(), &prefix); e != nil {
			log.L().Panicf("Failed to unmarshal prefix: %v", e)
		}

		var nha bgpapi.NextHopAttribute
		if e := extractAttr(p.GetPattrs(), &nha); e != nil {
			log.L().Panicf("Failed to unmarshal NextHopAttribute: %v", e)
		}

		var n = &net.IPNet{
			IP:   net.ParseIP(prefix.Prefix),
			Mask: net.CIDRMask(int(prefix.PrefixLen), int(prefix.PrefixLen)),
		}
		var nh = net.ParseIP(nha.NextHop)
		var m = cfg.AppCfg.Routing().Kernel().Inject().Metric()

		if p.GetIsWithdraw() {
			krt.Withdraw(n, nh, m)
		} else /*if p.Best*/ {
			krt.Advance(n, nh, m)
		}
	}
}
