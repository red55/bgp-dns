package dns

import (
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/log"
	"time"
)

type operation int

const (
	opAdd operation = iota
	opRemove
	opClear
	opQuit
)

type msg struct {
	op   operation
	fqdn string
}

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

func refresher(c chan *msg) {
	var ttl = uint32(1) // Until cfg is read on startup sleep only 1 sec

	for {
		select {
		case op := <-c:
			switch op.op {
			case opAdd:
				if _, e := cache.add(op.fqdn); e == nil {
					log.L().Debugf("Added %s to the cache.", op.fqdn)
				} else {
					log.L().Debugf("Adding %s failed with %v", op.fqdn, e)
				}

			case opRemove:
				if e := cache.remove(op.fqdn); e != nil {
					log.L().Errorf("Remove failed: %v.", e)
				}
			case opClear:
				cache.clear()
			case opQuit:
				return
			}
		case <-time.After(time.Duration(ttl) * time.Second):

			if e := resolve(cache.getNextRefresh()); e != nil {
				log.L().Errorf("Refresh failed: %v.", e)
			}
			ttl = calcTTL(cache.getNextRefresh())
		}
	}
}
