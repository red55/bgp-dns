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
	opQuit
)

type dnsOp struct {
	op   op
	fqdn string
}

var _cmdChannel = make(chan *dnsOp)

func calcTTL(de *Entry) uint32 {
	var ttl = cfg.AppCfg.Timeouts().DefaultTTL()

	if de == nil {
		log.L().Infof("Refresher will sleep for %ds until %s for <empty>", ttl,
			time.Now().Add(time.Duration(ttl)*time.Second))
	} else {
		var t = de.Expire().Sub(time.Now()).Seconds()
		if t > 0 {
			ttl = uint32(t)
		} else {
			log.L().Warnf("Looks like %v already expired. Missed time %f.", de, t)
		}

		log.L().Infof("Refresher will sleep for %ds until %s for %s", ttl, de.Expire(),
			de.Fqdn())
	}

	return ttl
}

func loop(c chan *dnsOp) {
	var ttl = uint32(1) // Until cfg is read on startup sleep only 1 sec
	_wg.Add(1)
	defer _wg.Done()
	for {
		select {
		case op := <-c:
			switch op.op {
			case opAdd:
				if _, e := _Cache.add(op.fqdn); e == nil {
					log.L().Debugf("Added %s to the _Cache.", op.fqdn)
				} else {
					log.L().Debugf("Adding %s failed with %v", op.fqdn, e)
				}

			case opRemove:
				if e := _Cache.remove(op.fqdn); e != nil {
					log.L().Errorf("Remove failed: %v.", e)
				}
			case opClear:
				_Cache.clear()
			case opQuit:
				return
			}
		case <-time.After(time.Duration(ttl) * time.Second):
			if e := resolve(_Cache.getNextRefresh()); e != nil {
				log.L().Errorf("Refresh failed: %v.", e)
			}
			ttl = calcTTL(_Cache.getNextRefresh())
		}
	}
}
