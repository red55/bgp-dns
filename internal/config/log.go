package config

import (
	"github.com/rs/zerolog"
)

type logCfg struct {
	Level zerolog.Level `yaml:"Level" json:"Level"`
}
