package bgp

import (
	zaphook "github.com/Sytten/logrus-zap-hook"
	"github.com/osrg/gobgp/v3/pkg/log"
	mylog "github.com/red55/bgp-dns/internal/log"
	"github.com/sirupsen/logrus"
	"io"
)

type ZapLogrusOverride struct {
	logger *logrus.Logger
}

func NewZapLogrusOverride() *ZapLogrusOverride {
	r := &ZapLogrusOverride{
		logger: logrus.New(),
	}
	r.logger.ReportCaller = true
	r.logger.SetOutput(io.Discard)
	h, _ := zaphook.NewZapHook(mylog.L().Desugar())
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
	return log.LogLevel(6)
}

/*
switch cfg.AppCfg.Log().Level.Level() {
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
*/
