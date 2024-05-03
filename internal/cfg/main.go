package cfg

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/red55/bgp-dns-peer/internal/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"net"
	"reflect"
	"sync"
)

var AppCfg *appCfgT
var m sync.RWMutex

type ConfigChangedHandler func()

type ConfigChangeHandlerRegistry struct {
	onChange []ConfigChangedHandler
	m        sync.RWMutex
}

var changeHandlers = ConfigChangeHandlerRegistry{
	onChange: make([]ConfigChangedHandler, 0, 3),
	m:        sync.RWMutex{},
}

func init() {
	AppCfg = &appCfgT{
		Rslvrs: []addrT{
			{Ip: net.ParseIP("8.8.8.8"), Port: 53},
		},
		Touts: &appCfgTimeoutsT{
			DfltTtl: 30,
		},
		Lg: log.NewDefaultConfig(),
	}

	viper.SetConfigType("yaml")
	viper.AutomaticEnv()
}

func Init() {

	pflag.StringP("config", "c", "appsettings.yml", "Path to configuration file.")
	fn := pflag.Lookup("config")

	viper.SetConfigFile(fn.Value.String())
	viper.OnConfigChange(func(in fsnotify.Event) {
		reloadConfig()
	})
	viper.WatchConfig()
	reloadConfig()
}

func RegisterConfigChangeHandler(handler ConfigChangedHandler) error {
	p := reflect.ValueOf(handler).Pointer()
	var found bool = false

	changeHandlers.m.Lock()
	defer changeHandlers.m.Unlock()

	for _, v := range changeHandlers.onChange {
		if reflect.ValueOf(v).Pointer() == p {
			found = true
		}
	}

	if found {
		return fmt.Errorf("config change handler already registered")
	}

	changeHandlers.onChange = append(changeHandlers.onChange, handler)

	return nil
}

func Deinit() {
	log.L().Debugf("cfg.Deinit called")
	m.Lock()
	defer m.Unlock()
	changeHandlers.onChange = []ConfigChangedHandler{}
}

func readConfig() {
	m.Lock()
	defer m.Unlock()

	if e := viper.ReadInConfig(); e != nil {
		log.L().Fatalf("error opening config file %v", e)
	}

	if e := viper.Unmarshal(&AppCfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "yaml"
		config.DecodeHook = mapstructure.ComposeDecodeHookFunc(func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {

			if from.Kind() == reflect.String {
				if to == reflect.TypeOf(net.IP{}) {
					return net.ParseIP(data.(string)), nil
				}

				if to == reflect.TypeOf(zap.AtomicLevel{}) {
					if l, e := zap.ParseAtomicLevel(data.(string)); e == nil {
						return l, nil
					} else {
						return nil, e
					}
				}
			}

			return data, nil
		})
	}); e != nil {
		log.L().Fatalf("error loading config file into memory, %v", e)
	}
}

func fireOnChange() {
	m.RLock()
	defer m.RUnlock()

	if len(changeHandlers.onChange) > 0 {
		for _, handler := range changeHandlers.onChange {
			handler()
		}
	}
}

func reloadConfig() {

	readConfig()

	fireOnChange()
}
