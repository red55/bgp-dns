package main

import (
	"context"
	"github.com/red55/bgp-dns/cmd/bgp-dnsd/cli"
	"github.com/red55/bgp-dns/cmd/bgp-dnsd/commands"
	"github.com/red55/bgp-dns/internal/bgp"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/dns"
	"github.com/red55/bgp-dns/internal/fswatcher"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/red55/bgp-dns/internal/version"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/red55/bgp-dns/internal/app"
)

var _app *app.Application

func main() {

	_app = app.New(filepath.Base(os.Args[0]), zerolog.InfoLevel)
	_app.StdOut("Starting up %s %s...", _app.Name(), version.Version())

	if e := commands.NewRootCmd(_app).Execute(); e != nil {
		_app.StdErr(e, "error executing command")
	}

	configPath, e := filepath.Abs(_app.Flags.Config)
	if e != nil {
		_app.StdErr(e, "cannot resolve config path")
		os.Exit(1)
	}
	var cfg *config.AppCfg
	cfg, e = config.Init(configPath)
	if e != nil {
		_app.StdErr(e, "failed to load configuration")
		os.Exit(1)
	}
	_app.SetLevel(cfg.Log.Level)
	log.SetLevel(cfg.Log.Level)

	defer func() {
		_app.StdOut("Shutdown complete.")
	}()

	ctx := context.Background()
	ctx = context.WithValue(ctx, config.ConfigKey{}, cfg)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// BGP service — created via constructor for deferred cleanup
	bgpSrv, e := bgp.NewBgp(ctx, cfg, loop.NewLoop(1, log.L()), log.L())
	if e != nil {
		_app.StdErr(e, "failed to start BGP service")
		os.Exit(1)
	}
	defer func() {
		if e := bgpSrv.Shutdown(ctx); e != nil {
			_app.StdErr(e, "BGP shutdown failed")
		}
	}()

	// CLI service
	if e := cli.Serve(_app, cfg.Dns.List.File); e != nil {
		_app.StdErr(e, "failed to start CLI service")
		os.Exit(1)
	}
	defer func() {
		if e := cli.Shutdown(_app); e != nil {
			_app.StdErr(e, "CLI shutdown failed")
		}
	}()

	// DNS service — created via constructor for deferred cleanup
	dnsSrv, e := dns.NewDns(cfg, loop.NewLoop(1, log.L()), log.L())
	if e != nil {
		_app.StdErr(e, "failed to start DNS service")
		os.Exit(1)
	}
	defer func() {
		if e := dnsSrv.Shutdown(ctx); e != nil {
			_app.StdErr(e, "DNS shutdown failed")
		}
	}()

	// Load domainlist
	if e := dns.Load(cfg.Dns.List.File); e != nil {
		_app.StdErr(e, "failed to load domainlist")
		os.Exit(1)
	}

	// FSWatcher service — created via constructor for deferred cleanup
	fsSrv, e := fswatcher.NewFsWatcher(cfg, loop.NewLoop(1, log.L()), log.L())
	if e != nil {
		_app.StdErr(e, "failed to start FS watcher")
		os.Exit(1)
	}
	defer func() {
		if e := fsSrv.Shutdown(ctx); e != nil {
			_app.StdErr(e, "FS watcher shutdown failed")
		}
	}()

	_app.StdOut("Startup complete.")
	select {
	case <-c:
		_app.StdOut("Gracefully shutting down...")
	case <-ctx.Done():
		if ctx.Err() != nil {
			_app.StdErr(ctx.Err(), "shutdown signal")
		}
	}
}
