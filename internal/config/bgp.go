package config

import (
	"net"
)

type bgpNeighbor struct {
	Asn    uint32         	`yaml:"Asn" json:"Asn"`
	Addr 	 net.TCPAddr 	`yaml:"Address" json:"Address"`
	Multihop bool			`yaml:"Multihop" json:"Multihop"`
	PassiveMode bool 		`yaml:"PassiveMode" json:"PassiveMode"`
}

type bgpCfg struct {
	Asn    uint32         	`yaml:"Asn" json:"Asn"`
	Id     	net.IP         	`yaml:"Id" json:"Id"`
	Listen   net.TCPAddr    `yaml:"Listen" json:"Listen"`
	Peers []*bgpNeighbor 	`yaml:"Peers" json:"Peers"`
}
