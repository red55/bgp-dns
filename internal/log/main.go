package log

import (
	"fmt"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/rs/zerolog"
	"os"
	"sync/atomic"
	"time"
)

type Log struct {
	l atomic.Pointer[zerolog.Logger]
	lvl atomic.Value
}

func (l *Log) setLogger(log *zerolog.Logger) {
	l.l.Store(log)
}

func (l *Log) L() *zerolog.Logger {
	r := l.l.Load()
	return r;
}

func (l *Log) SetLevel(lvl zerolog.Level) {
	ll := l.L().Level(lvl)
	l.l.Store(&ll)
	l.lvl.Store(lvl)
}
func (l *Log) Level() zerolog.Level {
	return l.lvl.Load().(zerolog.Level)
}
var (
	_logger Log
)

func NewLog(l *zerolog.Logger, moduleName string) (r Log) {
	if moduleName == "" {
		moduleName = "main"
	}

	mn := fmt.Sprintf("%-10s", moduleName)
	nl := l.With().Str("m", mn).Logger()
	r.setLogger(&nl)

	return
}
func Init(cfg *config.AppCfg) {
	l := zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stdout,
		TimeFormat: time.RFC3339,
		PartsOrder: []string{
			zerolog.TimestampFieldName,
			"m",
			zerolog.LevelFieldName,
			zerolog.CallerFieldName,
			zerolog.MessageFieldName,
		},
		FieldsExclude: []string {
			"m",
		},
	}).Level(cfg.Log.Level).With().Timestamp().Logger()
	_logger.setLogger(&l)
}
func L()* zerolog.Logger {
	return _logger.L()
}

