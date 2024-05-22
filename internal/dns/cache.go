package dns

import (
	"bufio"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/log"
	"os"
	"slices"
	"strings"
	"sync"
)

type Cache struct {
	m           sync.RWMutex
	gen         uint64
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

var _cache = &Cache{}

func (c *Cache) generation() uint64 {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.gen
}

func (c *Cache) setGeneration(gen uint64) {
	c.m.Lock()
	defer c.m.Unlock()
	c.gen = gen
}

func (c *Cache) updateNextRefresh(lock bool) {
	found := c.findLeastTTLCacheEntry(lock)
	log.L().Debugf("Found least TTL cache entry: %v", found)
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
		entry.generation = c.generation()

		c.m.Lock()
		c.entries = append(c.entries, entry)
		c.m.Unlock()

		dns.HandleFunc(fqdn, respond)

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
	dns.HandleRemove(fqdn)
	c.entries[found] = c.entries[len(c.entries)-1]
	c.entries = c.entries[:len(c.entries)-1]
	c.updateNextRefresh(false)

	return nil
}
func (c *Cache) findPreviousGeneration(gen uint64) []*Entry {
	c.m.RLock()
	defer c.m.RUnlock()

	r := make([]*Entry, 0, len(c.entries)/2)
	for _, e := range c.entries {
		if e.generation < gen {
			r = append(r, e)
		}
	}
	return r
}

func (c *Cache) evictByGeneration() {
	prevG := c.findPreviousGeneration(c.generation())

	for _, e := range prevG {
		log.L().Debugf("Evicting entry: %s, gen: %d, current gen: %d", e.Fqdn(), e.generation, c.generation())
		_ = c.remove(e.Fqdn())
	}
}

func (c *Cache) loadFile(file string) error {
	f, e := os.Open(file)
	if e != nil {
		return e
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.L().Warnf("Failed to close file: %v", err)
		}
	}(f)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fqdn := strings.TrimSpace(scanner.Text())
		if len(fqdn) == 0 {
			continue
		}
		found := c.find(fqdn)

		if found == nil {
			_, _ = c.add(fqdn)
		} else {
			found.generation = c.generation()
		}
	}

	c.updateNextRefresh(true)
	return e
}

func (c *Cache) foreEach(f func(entry *Entry) bool /*return false to break iteration*/) {
	c.m.RLock()

	for _, e := range c.entries {
		if !f(e) {
			break
		}
	}
	defer c.m.RUnlock()
}

func (c *Cache) load(files []string) {
	c.setGeneration(c.generation() + 1)

	for _, file := range files {
		if e := c.loadFile(file); e != nil {
			log.L().Warnf("[Cache Load] Unable to open file, ignoring: %v", e)
			continue
		}
	}

	c.evictByGeneration()
}

func (c *Cache) clear() {
	c.m.Lock()
	defer c.m.Unlock()

	c.entries = make([]*Entry, 0, len(c.entries))
}
