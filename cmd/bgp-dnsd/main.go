package main
import (
    "github.com/red55/bgp-dns/internal/config"
    "github.com/spf13/pflag"
)
func main() {
    pflag.StringP("config", "c", "appsettings.yml", "Path to configuration file.")
    pflag.Parse()

    fn := pflag.Lookup("config")
    configPath, e := filepath.Abs(fn.Value.String())
    if e != nil {
        log.L().Fatalf("Wrong path to configuration file")
    }

    config.Init(configPath)
}
