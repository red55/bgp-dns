package main

import (
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/dns"
	"github.com/red55/bgp-dns-peer/internal/log"
	"net"
	"os"
	"syscall"
)

func main() {
	cfg.Init()
	defer cfg.Deinit()

	if e := log.Init(cfg.AppCfg.Log()); e != nil {
		panic(e)
	}
	defer log.Deinit()
	// Reload Logging configuration
	_ = cfg.RegisterConfigChangeHandler(func() {
		_ = log.FireConfigChanged(cfg.AppCfg.Log())
	},
	)

	dns.Init()
	defer dns.Deinit()

	wd, _ := syscall.Getwd()
	log.L().Infof("My working directory: %s", wd)

	var resolvers []*net.UDPAddr = make([]*net.UDPAddr, len(cfg.AppCfg.Resolvers()))
	for i, r := range cfg.AppCfg.Resolvers() {
		resolvers[i] = r
	}

	dns.SetResolvers(resolvers)

	log.L().Info("Waiting for termination signal")
	var b []byte
	_, _ = os.Stdin.Read(b)
}
