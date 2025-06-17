package config

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"net"
	"os"
	"reflect"

	"github.com/spf13/viper"
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
	var cfg = &AppCfg{}

	decodeHook := func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
		if from.Kind() == reflect.String {
			if to == reflect.TypeOf(net.IP{}) {
				s := data.(string)
				s = strings.Trim(s, "\"'")
				return net.ParseIP(s), nil
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
	}

	if err = viper.Unmarshal(cfg, viper.DecodeHook(decodeHook)); err != nil {
		return nil, fmt.Errorf("error loading config file into memory, %w", err)
	}

	return cfg, nil
}
