package bgp

import (
    bgplog "github.com/osrg/gobgp/v3/pkg/log"
    "github.com/rs/zerolog"
	"github.com/red55/bgp-dns/internal/log"
)

type zeroLogger struct {
	lg zerolog.Logger
	l zerolog.Level
}


func newZeroLogger(l zerolog.Level) bgplog.Logger {
	r := &zeroLogger{
		lg: log.L().With().Str("m", "bgp").Logger(),
		l: l,
	}

	r.SetLevel(zeroLogLevel2bgpLogLevel(l))

	return r
}
func withFields(e *zerolog.Event, fields bgplog.Fields) *zerolog.Event {
	for k,v := range fields{
		e = e.Any(k,v)
	}

	return e
}
func (h *zeroLogger) Panic(msg string, fields bgplog.Fields) {
	withFields(log.L().Panic(), fields).Msg(msg)
}
func (h *zeroLogger) Fatal(msg string, fields bgplog.Fields) {
	withFields(log.L().Fatal(), fields).Msg(msg)
}
func (h *zeroLogger) Error(msg string, fields bgplog.Fields) {
	withFields(log.L().Error(), fields).Msg(msg)
}
func (h *zeroLogger) Warn(msg string, fields bgplog.Fields) {
	withFields(log.L().Warn(), fields).Msg(msg)
}
func (h *zeroLogger) Info(msg string, fields bgplog.Fields) {
	withFields(log.L().Info(), fields).Msg(msg)
}
func (h *zeroLogger) Debug(msg string, fields bgplog.Fields) {
	withFields(log.L().Debug(), fields).Msg(msg)
}
func (h *zeroLogger) SetLevel(level bgplog.LogLevel) {
	h.l = bgpLogLevel2ZeroLogLevel(level)
	h.lg = h.lg.Level(h.l)
}

func (h *zeroLogger)  GetLevel() bgplog.LogLevel {
	return zeroLogLevel2bgpLogLevel(log.L().GetLevel())
}

func zeroLogLevel2bgpLogLevel(l zerolog.Level) bgplog.LogLevel {
    switch l {
	case zerolog.DebugLevel:
		return bgplog.DebugLevel
	case zerolog.InfoLevel:
		return bgplog.InfoLevel
	case zerolog.WarnLevel:
		return bgplog.WarnLevel
	case zerolog.ErrorLevel:
		return bgplog.ErrorLevel
	case zerolog.FatalLevel:
		return bgplog.FatalLevel
	case zerolog.PanicLevel:
		return bgplog.PanicLevel
    default:
		return bgplog.TraceLevel
    }
}
func bgpLogLevel2ZeroLogLevel(l bgplog.LogLevel) zerolog.Level {
    switch l {
	case bgplog.PanicLevel:
		return zerolog.PanicLevel
	case bgplog.FatalLevel:
		return zerolog.FatalLevel
	case bgplog.ErrorLevel:
		return zerolog.ErrorLevel
	case bgplog.WarnLevel:
		return zerolog.WarnLevel
	case bgplog.InfoLevel:
		return zerolog.InfoLevel
	case bgplog.DebugLevel:
		return zerolog.DebugLevel
    default:
		return zerolog.TraceLevel
    }
}