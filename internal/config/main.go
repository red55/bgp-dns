package config

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"

	//"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"net"
	"os"
	"reflect"
)

func Init(path string) (*AppCfg, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	_, err := os.Stat(path)
	if len(path) > 0 && err == nil {
		viper.AddConfigPath(path)
	} else {
		viper.AddConfigPath(".")
	}

	if err = viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("unable to read application configuration: %w", err)
	}
	var cfg = &AppCfg {}

	if err = viper.Unmarshal(cfg, func(config *mapstructure.DecoderConfig) {
		config.TagName = "json"
		config.DecodeHook = mapstructure.ComposeDecodeHookFunc(func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {

			if from.Kind() == reflect.String {
				if to == reflect.TypeOf(net.IP{}) {
					return net.ParseIP(data.(string)), nil
				}

				if to == reflect.TypeOf(zerolog.DebugLevel) {
					if l, e := zerolog.ParseLevel(data.(string)); e == nil {
						return l, nil
					} else {
						return nil, e
					}
				}
			}

			return data, nil
		})
	}); err != nil {
		return nil, fmt.Errorf("error loading config file into memory, %w", err)
	}

	return cfg, nil
}
