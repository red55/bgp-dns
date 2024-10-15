package main
import (
    "context"
    "errors"
    "github.com/red55/bgp-dns/internal/config"
    "github.com/red55/bgp-dns/internal/dns"
    "github.com/red55/bgp-dns/internal/log"
    "github.com/spf13/pflag"
    "os"
    "os/signal"
    "path/filepath"
    "runtime/debug"
)
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

    bi, _ := debug.ReadBuildInfo()
    log.L().Info().Msgf("Starting up %s...", bi.Main.Version )
    defer func () {
        log.L().Info().Msg("Shutdown complete.")
    }()

    ctx := context.Background()
    ctx = context.WithValue(ctx, "cfg", cfg)
    ctx, cancel := context.WithCancel(ctx)

    c := make (chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)

    if e = dns.Serve(ctx); e != nil {
        panic(e)
    }
    defer func() {
        if e = dns.Shutdown(ctx); e != nil {
            log.L().Err(e)
        }
    }()

    if e = dns.Load(cfg.Dns.List.File); e != nil {
        panic(e)
    }

    log.L().Info().Msg("Startup complete.")
    select {
    case <-c :
        log.L().Info().Msg("Gracefully shutting down...")
        cancel()
    case <-ctx.Done():
        if ctx.Err() != nil {
            log.L().Err(ctx.Err())
        }
    }
}

