package dns

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	_server *dns.Server
	_wg     sync.WaitGroup
	_resolvers *resolvers;
	_cancel context.CancelFunc
	_cache *cache

	EInvalidFQDN = errors.New("Invalid FQDN")
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
		dns.HandleFunc(".", proxyQuery)
		if e := _server.ListenAndServe(); e != nil {
			log.L().Fatal().Err(e).Msg("Failed to bind DSN resolver")
		}
		_wg.Done()
	}(ctx)

	_cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, newResolvers(cfg.Dns.List.Resolvers))

	go _cache.loop(ctx)

	return nil
}

func Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func () {
		cancel()
	}()

	if nil != _cancel {
		_cancel()
		_cancel = nil
	}
	if e := _server.ShutdownContext(ctx); e != nil && !errors.Is(e, context.Canceled) {
		return e
	}

	evictByGeneration(_cache.generation())

	_wg.Wait()



	return nil
}

func Register(fqdn string) error {
	if len (fqdn) < 2 {
		return fmt.Errorf("'%s'. %w", fqdn, EInvalidFQDN)
	}
	cn := dns.CanonicalName(fqdn)
	dns.HandleFunc(dns.CanonicalName(fqdn), resolve)

	q := new(dns.Msg)
	q.SetQuestion(cn, dns.TypeA)
	// resolve will call cache.upsert on resolved IPs
	resolve(nil, q)

	return nil
}

func Unregister(fqdn string) error {
	if len (fqdn) < 2 {
		return fmt.Errorf("'%s'. %w", fqdn, EInvalidFQDN)
	}

	cn := dns.CanonicalName(fqdn)
	dns.HandleRemove(cn)

	var kr [] string
	for _, k := range _cache.entries.Keys(true) {
		s := k.(string)
		if strings.HasSuffix(s, cn) {
			kr = append(kr, s)
		}
	}

	for _, k := range kr {
		_ = _cache.entries.Remove(k);
	}

	return nil
}

func Load(fn string) error {
	f, e := os.Open(fn)
	if e != nil {
		return e
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.L().Warn().Msgf("Failed to close file: %v", err)
		}
	}(f)

	scanner := bufio.NewScanner(f)
	oldGeneration := _cache.generation()
	_ = _cache.increaseGeneration()

	for scanner.Scan() {
		fqdn := strings.TrimSpace(scanner.Text())
		if len(fqdn) == 0 {
			continue
		}
		if fqdn[0] == '#' || fqdn[0] == ';'{
			continue
		}
		if e = Register(fqdn); e != nil {
			return e
		}

	}

	return evictByGeneration(oldGeneration);
}

func evictByGeneration(gen uint64) error {
	keys := _cache.findKeysByGeneration(gen)

	for _, k := range keys {
		Unregister(k)
	}

	return nil
}