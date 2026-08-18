package config

import "time"

// TestConfig returns a minimal config suitable for tests.
// This avoids needing to construct unexported config types from other packages.
func TestConfig() *AppCfg {
	return &AppCfg{
		Dns: dnsCfg{
			Timeout: 5 * time.Second,
			Cache: cacheCfg{
				MinTtl: 60 * time.Second,
			},
		},
	}
}
