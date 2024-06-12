package cfg

import (
	"fmt"
	"net"
	"reflect"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var AppCfg *appCfgT
var m sync.RWMutex

type ConfigChangedHandler func()

type ConfigChangeHandlerRegistry struct {
	onChange []ConfigChangedHandler
	m        sync.RWMutex
}

var _changeHandlers = ConfigChangeHandlerRegistry{
	onChange: make([]ConfigChangedHandler, 0, 3),
	m:        sync.RWMutex{},
}

const (
	defaultTtl     = 30
	ttlForZero     = 30
	ttl4ZeroJitter = 10
)

func init() {
	AppCfg = &appCfgT{
		Rslvrs: []*net.UDPAddr{
			{IP: net.ParseIP("8.8.8.8"), Port: 53},
		},
		Touts: &appCfgTimeoutsT{
			DfltTtl:        defaultTtl,
			TtlforZero:     ttlForZero,
			Ttl4ZeroJitter: ttl4ZeroJitter,
		},
		Lg: log.NewDefaultConfig(),
	}

	viper.SetConfigType("yaml")
	viper.AutomaticEnv()
}

func Init(configFile string) {
	viper.SetConfigFile(configFile)

	viper.OnConfigChange(func(in fsnotify.Event) {
		reloadConfig()
	})
	// viper.WatchConfig()
	reloadConfig()
}

func RegisterConfigChangeHandler(handler ConfigChangedHandler) error {
	p := reflect.ValueOf(handler).Pointer()
	var found bool = false

	_changeHandlers.m.Lock()
	defer _changeHandlers.m.Unlock()

	for _, v := range _changeHandlers.onChange {
		if reflect.ValueOf(v).Pointer() == p {
			found = true
		}
	}

	if found {
		return fmt.Errorf("config change handler already registered")
	}

	_changeHandlers.onChange = append(_changeHandlers.onChange, handler)

	return nil
}

func Deinit() {
	log.L().Debugf("cfg.Deinit called")
	m.Lock()
	defer m.Unlock()
	_changeHandlers.onChange = []ConfigChangedHandler{}
}

func readConfig() {
	m.Lock()
	defer m.Unlock()

	if e := viper.ReadInConfig(); e != nil {
		log.L().Fatalf("error opening config file %v", e)
	}

	if e := viper.Unmarshal(&AppCfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "json"
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

	_ = log.FireConfigChanged(AppCfg.Log())

	m.RLock()
	defer m.RUnlock()

	if len(_changeHandlers.onChange) > 0 {
		for _, handler := range _changeHandlers.onChange {
			handler()
		}
	}
}

func reloadConfig() {

	readConfig()

	fireOnChange()
}
