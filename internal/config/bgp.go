package config

import (
	"net"
)

type bgpNeighbor struct {
	AddComms []string 		`yaml:"Communities" json:"Communities"`
	Addr 	 net.TCPAddr 	`yaml:"Address" json:"Address"`
}

type bgpCfg struct {
	Asn    uint32         	`yaml:"Asn" json:"Asn"`
	Id     net.IP         	`yaml:"Id" json:"Id"`
	Listen   net.TCPAddr    `yaml:"Listen" json:"Listen"`
	Peers []*bgpNeighbor 	`yaml:"Peers" json:"Peers"`
}
