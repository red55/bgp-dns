package dns

import (
	"fmt"
	"reflect"
	"slices"
	"sync"
)

type CacheCallbacks struct {
	callbacks []CacheOperationCallback
	mutex     sync.RWMutex
}

var cacheCallbacks *CacheCallbacks = &CacheCallbacks{}

type CacheOperationCallback func(fqdn string, previps, ips []string)

func RegisterDnsCallback(cb CacheOperationCallback) error {
	cacheCallbacks.mutex.Lock()
	defer cacheCallbacks.mutex.Unlock()

	p := reflect.ValueOf(cb).Pointer()
	found := slices.ContainsFunc(cacheCallbacks.callbacks, func(callback CacheOperationCallback) bool {
		return reflect.ValueOf(callback).Pointer() == p
	})

	if found {
		return fmt.Errorf("cache callback already registered")
	}
	cacheCallbacks.callbacks = append(cacheCallbacks.callbacks, cb)

	return nil
}

func fireCallbacks(fqdn string, previps, ips []string) {
	cacheCallbacks.mutex.RLock()
	defer cacheCallbacks.mutex.RUnlock()
	for _, cb := range cacheCallbacks.callbacks {
		cb(fqdn, previps, ips)
	}
}
