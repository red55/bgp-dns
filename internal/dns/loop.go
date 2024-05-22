package dns

import (
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/log"
	"time"
)

type op int

const (
	opAdd op = iota
	opRemove
	opClear
	opLoad
	opQuit
)

type dnsOpI interface {
	Op() op
}

type dnsOp struct {
	op op
}

func (o dnsOp) Op() op {
	return o.op
}

type dnsOpFqdnI interface {
	dnsOpI
	Fqdn() string
}
type dnsOpFqdn struct {
	dnsOp
	fqdn string
}

func (o dnsOpFqdn) Fqdn() string {
	return o.fqdn
}

func (o dnsOpFqdn) Op() op {
	return o.op
}

type dnsOpLoadI interface {
	Files() []string
	Additive() bool
}

type dsnOpLoad struct {
	dnsOp
	files    []string
	additive bool
}

func (o dsnOpLoad) Files() []string {
	return o.files
}

func (o dsnOpLoad) Additive() bool {
	return o.additive
}

var _cmdChannel = make(chan dnsOpI, 3)

func calcTTL(de *Entry) uint32 {
	var ttl = cfg.AppCfg.Timeouts().DefaultTTL()

	if de == nil {
		log.L().Infof("Refresher will sleep for %ds until %s for <empty>", ttl,
			time.Now().Add(time.Duration(ttl)*time.Second))
	} else {
		for {
			t := de.Expire().Sub(time.Now()).Seconds()
			if t > 0 {
				ttl = uint32(t)
				break
			} else {
				log.L().Warnf("Looks like %v already expired. Missed time %f.", de.Fqdn(), t)
				de.SetTtl(ttl)
				_cache.updateNextRefresh(true)
				de = _cache.getNextRefresh()
			}
		}
		log.L().Infof("Refresher will sleep for %ds until %s for %s", ttl, de.Expire(),
			de.Fqdn())
	}

	return ttl
}

func loop(c chan dnsOpI) {
	var ttl = uint32(1) // Until cfg is read on startup sleep only 1 sec
	_wg.Add(1)
	defer _wg.Done()
	for {
		select {
		case operation := <-c:
			switch operation.Op() {
			case opAdd:
				o, _ := operation.(dnsOpFqdnI)
				if _, e := _cache.add(o.Fqdn()); e == nil {
					log.L().Debugf("Added %s to the cache.", o.Fqdn())
				} else {
					log.L().Debugf("Adding %s failed with %v", o.Fqdn(), e)
				}
			case opRemove:
				o, _ := operation.(*dnsOpFqdn)
				if e := _cache.remove(o.fqdn); e != nil {
					log.L().Errorf("Remove failed: %v.", e)
				}
			case opLoad:
				if l, ok := operation.(*dsnOpLoad); ok {
					_cache.load(l.Files())
				}
			case opClear:
				_cache.clear()
			case opQuit:
				return
			}
		case <-time.After(time.Duration(ttl) * time.Second):
			// resolve can change next entry for refresh
			if e := resolve(_cache.getNextRefresh()); e != nil {
				log.L().Errorf("Refresh failed: %v.", e)
			}
			ttl = calcTTL(_cache.getNextRefresh())
		}
	}
}
