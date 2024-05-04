package main

import (
	"github.com/red55/bgp-dns-peer/internal/bgp"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/dns"
	"github.com/red55/bgp-dns-peer/internal/log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	wd, _ := syscall.Getwd()
	log.L().Infof("My working directory: %s", wd)

	cfg.Init()
	defer cfg.Deinit()

	if e := log.Init(cfg.AppCfg.Log()); e != nil {
		panic(e)
	}
	defer log.Deinit()
	// Reload Logging configuration

	dns.Init()
	defer dns.Deinit()

	bgp.Init()
	defer bgp.Deinit()

	log.L().Info("Waiting for termination signal")
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
