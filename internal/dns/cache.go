package dns

import (
	"fmt"
	"github.com/red55/bgp-dns/internal/log"
	"slices"
	"sync"
)

type Cache struct {
	m           sync.RWMutex
	entries     []*Entry
	nextRefresh *Entry
}

func (c *Cache) String() string {
	c.m.RLock()
	defer c.m.RUnlock()
	var ret string
	for _, e := range c.entries {
		ret += fmt.Sprintf("\t%s\n", e)
	}

	return ret
}

var cache = &Cache{}

func (c *Cache) updateNextRefresh(lock bool) {
	found := c.findLeastTTLCacheEntry(lock)
	log.L().Debugf("Found least DefaultTTL cache entry: %v", found)
	c.setNextRefresh(found)
}

func (c *Cache) findLeastTTLCacheEntry(lock bool) *Entry {
	if lock {
		c.m.RLock()
		defer c.m.RUnlock()
	}

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

func (c *Cache) find(dnsName string) *Entry {
	c.m.RLock()
	defer c.m.RUnlock()
	foundIdx := slices.IndexFunc(c.entries, func(de *Entry) bool { return de.Fqdn() == dnsName })
	if foundIdx > -1 {
		return c.entries[foundIdx]
	} else {
		return nil
	}
}
func (c *Cache) add(fqdn string) (*Entry, error) {
	var entry = c.find(fqdn)
	var e error = nil
	if entry == nil {

		entry = NewEntry(fqdn)
		c.m.Lock()
		c.entries = append(c.entries, entry)
		c.m.Unlock()

		e = resolve(entry)
	}

	return entry, e
}

func (c *Cache) getNextRefresh() *Entry {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.nextRefresh
}

func (c *Cache) setNextRefresh(next *Entry) {
	c.m.Lock()
	defer c.m.Unlock()

	c.nextRefresh = next
}

func (c *Cache) remove(fqdn string) error {
	c.m.Lock()
	defer c.m.Unlock()

	found := slices.IndexFunc(c.entries, func(de *Entry) bool { return de.Fqdn() == fqdn })
	if found == -1 {
		return fmt.Errorf("entry not found")
	}

	c.entries[found] = c.entries[len(c.entries)-1]
	c.entries = c.entries[:len(c.entries)-1]
	c.updateNextRefresh(false)

	return nil
}

func (c *Cache) clear() {
	c.m.Lock()
	defer c.m.Unlock()

	c.entries = make([]*Entry, 0, len(c.entries))
}
