package cfg

import (
	"go.uber.org/zap"
	"net"
)

type appCfgTimeoutsT struct {
	DfltTtl uint32 `yaml:"DefaultTTL"`
}

func (act *appCfgTimeoutsT) DefaultTTL() uint32 {
	return act.DfltTtl
}

type addrT struct {
	Ip   net.IP `yaml:"Ip" validate:"required"`
	Port int    `yaml:"Port"`
}

type appCfgT struct {
	Lg     *zap.Config      `yaml:"Logging"`
	Touts  *appCfgTimeoutsT `yaml:"Timeouts"`
	Rslvrs []addrT          `yaml:"Resolvers"`
	Rspndr addrT            `yaml:"Responder"`
	Nms    []string         `yaml:"Names"`
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

	r := make([]*net.UDPAddr, len(ac.Rslvrs))
	for i, resolver := range ac.Rslvrs {
		r[i] = &net.UDPAddr{IP: resolver.Ip, Port: resolver.Port}
	}

	return r
}

func (ac *appCfgT) Responder() *net.UDPAddr {
	m.RLock()
	defer m.RUnlock()

	return &net.UDPAddr{IP: ac.Rspndr.Ip, Port: ac.Rspndr.Port}
}

func (ac *appCfgT) Log() *zap.Config {
	m.RLock()
	defer m.RUnlock()

	return ac.Lg
}
