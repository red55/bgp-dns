package dns

import (
	"errors"
	"context"
	"github.com/red55/bgp-dns/internal/log"
)

func (c *cache) loop(ctx context.Context) {
	_wg.Add(1)
L:
	for {
		select {
		case <- ctx.Done():
			if !errors.Is(ctx.Err(), context.Canceled) {
				log.L().Error().Err(ctx.Err())
			}
			break L
		}
	}
	_wg.Done()
}