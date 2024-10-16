package bgp

import (
	"context"
	"github.com/red55/bgp-dns/internal/log"
)
func (s *bgpSrv) loop(ctx context.Context) {
	s.wg.Add(1)
	defer func () {
		s.wg.Done()
	}()
L:	for {
		select {
		case o := <- s.Chan():
			s.HandleOp(o)

			log.L().Trace().Any("a", o)
			break
		case <- ctx.Done():
			break L
		}
	}
}