package bgp

import (
	"context"
)
func (s *bgpSrv) loop(ctx context.Context) {
	s.wg.Add(1)
	defer func () {
		s.wg.Done()
	}()
L:	for {
		select {
		case o := <- s.ChanOp():
			s.HandleOp(o)
			break
		case <- ctx.Done():
			break L
		}
	}
}