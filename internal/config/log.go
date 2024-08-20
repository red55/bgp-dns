package config

import (
	"github.com/rs/zerolog"
	"sync"
)

type logCfg struct {
	m  *sync.RWMutex
	Ll zerolog.Level
}

func (lCfg *logCfg) Level() zerolog.Level {
	lCfg.m.RLock()
	defer lCfg.m.RUnlock()
	return lCfg.Ll
}
