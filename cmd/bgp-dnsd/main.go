package main
import (
    "context"
    "errors"
    "github.com/red55/bgp-dns/internal/bgp"
    "github.com/red55/bgp-dns/internal/config"
    "github.com/red55/bgp-dns/internal/dns"
    "github.com/red55/bgp-dns/internal/fswatcher"
    "github.com/red55/bgp-dns/internal/log"
    "github.com/spf13/pflag"
    "os"
    "os/signal"
    "path/filepath"
    "runtime/debug"
)
type app struct {
    log.Log
}

var _app *app

func main() {
    pflag.StringP("config", "c", "appsettings.yml", "Path to configuration file.")
    pflag.Parse()

    fn := pflag.Lookup("config")
    configPath, e := filepath.Abs(fn.Value.String())
    if e != nil {
        panic(errors.New("wrong path to configuration file"))
    }
    var cfg *config.AppCfg
    if cfg, e = config.Init(configPath); e != nil {
        panic(e)
    }

    log.Init(cfg)
    _app = &app{
        Log: log.NewLog(log.L(), ""),
    }

    bi, _ := debug.ReadBuildInfo()
    _app.L().Info().Msgf("Starting up %s...", bi.Main.Version )
    defer func () {
        _app.L().Info().Msg("Shutdown complete.")
    }()

    ctx := context.Background()
    ctx = context.WithValue(ctx, "cfg", cfg)
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    c := make (chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)

    if e = bgp.Serve(ctx); e != nil {
        panic(e)
    }
    defer func() {
        if e = bgp.Shutdown(ctx); e != nil {
            _app.L().Err(e)
        }
    }()

    if e = dns.Serve(ctx); e != nil {
        panic(e)
    }
    defer func() {
        if e = dns.Shutdown(ctx); e != nil {
            _app.L().Err(e)
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
            _app.L().Err(e)
        }
    }()

    _app.L().Info().Msg("Startup complete.")
    select {
    case <-c :
        _app.L().Info().Msg("Gracefully shutting down...")
        break
    case <-ctx.Done():
        if ctx.Err() != nil {
            _app.L().Err(ctx.Err())
        }
    }
}

