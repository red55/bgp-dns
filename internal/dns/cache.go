package dns

import (
	"fmt"
	"slices"
	"sync"
)

type Cache struct {
	m            sync.RWMutex
	entries      []*Entry
	next2Refresh *Entry
}

var chanResolver chan string
var chanRefresher chan struct{}

func (c *Cache) String() string {
	c.m.RLock()
	defer c.m.RUnlock()
	var ret string
	for _, e := range c.entries {
		ret += fmt.Sprintf("\t%s\n", e)
	}

	return ret
}

var cache Cache

func (c *Cache) findLeastTTLCacheEntry() *Entry {
	c.m.RLock()
	defer c.m.RUnlock()

	if len(c.entries) == 0 {
		return nil
	} else {
		return slices.MinFunc(c.entries, func(a, b *Entry) int {
			if a.Expire().Before(b.Expire()) {
				return -1
			} else if a.Expire().Equal(b.Expire()) {
				return 0
			} else {
				return 1
			}
		})
	}

}

func (c *Cache) findCacheEntry(dnsName string) *Entry {
	c.m.RLock()
	defer c.m.RUnlock()
	foundIdx := slices.IndexFunc(c.entries, func(de *Entry) bool { return de.Fqdn() == dnsName })
	if foundIdx > -1 {
		return c.entries[foundIdx]
	} else {
		return nil
	}
}
func (c *Cache) addCacheEntry(dnsName string) *Entry {
	var de = NewEntry(dnsName)

	c.m.Lock()
	defer c.m.Unlock()

	c.entries = append(c.entries, de)
	if len(c.entries) == 1 {
		c.next2Refresh = c.entries[0]
	}

	return de
}

func (c *Cache) setNextRefreshEntry(next *Entry) {
	c.m.Lock()
	c.next2Refresh = next
	defer c.m.Unlock()

}
func (c *Cache) lookupOrResolve(dnsName string) (*Entry, error) {
	var de = c.findCacheEntry(dnsName)
	var e error = nil
	if de == nil {
		de = c.addCacheEntry(dnsName)
		e = resolve(de)
	}
	return de, e
}
func (c *Cache) clear() {
	c.m.Lock()
	defer c.m.Unlock()

	c.entries = make([]*Entry, 0, len(c.entries))
}
