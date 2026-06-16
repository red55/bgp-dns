package fswatcher

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"os"
	"sync"
)

type fsWatcher struct {
	loop.Loop
	log.Log
	w *fsnotify.Watcher
	wg sync.WaitGroup
	cancel context.CancelFunc
}


var (
	_watcher *fsWatcher
)


func Serve(ctx context.Context) (e error) {
	var cfg = ctx.Value("cfg").(*config.AppCfg)

	_watcher = &fsWatcher{
		Loop:   loop.NewLoop(1),
		Log:    log.NewLog(log.L(), "fswatcher"),
		w:      nil,
		wg:     sync.WaitGroup{},
		cancel: nil,
	}
	if _watcher.w, e = fsnotify.NewWatcher(); e != nil {
		return
	}

	var inf os.FileInfo
	if inf, e = os.Stat(cfg.Dns.List.File); e != nil || inf.IsDir() {
		e = fmt.Errorf("%s is not a file", cfg.Dns.List.File)
		return
	}

	if e = _watcher.w.Add(cfg.Dns.List.File); e != nil {
		return
	}

	ctx, _watcher.cancel = context.WithCancel(ctx)
	go _watcher.loop(ctx)

	return
}

func Shutdown(ctx context.Context) (e error) {
	if _watcher.w == nil {
		return
	}

	if _watcher.cancel != nil {
		_watcher.cancel()
		_watcher.cancel = nil
	}
	e = _watcher.w.Close()
	_watcher.wg.Wait()
	_watcher.w = nil
	return
}