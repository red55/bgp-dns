package log

import (
	"github.com/red55/bgp-dns/internal/config"
	"github.com/rs/zerolog"
	"os"
	"time"
)
var (
	_logger zerolog.Logger
)
func Init(cfg *config.AppCfg) {
	_logger = zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stdout,
		TimeFormat: time.RFC3339,
	}).Level(cfg.Log.Level).With().Timestamp().Logger()
}
func L()* zerolog.Logger {
	return &_logger
}
