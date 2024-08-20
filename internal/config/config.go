package config

import (
	"sync"
)

type appCfg struct {
	m sync.RWMutex

	L logCfg `yaml:"Log"`
}

func (cfg *appCfg) Log() logCfg {
	cfg.m.RLock()
	defer cfg.m.RUnlock()
	cfg.L.m = &cfg.m
	return cfg.L
}

var AppCfg *appCfg = &appCfg{}
