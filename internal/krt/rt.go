package krt

import (
	"net"
	"slices"
	"sync"
)

type krtEntry struct {
	n *net.IPNet
	g net.IP
	m uint16
}

type kernelRoutingTable struct {
	routes []*krtEntry
	m      sync.RWMutex
}

var routeTable kernelRoutingTable

func init() {
	routeTable.m.Lock()
	defer routeTable.m.Unlock()
	routeTable.routes = make([]*krtEntry, 0, 100)
}

func cmpIPNet(a, b *net.IPNet) bool {
	if a == nil || b == nil {
		return false
	}
	if !a.IP.Equal(b.IP) {
		return false
	}
	if len(a.Mask) != len(b.Mask) {
		return false
	}
	for i, v := range a.Mask {
		if v != b.Mask[i] {
			return false
		}
	}
	return true
}

func (e *krtEntry) Equal(o *krtEntry) bool {
	return cmpIPNet(e.n, o.n) && e.m == o.m && e.g.Equal(o.g)
}
func (k *kernelRoutingTable) rtFind(n *net.IPNet, g net.IP, m uint16) (int, *krtEntry) {
	ke := &krtEntry{n: n, g: g, m: m} // TODO: remove extra allocation. Don't care for now.
	k.m.RLock()
	defer k.m.RUnlock()
	found := slices.IndexFunc(k.routes, func(a *krtEntry) bool {
		return a.Equal(ke)
	})

	if found == -1 {
		return found, nil
	}
	return found, k.routes[found]
}

func (k *kernelRoutingTable) rtAdd(n *net.IPNet, g net.IP, m uint16) error {
	_, found := k.rtFind(n, g, m)
	if found == nil {
		k.m.Lock()
		k.routes = append(k.routes, &krtEntry{n, g, m})
		k.m.Unlock()
	}
	return nil
}

func (k *kernelRoutingTable) rtRemove(n *net.IPNet, g net.IP, m uint16) error {

	idx, found := k.rtFind(n, g, m)

	if found != nil {
		k.m.Lock()
		l := len(k.routes) - 1
		k.routes[idx] = k.routes[l]
		k.routes = k.routes[:l]
		k.m.Unlock()
	}
	return nil
}
