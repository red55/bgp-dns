package log

import (
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sync"
)

var (
	_logger  *zap.SugaredLogger = nil
	_wasInit bool               = false
	_m       sync.RWMutex
)

type Config struct {
	zap.Config //`mapstructure:",squash"`
}

func init() {
	_m.Lock()
	defer _m.Unlock()
	l, _ := zap.NewDevelopment()
	_logger = l.Sugar()
}

func NewDefaultConfig() *zap.Config {
	r := zap.NewDevelopmentConfig()
	r.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	return &r
}
func Init(cfg *zap.Config) error {

	var e error = nil

	_m.Lock()
	defer _m.Unlock()

	if _logger != nil && _wasInit {
		return nil
	}

	if cfg == nil {
		cfg = NewDefaultConfig()
	}

	e = fireConfigChanged(cfg, false)
	_wasInit = true

	return e
}
func FireConfigChanged(cfg *zap.Config) error {
	return fireConfigChanged(cfg, true)
}
func fireConfigChanged(cfg *zap.Config, lock bool) error {
	if lock {
		_m.Lock()
		defer func() {
			_m.Unlock()
		}()
	}
	var logger *zap.Logger
	var e error = nil

	if logger, e = cfg.Build(); e != nil {
		return e
	}

	if _, e := zap.RedirectStdLogAt(logger, zap.DebugLevel); e != nil {
		return fmt.Errorf("Unable to redirect std logger.")
	}

	zap.ReplaceGlobals(logger)
	_logger = logger.Sugar()

	return e
}

func Deinit() {
	_m.Lock()
	defer _m.Unlock()

	if _logger == nil {
		return
	}
	err := _logger.Sync()
	if err != nil {
		_logger.Debugf("Deinit and sync failed, ignoring..(%v)", err)
	}
}
func L() *zap.SugaredLogger {
	_m.RLock()
	defer _m.RUnlock()
	return _logger
}
