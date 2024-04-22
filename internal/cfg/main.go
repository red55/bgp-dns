package cfg

import (
	"github.com/red55/bgp-dns-peer/internal/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"syscall"
	"time"
)

type appCfgTimeoutsT struct {
	defaultTTL time.Duration `yaml:"dnszero"`
}

func (act *appCfgTimeoutsT) TTL() time.Duration {
	return act.defaultTTL
}

type appCfgT struct {
	timeouts  *appCfgTimeoutsT `yaml:"timeouts"`
	resolvers []string         `yaml:"resolvers"`
	Names     []string         `yaml:"names" validate:"required"`
}

func (ac *appCfgT) Timeouts() *appCfgTimeoutsT {
	return ac.timeouts
}

var AppCfg *appCfgT

func init() {
	log.Init()

	AppCfg = &appCfgT{
		resolvers: []string{"1.1.1.1:53", "8.8.8.8:53"},
		timeouts: &appCfgTimeoutsT{
			defaultTTL: 30 * time.Second,
		},
	}
	wd, _ := syscall.Getwd()
	log.L().Infof("My working directory: %s", wd)

	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	pflag.StringP("config", "c", "appsettings.yml", "Path to configuration file.")
	fn := pflag.Lookup("config")

	viper.SetConfigFile(fn.Value.String())

	if e := viper.ReadInConfig(); e != nil {
		log.L().Fatalf("error opening config file %s, %v", fn.Value.String(), e)
	}

	if e := viper.Unmarshal(&AppCfg); e != nil {
		log.L().Fatalf("error loading config file %s into memory, %v", fn.Value.String(), e)
	}
}
