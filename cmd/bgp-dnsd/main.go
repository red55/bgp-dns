package main
import (
    "context"
    "errors"
    "fmt"
    "github.com/red55/bgp-dns/internal/bgp"
    "github.com/red55/bgp-dns/internal/config"
    "github.com/red55/bgp-dns/internal/dns"
    "github.com/red55/bgp-dns/internal/fswatcher"
    "github.com/red55/bgp-dns/internal/log"
    "github.com/rs/zerolog"
    "github.com/spf13/pflag"
    "os"
    "os/signal"
    "path/filepath"
)
type app struct {
    log.Log
}

var (
    _app *app
    version = "dev"
    commit  = "none"
    date    = "unknown"
)

func (a *app) stdErr(e error, s string,  v ...interface{}) {
    s = fmt.Sprintf(s, v...)
    a.L().Error().Err(e).Msgf(s, v...)
    if a.L().GetLevel() > zerolog.ErrorLevel {
        _, _ = fmt.Fprintf(os.Stderr, "%s - %v\n", s, e)
    }

}

func (a *app) stdOut(s string,  v ...interface{}) {
    s = fmt.Sprintf(s, v...)
    a.L().Info().Msgf(s, v...)
    if a.L().GetLevel() > zerolog.InfoLevel {
        _, _ = fmt.Fprintf(os.Stdout, "%s\n", s)
    }
}

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

    _app.stdOut("Starting up %s (%s) built on %s...", version, commit, date )
    defer func () {
        _app.stdOut("Shutdown complete.")
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
            _app.stdErr(e, "BGP Shutdown failed ")
        }
    }()

    if e = dns.Serve(ctx); e != nil {
        panic(e)
    }
    defer func() {
        if e = dns.Shutdown(ctx); e != nil {
            _app.stdErr(e, "DNS Shutdown failed ")
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
            _app.stdErr(e, "FSWatcher Shutdown failed ")
        }
    }()

    _app.stdOut("Startup complete.")
    select {
    case <-c :
        _app.stdOut("Gracefully shutting down...")
        break
    case <-ctx.Done():
        if ctx.Err() != nil {
            _app.stdErr(e, "Ctrl+C failed ")
        }
    }
}

