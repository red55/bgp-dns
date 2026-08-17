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
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/rs/zerolog"
)

// Service holds all DNS subsystem state.
type Service struct {
	loop      loop.Loop
	log.Log
	cfg       *config.AppCfg
	cache     *cache
	resolvers *resolvers
	cancel    context.CancelFunc
	server    *dns.Server
	wg        sync.WaitGroup
}

// Package-level global for backward compatibility (set by Serve).
var _dns *Service

var (
	EInvalidFQDN    = errors.New("invalid FQDN")
	ENotInitialized = errors.New("cache subsystem is not initialized")

	QTypeToString = dns.TypeToString
)

// NewDns creates a DNS service with explicit dependencies.
// Returns (Service, error) — no panics.
func NewDns(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*Service, error) {
	s := &Service{
		cfg:  cfg,
		loop: l,
		Log:  log.NewLog(logger, "dns"),
	}

	s.resolvers = newResolvers(cfg.Dns.Resolvers, logger)
	s.cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, s.resolvers, logger, cfg, l)
	mux := newRegexServeMux()
	s.cache.SetMux(mux)
	mux.SetCatchAll(s.resolvers.proxyQuery)

	if err := s.cache.serve(context.Background()); err != nil {
		return nil, fmt.Errorf("dns: cache serve failed: %w", err)
	}

	return s, nil
}

// Serve creates the DNS service and stores it in the package-level _dns global.
// This wrapper exists for backward compatibility with Phase 2 tests.
func Serve(ctx context.Context) error {
	cfg := ctx.Value(config.ConfigKey{}).(*config.AppCfg)
	l := loop.NewLoop(1, log.L())
	s, err := NewDns(cfg, l, log.L())
	if err != nil {
		return err
	}
	_dns = s
	return nil
}

// Shutdown shuts down the DNS service.
func Shutdown(ctx context.Context) error {
	if _dns == nil {
		return nil
	}
	return _dns.Shutdown(ctx)
}

// Shutdown shuts down this DNS service instance.
func (s *Service) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.cache.mux.clear()
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	_ = s.cache.shutdown()

	if s.server != nil {
		if e := s.server.ShutdownContext(shutdownCtx); e != nil && !errors.Is(e, context.Canceled) {
			return e
		}
	}
	_ = s.cache.evictByGeneration(s.cache.generation())
	s.wg.Wait()
	return nil
}

// Load delegates to the package-level service.
func Load(fn string) error {
	if _dns == nil || _dns.cache == nil {
		return ENotInitialized
	}
	return _dns.cache.load(fn)
}

// DumpCache delegates to the package-level service.
func DumpCache(callback func(qtype uint16, fqdn string, fails uint64, ips []string, ttl time.Duration, expiration time.Time, gen uint64) error) error {
	if _dns == nil || _dns.cache == nil {
		return ENotInitialized
	}
	return _dns.cache.dump(callback)
}

// ClearCache delegates to the package-level service.
func ClearCache() (uint64, error) {
	if _dns == nil || _dns.cache == nil {
		return 0, ENotInitialized
	}
	return _dns.cache.clear(), nil
}
