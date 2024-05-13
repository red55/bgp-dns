package bgp

import (
	"fmt"
	"github.com/red55/bgp-dns/internal/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"reflect"
	"slices"
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
