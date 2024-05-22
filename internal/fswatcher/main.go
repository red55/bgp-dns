package fswatcher

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/log"
	"os"
	"reflect"
	"slices"
	"sync"
)

type onFsChangeT func(e fsnotify.Event)

type watcherT struct {
	m        sync.RWMutex
	w        *fsnotify.Watcher
	watchers []onFsChangeT
}

var (
	_watcher watcherT
	_wg      sync.WaitGroup
)

func Init() {
	initWatcher()
}

func initWatcher() {

	if _watcher.w != nil {
		StopWatcher()
	}

	_watcher.m.Lock()
	defer _watcher.m.Unlock()

	var err error
	_watcher.w, err = fsnotify.NewWatcher()
	if err != nil {
		log.L().Fatalf("fsnotify.NewWatcher failed: %v", err)
	}

	if inf, err := os.Stat(cfg.AppCfg.DnsListsFolder); err != nil || !inf.IsDir() {
		log.L().Fatalf("%s is not a directory", cfg.AppCfg.DnsListsFolder)
	}

	if err = _watcher.w.Add(cfg.AppCfg.DnsListsFolder); err != nil {
		log.L().Fatalf("watcher.Add failed: %v", err)
	}

	go func(watcher *watcherT) {
		_wg.Add(1)
		defer _wg.Done()
		for {
			select {
			case event, ok := <-watcher.w.Events:
				log.L().Debugf("[FSWatcher] event: %v", event)
				if !ok {
					log.L().Errorf("watcher.Events channel closed")
					return
				}
				watcher.fireWatchers(event)
			case err, ok := <-watcher.w.Errors:
				if !ok {
					return
				}
				log.L().Errorf("watcher.Errors: %v", err)
			}
		}
	}(&_watcher)

}

func (w *watcherT) fireWatchers(event fsnotify.Event) {
	w.m.RLock()
	defer w.m.RUnlock()
	for _, cb := range w.watchers {
		cb(event)
	}
}
func StopWatcher() {
	_watcher.m.Lock()
	defer _watcher.m.Unlock()

	if _watcher.w == nil {
		return
	}

	if e := _watcher.w.Close(); e != nil {
		log.L().Errorf("_watcher.w.Close failed: %v", e)
		return
	}
	_wg.Wait()
	_watcher.w = nil
}
func TriggerCreate(path string) {
	_watcher.m.Lock()
	w := _watcher.w
	_watcher.m.Unlock()
	if w == nil {
		return
	}

	_watcher.w.Events <- fsnotify.Event{
		Name: path,
		Op:   fsnotify.Create,
	}
}
func AddWatcher(f onFsChangeT) error {
	_watcher.m.RLock()
	b := _watcher.w == nil
	_watcher.m.RUnlock()

	if b {
		initWatcher()
	}

	_watcher.m.Lock()
	defer _watcher.m.Unlock()

	p := reflect.ValueOf(f).Pointer()
	found := slices.ContainsFunc(_watcher.watchers, func(callback onFsChangeT) bool {
		return reflect.ValueOf(callback).Pointer() == p
	})

	if found {
		return fmt.Errorf("FSWatcher callback already registered")
	}

	_watcher.watchers = append(_watcher.watchers, f)

	return nil
}

func Deinit() {
	log.L().Infof("FSWatcher->Deinit(): Enter")
	defer log.L().Infof("FSWatcher->Deinit(): Done")

	StopWatcher()
}
