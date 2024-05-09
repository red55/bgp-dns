package cfg

import (
	"go.uber.org/zap"
	"net"
)

type bgpPolicyT struct {
	Comms []string `json:"Communities,omitempty"`
	Gw    net.IP   `json:"NextHops,omitempty"`
}

func (p *bgpPolicyT) NextHop() net.IP {
	m.RLock()
	defer m.RUnlock()
	return p.Gw
}

func (p *bgpPolicyT) Communities() []string {
	m.RLock()
	defer m.RUnlock()
	return p.Comms
}

type bgpNeighborT struct {
	ASN         uint32       `json:"Asn"`
	Adr         *net.TCPAddr `json:"Addr"`
	ExprtPolicy *bgpPolicyT  `json:"ExportPolicy,omitempty"`
	ImprtPolicy *bgpPolicyT  `json:"ImportPolicy,omitempty"`
}

func (n *bgpNeighborT) Asn() uint32 {
	m.RLock()
	defer m.RUnlock()
	return n.ASN
}

func (n *bgpNeighborT) Addr() *net.TCPAddr {
	m.RLock()
	defer m.RUnlock()
	return n.Adr
}

func (n *bgpNeighborT) ExportPolicy() *bgpPolicyT {
	m.RLock()
	defer m.RUnlock()
	return n.ExprtPolicy
}

func (n *bgpNeighborT) ImportPolicy() *bgpPolicyT {
	m.RLock()
	defer m.RUnlock()
	return n.ImprtPolicy
}

type bgpT struct {
	AutoSN uint32         `json:"Asn"`
	ID     string         `json:"Id"`
	Lstn   *net.TCPAddr   `json:"Listen"`
	Prs    []bgpNeighborT `json:"Peers"`
	Comms  []string       `json:"Communities,omitempty"`
}

func (b *bgpT) Asn() uint32 {
	m.RLock()
	defer m.RUnlock()
	return b.AutoSN
}

func (b *bgpT) Id() string {
	m.RLock()
	defer m.RUnlock()
	return b.ID
}

func (b *bgpT) Listen() *net.TCPAddr {
	m.RLock()
	defer m.RUnlock()
	return b.Lstn
}

func (b *bgpT) Peers() []bgpNeighborT {
	m.RLock()
	defer m.RUnlock()
	return b.Prs
}

func (b *bgpT) Communities() []string {
	m.RLock()
	defer m.RUnlock()
	return b.Comms
}

type appCfgTimeoutsT struct {
	DfltTtl uint32 `json:"DefaultTTL"`
}

func (act *appCfgTimeoutsT) DefaultTTL() uint32 {
	return act.DfltTtl
}

type kernRoutingInject struct {
	Communts []string `json:"Communities,required"`
}

func (k *kernRoutingInject) Communities() []string {
	m.RLock()
	defer m.RUnlock()
	return k.Communts
}

type kernRoutingT struct {
	Injct *kernRoutingInject `json:"Inject,omitempty"`
}

func (act *kernRoutingT) Inject() *kernRoutingInject {
	m.RLock()
	defer m.RUnlock()
	return act.Injct
}

type routingT struct {
	Bgps *bgpT         `json:"Bgp,required"`
	Krnl *kernRoutingT `json:"Kernel,omitempty"`
}

func (r *routingT) Bgp() *bgpT {
	m.RLock()
	defer m.RUnlock()
	return r.Bgps
}
func (r *routingT) Kernel() *kernRoutingT {
	m.RLock()
	defer m.RUnlock()
	return r.Krnl
}

type appCfgT struct {
	Lg        *zap.Config      `json:"Logging"`
	Rt        *routingT        `json:"Routing"`
	Touts     *appCfgTimeoutsT `json:"Timeouts"`
	Rslvrs    []*net.UDPAddr   `json:"Resolvers"`
	DefRslvrs []*net.UDPAddr   `json:"DefaultResolvers"`
	Rspndr    *net.UDPAddr     `json:"Responder"`
	Nms       []string         `json:"Names"`
}

func (ac *appCfgT) Routing() *routingT {
	m.RLock()
	defer m.RUnlock()
	return ac.Rt
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
