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

var _cacheCallbacks *CacheCallbacks = &CacheCallbacks{}

type CacheOperationCallback func(fqdn string, previps, ips []string)

func RegisterDnsCallback(cb CacheOperationCallback) error {
	_cacheCallbacks.mutex.Lock()
	defer _cacheCallbacks.mutex.Unlock()

	p := reflect.ValueOf(cb).Pointer()
	found := slices.ContainsFunc(_cacheCallbacks.callbacks, func(callback CacheOperationCallback) bool {
		return reflect.ValueOf(callback).Pointer() == p
	})

	if found {
		return fmt.Errorf("_Cache callback already registered")
	}
	_cacheCallbacks.callbacks = append(_cacheCallbacks.callbacks, cb)

	return nil
}

func fireCallbacks(fqdn string, previps, ips []string) {
	_cacheCallbacks.mutex.RLock()
	defer _cacheCallbacks.mutex.RUnlock()
	for _, cb := range _cacheCallbacks.callbacks {
		cb(fqdn, previps, ips)
	}
}
