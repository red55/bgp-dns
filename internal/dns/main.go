package dns

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
)

var (
	_server    *dns.Server
	_wg        sync.WaitGroup
	_resolvers *resolvers
	_cancel    context.CancelFunc
	_cache     *cache

	EInvalidFQDN    = errors.New("invalid FQDN")
	ENotInitialized = errors.New("cache subsystem is not initialized")

	QTypeToString = dns.TypeToString
)

func Serve(ctx context.Context) (e error) {
	var cfg = ctx.Value("cfg").(*config.AppCfg)

	if nil != _cancel {
		_cancel()
	}
	ctx, _cancel = context.WithCancel(ctx)

	_resolvers = newResolvers(cfg.Dns.Resolvers, log.L())
	_cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, newResolvers(cfg.Dns.List.Resolvers, log.L()), log.L())
	mux := newRegexServeMux()
	_cache.SetMux(mux)
	mux.SetCatchAll(_resolvers.proxyQuery)
	e = _cache.serve(ctx)

	go func(c context.Context) {
		_server = &dns.Server{
			Addr:      fmt.Sprintf("%s:%d", cfg.Dns.Listen.IP.String(), cfg.Dns.Listen.Port),
			Net:       "udp",
			ReusePort: true,
			Handler:   mux,
		}
		_wg.Add(1)
		defer _wg.Done()

		if err := _server.ListenAndServe(); err != nil {
			log.L().Fatal().Str("m", "dns").Err(err).Msg("Failed to bind DNS resolver")
		}
	}(ctx)

	return e
}

func Shutdown(ctx context.Context) error {
	_cache.mux.clear()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
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

func Load(fn string) error {
	if _cache == nil {
		return ENotInitialized
	}
	return _cache.load(fn)
}

func DumpCache(callback func(qtype uint16, fqdn string, fails uint64, ips []string, ttl time.Duration, expiration time.Time, gen uint64) error) error {
	if _cache == nil {
		return ENotInitialized
	}

	return _cache.dump(callback)
}

func ClearCache() (uint64, error) {
	if _cache == nil {
		return 0, ENotInitialized
	}

	return _cache.clear(), nil
}
