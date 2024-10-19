package bgp
import (
	"context"
	"fmt"
	bgpapi "github.com/osrg/gobgp/v3/api"
	"google.golang.org/protobuf/types/known/anypb"
	"errors"
	"net/netip"
	"slices"
)

func newBgpPath(prefix *bgpapi.IPAddressPrefix, asn uint32, nh string) *bgpapi.Path {
	nlri, _ := anypb.New(prefix)

	a1, _ := anypb.New(&bgpapi.OriginAttribute{
		Origin: 1, // IGP
	})
	a2, _ := anypb.New(&bgpapi.NextHopAttribute{
		NextHop: nh,
	})
	a3, _ := anypb.New(&bgpapi.AsPathAttribute{
		Segments: []*bgpapi.AsSegment{
			{
				Type:    bgpapi.AsSegment_AS_SEQUENCE,
				Numbers: []uint32{asn},
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

func (s *bgpSrv) add(prefix *bgpapi.IPAddressPrefix, asn uint32) error {
	if prefix == nil {
		return fmt.Errorf("prefix is nil")
	}

	s.L().Info().Msgf("Adding prefix: %s", prefix.String())
	//TODO: pass context
	if _, e := s.bgp.AddPath(context.Background(), &bgpapi.AddPathRequest{
		Path: newBgpPath(prefix, asn, s.id.String()),
	}); e != nil {
		return fmt.Errorf("unable to add path: %v, %w", prefix, e)
	}

	return nil
}

func (s *bgpSrv) find(prefixes []*bgpapi.IPAddressPrefix) (found *bgpapi.IPAddressPrefix, e error) {
	found = nil
	if prefixes == nil {
		e = errors.New("prefix is nil")
		return found, e
	}

	tl := make([]*bgpapi.TableLookupPrefix, len(prefixes))
	for i, p := range prefixes {
		tl[i] = &bgpapi.TableLookupPrefix{
			Prefix: fmt.Sprintf("%s/%d", p.Prefix, p.PrefixLen),
			Type:   bgpapi.TableLookupPrefix_SHORTER,
		}
	}

	if e = s.bgp.ListPath(context.Background(), &bgpapi.ListPathRequest{
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
		return
	}

	return
}

func (s *bgpSrv) remove(prefix *bgpapi.IPAddressPrefix, asn uint32) error {
	if prefix == nil {
		return fmt.Errorf("prefix is nil")
	}

	found, _ := s.find([]*bgpapi.IPAddressPrefix{prefix})
	s.L().Info().Msgf("Removing prefix: %s, found: %t", prefix.String(), found != nil)

	if found != nil {
		e := s.bgp.DeletePath(context.Background(), &bgpapi.DeletePathRequest{
			Path: newBgpPath(prefix, asn, s.id.String()),
		})
		return e
	}
	return fmt.Errorf("prefix %s is not found", prefix.String())
}