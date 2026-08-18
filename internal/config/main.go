package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"net"
	"os"
	"reflect"
	"time"

	"github.com/spf13/viper"
)

// ConfigKey is the typed context key for config values.
// Being a struct type prevents external packages from creating colliding keys.
type ConfigKey struct{}

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

			if to == reflect.TypeOf(time.Duration(0)) {
				d, e := time.ParseDuration(data.(string))
				if e != nil {
					return nil, e
				}
				return d, nil
			}
		}

		return data, nil
	}

	if err = viper.Unmarshal(cfg, viper.DecodeHook(decodeHook)); err != nil {
		return nil, fmt.Errorf("error loading config file into memory, %w", err)
	}

	switch {
	case cfg.Dns.Timeout <= 0:
		// Absent or misparsed (e.g. bare YAML integer decoded to nanoseconds):
		// fall back to the documented default instead of rejecting.
		cfg.Dns.Timeout = 5 * time.Second
	case cfg.Dns.Timeout < 3*time.Second || cfg.Dns.Timeout > 30*time.Second:
		return nil, fmt.Errorf("dns: timeout %v out of range (allowed: 3s-30s)", cfg.Dns.Timeout)
	}

	var listFile string
	listFile, _ = filepath.Abs(cfg.Dns.List.File)
	cfg.Dns.List.File = strings.ReplaceAll(listFile, "\\", string(os.PathSeparator))

	return cfg, nil
}
