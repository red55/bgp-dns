package log

import (
	"go.uber.org/zap"
	"log"
)

var (
	_logger *zap.SugaredLogger = nil
)

func Init() {

	if _logger != nil {
		return
	}

	logger, _ := zap.NewDevelopment()

	if _, err := zap.RedirectStdLogAt(logger, zap.InfoLevel); err != nil {
		log.Fatal("Unable to redirect std logger.")
	}

	zap.ReplaceGlobals(logger)
	_logger = logger.Sugar()
}

func Deinit() {
	if _logger == nil {
		return
	}
	err := _logger.Sync()
	if err != nil {
		_logger.Debugf("Deinit and sync failed, ignoring..(%v)", err)
	}
}
func L() *zap.SugaredLogger {
	return _logger
}
