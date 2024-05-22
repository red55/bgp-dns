package bgp

import (
	"fmt"
	bgpapi "github.com/osrg/gobgp/v3/api"
	"github.com/red55/bgp-dns/internal/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

func a2s[T proto.Message](a *anypb.Any, p T) error {
	if e := anypb.UnmarshalTo(a, p, proto.UnmarshalOptions{}); e != nil {
		log.L().Fatalf("anypb.UnmarshalTo failed")
		return e
	}
	return nil
}

func extractAttr[T proto.Message](attrs []*anypb.Any, p T) error {
	typeUrl := fmt.Sprintf("type.googleapis.com/%s", reflect.TypeOf(p).Elem().String())

	var idx = slices.IndexFunc(attrs, func(a *anypb.Any) bool {
		return a.TypeUrl == typeUrl
	})

	if idx > -1 {
		return a2s(attrs[idx], p)
	}

	return fmt.Errorf("attribute %s not found", typeUrl)
}

func path2String(p *bgpapi.Path) string {
	var s = make([]string, 0, 4)
	var ap bgpapi.IPAddressPrefix
	_ = a2s(p.Nlri, &ap)

	s = append(s, fmt.Sprintf("Prefix: %s", ap.Prefix))

	var nh bgpapi.NextHopAttribute
	if extractAttr(p.GetPattrs(), &nh) == nil {
		s = append(s, fmt.Sprintf("Nexthop: %s", nh.NextHop))
	}

	s = append(s, fmt.Sprintf("Best: %t", p.GetBest()))
	s = append(s, fmt.Sprintf("Withdraw: %t", p.GetIsWithdraw()))

	var cs bgpapi.CommunitiesAttribute
	if extractAttr(p.GetPattrs(), &cs) == nil {
		s1 := make([]string, len(cs.Communities))
		for _, c := range cs.Communities {
			as := c >> 16
			com := c & 0x00FF

			s1 = append(s1, fmt.Sprintf("%d:%d", as, com))
		}
		s = append(s, strings.Join(s1, ", "))
	}

	return fmt.Sprintf("{%s}", strings.Join(s, ", "))
}

func parseCommunity(c string) uint32 {
	parts := strings.Split(c, ":")
	if len(parts) != 2 {
		return 0
	}

	as, _ := strconv.Atoi(parts[0])
	comm, _ := strconv.Atoi(parts[1])

	return (uint32(as) << 16) | uint32(comm)
}
