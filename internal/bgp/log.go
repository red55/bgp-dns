package bgp

import (
	zaphook "github.com/Sytten/logrus-zap-hook"
	bgpapi "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/log"
	mylog "github.com/red55/bgp-dns/internal/log"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"io"
)

const _gobgpLogCallerSkip = 8

type ZapLogrusOverride struct {
	logger *logrus.Logger
}

func NewZapLogrusOverride() *ZapLogrusOverride {
	r := &ZapLogrusOverride{
		logger: logrus.New(),
	}

	r.logger.ReportCaller = true
	r.logger.SetOutput(io.Discard)

	l := mylog.L().Desugar()
	logger := l.WithOptions(zap.AddCallerSkip(_gobgpLogCallerSkip))
	h, _ := zaphook.NewZapHook(logger)

	r.logger.AddHook(h)

	return r
}

func (l *ZapLogrusOverride) Panic(msg string, fields log.Fields) {
	l.logger.WithFields(logrus.Fields(fields)).Panic(msg)
}

func (l *ZapLogrusOverride) Fatal(msg string, fields log.Fields) {
	l.logger.WithFields(logrus.Fields(fields)).Fatal(msg)
}

func (l *ZapLogrusOverride) Error(msg string, fields log.Fields) {
	l.logger.WithFields(logrus.Fields(fields)).Error(msg)
}

func (l *ZapLogrusOverride) Warn(msg string, fields log.Fields) {
	l.logger.WithFields(logrus.Fields(fields)).Warn(msg)
}

func (l *ZapLogrusOverride) Info(msg string, fields log.Fields) {
	l.logger.WithFields(logrus.Fields(fields)).Info(msg)
}

func (l *ZapLogrusOverride) Debug(msg string, fields log.Fields) {
	l.logger.WithFields(logrus.Fields(fields)).Debug(msg)
}

func (l *ZapLogrusOverride) SetLevel(level log.LogLevel) {
	l.logger.SetLevel(logrus.Level(level))
}

func (l *ZapLogrusOverride) GetLevel() log.LogLevel {
	return log.LogLevel(l.logger.Level)
}

func zapLogLevelToBgp(l zap.AtomicLevel) bgpapi.SetLogLevelRequest_Level {
	switch l.Level() {
	case zap.DebugLevel:
		return bgpapi.SetLogLevelRequest_DEBUG
	case zap.InfoLevel:
		return bgpapi.SetLogLevelRequest_INFO
	case zap.WarnLevel:
		return bgpapi.SetLogLevelRequest_WARN
	case zap.ErrorLevel:
		return bgpapi.SetLogLevelRequest_ERROR
	case zap.DPanicLevel:
		return bgpapi.SetLogLevelRequest_PANIC
	case zap.PanicLevel:
		return bgpapi.SetLogLevelRequest_PANIC
	case zap.FatalLevel:
		return bgpapi.SetLogLevelRequest_FATAL
	default:
		panic("unhandled default case")
	}
}

/*
func zapLogLevelToLogrus(l zapcore.Level) logrus.Level {
	switch l {
	case zapcore.DebugLevel:
		return logrus.DebugLevel
	case zapcore.InfoLevel:
		return logrus.InfoLevel
	case zapcore.WarnLevel:
		return logrus.WarnLevel
	case zapcore.ErrorLevel:
		return logrus.ErrorLevel
	case zapcore.DPanicLevel:
		return logrus.PanicLevel
	case zapcore.PanicLevel:
		return logrus.PanicLevel
	case zapcore.FatalLevel:
		return logrus.FatalLevel
	default:
		panic("unhandled default case")
	}
}
*/
