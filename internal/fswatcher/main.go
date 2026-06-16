package fswatcher

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/rs/zerolog"
	"os"
	"sync"
)

type fsWatcher struct {
	loop.Loop
	log.Log
	w   *fsnotify.Watcher
	wg  sync.WaitGroup
	cfg *config.AppCfg
	cancel context.CancelFunc
}


var (
	_watcher *fsWatcher
)


// Serve creates the FSWatcher service and stores it in the package-level _watcher global.
// This wrapper exists for backward compatibility with Phase 2 tests.
func Serve(ctx context.Context) (e error) {
	cfg := ctx.Value(config.ConfigKey{}).(*config.AppCfg)
	l := loop.NewLoop(1, log.L())
	s, err := NewFsWatcher(cfg, l, log.L())
	if err != nil {
		return err
	}
	_watcher = s
	return nil
}

// NewFsWatcher creates a file system watcher service with explicit dependencies.
// Returns (*fsWatcher, error) — no panics.
func NewFsWatcher(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*fsWatcher, error) {
	w := &fsWatcher{
		Loop:   l,
		Log:    log.NewLog(logger, "fswatcher"),
		cfg:    cfg,
		wg:     sync.WaitGroup{},
		cancel: nil,
	}
	var e error
	if w.w, e = fsnotify.NewWatcher(); e != nil {
		return nil, fmt.Errorf("fswatcher: create watcher failed: %w", e)
	}

	var inf os.FileInfo
	if inf, e = os.Stat(cfg.Dns.List.File); e != nil || inf.IsDir() {
		e = fmt.Errorf("%s is not a file", cfg.Dns.List.File)
		return nil, e
	}

	if e = w.w.Add(cfg.Dns.List.File); e != nil {
		return nil, fmt.Errorf("fswatcher: add watch failed: %w", e)
	}

	var loopCtx context.Context
	loopCtx, w.cancel = context.WithCancel(context.Background())
	go w.loop(loopCtx)

	return w, nil
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