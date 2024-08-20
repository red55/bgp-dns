package config

import (
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"log"
	"net"
	"os"
	"reflect"
)

func Init(path string) {
	viper.SetConfigName("appsettings")
	viper.SetConfigType("yaml")

	_, err := os.Stat(path)
	if len(path) > 0 && err == nil {
		viper.AddConfigPath(path)
	} else {
		viper.AddConfigPath(".")
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Unable to read application configuration: %v", err)
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
		log.Fatalf("error loading config file into memory, %v", e)
	}
}
