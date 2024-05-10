package main

import (
	"github.com/red55/bgp-dns/internal/bgp"
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/dns"
	"github.com/red55/bgp-dns/internal/krt"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/spf13/pflag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	wd, _ := syscall.Getwd()
	log.L().Infof("My working directory: %s", wd)

	pflag.StringP("config", "c", "appsettings.yml", "Path to configuration file.")
	pflag.Parse()

	fn := pflag.Lookup("config")
	configPath, e := filepath.Abs(fn.Value.String())
	if e != nil {
		log.L().Fatalf("Wrong path to configuration file")
	}

	log.L().Infof("My configrution file: %s", configPath)
	cfg.Init(configPath)

	defer cfg.Deinit()

	if e := log.Init(cfg.AppCfg.Log()); e != nil {
		panic(e)
	}
	defer log.Deinit()

	krt.Init()
	defer krt.Deinit()

	bgp.Init()
	defer bgp.Deinit()

	dns.Init()
	defer dns.Deinit()

	log.L().Info("Waiting for termination signal")
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
