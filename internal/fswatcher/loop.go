package fswatcher

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/dns"
)

func (w *fsWatcher) loop(ctx context.Context) {
	var cfg = ctx.Value("cfg").(*config.AppCfg)
	w.wg.Add(1)
	defer w.wg.Done()

	for {
		select {
		case <- ctx.Done():
			w.L().Debug().Msgf("shutdown received")
			return
		case ev, ok := <- w.w.Events:
			if !ok {
				w.L().Debug().Msgf("Channel closed")
				return
			}
			w.L().Trace().Msgf("Event: %s for %s", ev.Op.String(), ev.Name )
			if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) {
				if e := dns.Load(cfg.Dns.List.File); e !=nil {
					w.L().Error().Err(e)
				}
			}
		case e, ok := <- w.w.Errors:
			if !ok {
				w.L().Debug().Msgf("Channel closed")
				return
			}
			w.L().Error().Msgf("Error: %s", e)
		}
	}
}