package krt

import (
	"net"
	"slices"
)

type krtEntry struct {
	n  *net.IPNet
	gs []net.IP
	m  uint32
}

type kernelRoutingTable struct {
	routes map[string]*krtEntry
	//	m      sync.RWMutex
}

var _routeTable kernelRoutingTable

func init() {
	//	_routeTable.m.Lock()
	//	defer _routeTable.m.Unlock()
	_routeTable.routes = make(map[string]*krtEntry, 100)
}

func (k *kernelRoutingTable) rtFind(n *net.IPNet) *krtEntry {
	key := n.String()
	//k.m.RLock()
	//defer k.m.RUnlock()
	return k.routes[key]
}

func (k *kernelRoutingTable) rtAddOrUpdate(n *net.IPNet, g net.IP, m uint32) *krtEntry {
	found := k.rtFind(n)
	if found == nil {
		//		k.m.Lock()
		key := n.String()
		found = &krtEntry{
			n:  n,
			gs: []net.IP{g},
			m:  m,
		}
		k.routes[key] = found
		//		k.m.Unlock()
	} else {
		idx := slices.IndexFunc(found.gs, func(ip net.IP) bool {
			return ip.Equal(g)
		})
		if idx == -1 {
			found.gs = append(found.gs, g)
		}
	}
	return found
}

func (k *kernelRoutingTable) rtRemove(n *net.IPNet, g net.IP) {

	found := k.rtFind(n)

	if found != nil {
		key := n.String()
		//		k.m.Lock()
		found.gs = slices.DeleteFunc(found.gs, func(ip net.IP) bool {
			return ip.Equal(g)
		})

		if len(found.gs) == 0 {
			delete(k.routes, key)
		}
		//		k.m.Unlock()
	}
}
