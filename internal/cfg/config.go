package cfg

import (
	"go.uber.org/zap"
	"net"
)

type bgpPolicyT struct {
	Communities []string `json:"Communities,omitempty"`
	NextHop     net.IP   `json:"NextHops,omitempty"`
}
type bgpNeighborT struct {
	Asn          uint32       `json:"Asn"`
	Addr         *net.TCPAddr `json:"Addr"`
	ExportPolicy *bgpPolicyT  `json:"ExportPolicy,omitempty"`
	ImportPolicy *bgpPolicyT  `json:"ImportPolicy,omitempty"`
}
type bgpT struct {
	Asn         uint32         `json:"Asn"`
	Id          string         `json:"Id"`
	Listen      *net.TCPAddr   `json:"Listen"`
	Peers       []bgpNeighborT `json:"Peers"`
	Communities []string       `json:"Communities,omitempty"`
}
type appCfgTimeoutsT struct {
	DfltTtl uint32 `json:"DefaultTTL"`
}

func (act *appCfgTimeoutsT) DefaultTTL() uint32 {
	return act.DfltTtl
}

type appCfgT struct {
	Lg        *zap.Config      `json:"Logging"`
	Bgp       *bgpT            `json:"Bgp"`
	Touts     *appCfgTimeoutsT `json:"Timeouts"`
	Rslvrs    []*net.UDPAddr   `json:"Resolvers"`
	DefRslvrs []*net.UDPAddr   `json:"DefaultResolvers"`
	Rspndr    *net.UDPAddr     `json:"Responder"`
	Nms       []string         `json:"Names"`
}

func (ac *appCfgT) Timeouts() *appCfgTimeoutsT {
	m.RLock()
	defer m.RUnlock()

	return ac.Touts
}

func (ac *appCfgT) Names() []string {
	m.RLock()
	defer m.RUnlock()

	r := make([]string, len(ac.Nms))
	copy(r, ac.Nms)

	return r
}

func (ac *appCfgT) Resolvers() []*net.UDPAddr {
	m.RLock()
	defer m.RUnlock()

	return ac.Rslvrs
}

func (ac *appCfgT) DefaultResolvers() []*net.UDPAddr {
	m.RLock()
	defer m.RUnlock()

	return ac.DefRslvrs
}

func (ac *appCfgT) Responder() *net.UDPAddr {
	m.RLock()
	defer m.RUnlock()

	return &net.UDPAddr{IP: ac.Rspndr.IP, Port: ac.Rspndr.Port}
}

func (ac *appCfgT) Log() *zap.Config {
	m.RLock()
	defer m.RUnlock()

	return ac.Lg
}
