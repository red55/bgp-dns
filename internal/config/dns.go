package config

import (
	"net"
	"time"
)

type listCfg struct {
	File      string         `yaml:"File" json:"File"`
	Resolvers []*net.UDPAddr `yaml:"Resolvers" json:"Resolvers"`
}
type cacheCfg struct {
	MaxEntries int `yaml:"MaxEntries" json:"MaxEntries"`
	MinTtl	  time.Duration `yaml:"MinTtl" json:"MinTtl"`
}

type dnsCfg struct {
	Listen    *net.UDPAddr   `yaml:"Listen" json:"Listen"`
	Resolvers []*net.UDPAddr `yaml:"Resolvers" json:"Resolvers"`
	List      listCfg        `yaml:"List" json:"List"`
	Cache	  cacheCfg		 `yaml:"Cache" json:"Cache"`
}

