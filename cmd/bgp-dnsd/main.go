package main

import (
	"context"
	"errors"
	"github.com/red55/bgp-dns/cmd/bgp-dnsd/cli"
	"github.com/red55/bgp-dns/cmd/bgp-dnsd/commands"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/red55/bgp-dns/internal/app"
	"github.com/red55/bgp-dns/internal/bgp"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/dns"
	"github.com/red55/bgp-dns/internal/fswatcher"
	"github.com/red55/bgp-dns/internal/version"
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
		panic(errors.New("wrong path to configuration file"))
	}
	var cfg *config.AppCfg
	if cfg, e = config.Init(configPath); e != nil {
		panic(e)
	}
	_app.SetLevel(cfg.Log.Level)
	log.SetLevel(cfg.Log.Level)

	defer func() {
		_app.StdOut("Shutdown complete.")
	}()

	ctx := context.Background()
	ctx = context.WithValue(ctx, "cfg", cfg)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	if e = bgp.Serve(ctx); e != nil {
		panic(e)
	}
	defer func() {
		if e = bgp.Shutdown(ctx); e != nil {
			_app.StdErr(e, "BGP Shutdown failed ")
		}
	}()

	if e = cli.Serve(_app); e != nil {
		panic(e)
	}
	defer func() {
		if e = cli.Shutdown(_app); e != nil {
			_app.StdErr(e, "CLI Shutdown failed ")
		}
	}()

	if e = dns.Serve(ctx); e != nil {
		panic(e)
	}
	defer func() {
		if e = dns.Shutdown(ctx); e != nil {
			_app.StdErr(e, "DNS Shutdown failed ")
		}
	}()

	if e = dns.Load(cfg.Dns.List.File); e != nil {
		panic(e)
	}

	if e = fswatcher.Serve(ctx); e != nil {
		panic(e)
	}
	defer func() {
		if e = fswatcher.Shutdown(ctx); e != nil {
			_app.StdErr(e, "FSWatcher Shutdown failed ")
		}
	}()

	_app.StdOut("Startup complete.")
	select {
	case <-c:
		_app.StdOut("Gracefully shutting down...")
		break
	case <-ctx.Done():
		if ctx.Err() != nil {
			_app.StdErr(e, "Ctrl+C failed ")
		}
	}
}
