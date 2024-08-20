package config

import "net"

type bgpNei struct {
}
type bgpCfg struct {
	Asn         uint32
	Id          net.IP
	Lstn        net.TCPAddr
	Communities []string
	Prs
}
