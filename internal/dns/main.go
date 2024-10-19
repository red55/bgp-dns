package dns

import (
	"context"
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"sync"
	"time"
)

var (
	_server *dns.Server
	_wg     sync.WaitGroup
	_resolvers *resolvers
	_cancel context.CancelFunc
	_cache *cache

	EInvalidFQDN = errors.New("invalid FQDN")
	ENotInitialized = errors.New("cache subsystemd is not initialized")
)

func Serve(ctx context.Context) error {
	var cfg = ctx.Value("cfg").(*config.AppCfg)

	if nil != _cancel {
		_cancel()
	}
	ctx, _cancel = context.WithCancel(ctx)

	_resolvers = newResolvers(cfg.Dns.Resolvers)

	go func(c context.Context) {
		_server = &dns.Server{
			Addr:      fmt.Sprintf("%s:%d", cfg.Dns.Listen.IP.String(), cfg.Dns.Listen.Port),
			Net:       "udp",
			ReusePort: true,
		}
		_wg.Add(1)
		defer _wg.Done()

		dns.HandleFunc(".", _resolvers.proxyQuery)
		if e := _server.ListenAndServe(); e != nil {
			log.L().Fatal().Str("m", "dns").Err(e).Msg("Failed to bind DNS resolver")
		}
	}(ctx)

	_cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, newResolvers(cfg.Dns.List.Resolvers), log.L())

	return _cache.serve(ctx)
}

func Shutdown(ctx context.Context) error {
	dns.HandleRemove(".")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func () {
		cancel()
	}()

	if nil != _cancel {
		_cancel()
		_cancel = nil
	}
	_ = _cache.shutdown()

	if e := _server.ShutdownContext(ctx); e != nil && !errors.Is(e, context.Canceled) {
		return e
	}

	_ = _cache.evictByGeneration(_cache.generation())

	_wg.Wait()

	return nil
}
func Register(fqdn string) error {
	if _cache == nil {
		return ENotInitialized
	}
	return _cache.register(fqdn)
}

func Unregister(fqdn string) error {
	if _cache == nil {
		return ENotInitialized
	}
	return _cache.unregister(fqdn)
}

func Load(fn string) error {
	if _cache == nil {
		return ENotInitialized
	}
	return _cache.load(fn)
}
