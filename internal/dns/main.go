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
	///

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
}

func Deinit() {
	log.L().Infof("Dns->Deinit(): Enter")
	defer log.L().Infof("Dns->Deinit(): Done")
	_cmdChannel <- &dnsOp{
		op: opQuit,
	}
	_wg.Wait()
}

func Load(files []string) {
	_cmdChannel <- &dsnOpLoad{
		dnsOp: dnsOp{
			op: opLoad,
		},
		files:    files,
		additive: true,
	}
}

func CacheEnqueue(fqdn string) {
	var op = dnsOpFqdn{
		dnsOp: dnsOp{
			op: opAdd,
		},
		fqdn: fqdn,
	}
	_cmdChannel <- op
}

func CacheClear() {
	_cmdChannel <- &dnsOp{
		op: opClear,
	}
}
