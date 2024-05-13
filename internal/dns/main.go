package dns

import (
	"container/ring"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/log"
	"net"
	"sync"
)

type resolverT struct {
	addr *net.UDPAddr
	ok   bool
}

func (r *resolverT) String() string {
	return fmt.Sprintf("%s, ok=%t", r.addr.String(), r.ok)
}

type resolversT struct {
	m         sync.RWMutex
	resolvers *ring.Ring
}

var (
	_resolvers, _defaultResolvers resolversT
	_wg                           sync.WaitGroup
)

func (r *resolversT) setResolvers(resolvers []*net.UDPAddr) {
	r.m.Lock()
	defer r.m.Unlock()

	l := len(resolvers)
	r.resolvers = ring.New(l)

	for i := 0; i < l; i++ {
		r.resolvers.Value = &resolverT{
			resolvers[i],
			true,
		}
		r.resolvers = r.resolvers.Next()
	}
}

func Init() {
	// Clear dns _Cache and resolve configured dns names
	_ = cfg.RegisterConfigChangeHandler(resolveOnConfigChange)
	_ = cfg.RegisterConfigChangeHandler(responderOnConfigChange)

	dns.HandleFunc(".", proxyQuery)

	go func() {
		server := &dns.Server{
			Addr:      cfg.AppCfg.Responder().String(),
			Net:       "udp",
			ReusePort: true}
		if err := server.ListenAndServe(); err != nil {
			fmt.Printf("Failed to setup the udp server: %s\n", err.Error())
		}
	}()

	go loop(_cmdChannel)

	resolveOnConfigChange()
	responderOnConfigChange()
}

func Deinit() {
	log.L().Infof("Dns->Deinit(): Enter")
	defer log.L().Infof("Dns->Deinit(): Done")
	_cmdChannel <- &dnsOp{
		op: opQuit,
	}
	_wg.Wait()
}

func CacheEnqueue(fqdn string) {
	_cmdChannel <- &dnsOp{
		op:   opAdd,
		fqdn: fqdn,
	}
}

func CacheClear() {
	_cmdChannel <- &dnsOp{
		op: opClear,
	}
}
