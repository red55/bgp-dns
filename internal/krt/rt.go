package krt

import (
	"net"
	"slices"
)

type krtGw struct {
	g      net.IP
	weight uint32
}
type krtEntry struct {
	n  *net.IPNet
	gs []net.IP
	m  uint32
}

type kernelRoutingTable struct {
	routes map[string]*krtEntry
	//	m      sync.RWMutex
}

var routeTable kernelRoutingTable

func init() {
	//	routeTable.m.Lock()
	//	defer routeTable.m.Unlock()
	routeTable.routes = make(map[string]*krtEntry, 100)
}

func (k *kernelRoutingTable) rtFind(n *net.IPNet, g net.IP, m uint32) *krtEntry {
	key := n.String()
	//k.m.RLock()
	//defer k.m.RUnlock()
	return k.routes[key]
}

func (k *kernelRoutingTable) rtAddOrUpdate(n *net.IPNet, g net.IP, m uint32) *krtEntry {
	found := k.rtFind(n, g, m)
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

func (k *kernelRoutingTable) rtRemove(n *net.IPNet, g net.IP, m uint32) {

	found := k.rtFind(n, g, m)

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
