package dns

import (
	"container/ring"
	"fmt"
	"github.com/miekg/dns"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"net"
	"sync"
)

type resovlerT struct {
	addr *net.UDPAddr
	ok   bool
}

func (r *resovlerT) String() string {
	return fmt.Sprintf("%s, ok=%t", r.addr.String(), r.ok)
}

type resolversT struct {
	m         sync.RWMutex
	resolvers *ring.Ring
	current   *ring.Ring
}

var _resolvers resolversT

func init() {
	cache = Cache{}
	chanResolver = make(chan string, 10)
	chanRefresher = make(chan struct{})
}

func SetResolvers(resolvers []*net.UDPAddr) {
	_resolvers.m.Lock()
	defer _resolvers.m.Unlock()

	l := len(resolvers)
	_resolvers.resolvers = ring.New(l)
	_resolvers.current = _resolvers.resolvers

	for i := 0; i < l; i++ {
		_resolvers.current.Value = &resovlerT{
			resolvers[i],
			true,
		}
		_resolvers.current = _resolvers.current.Next()
	}
}

func Init() {
	// Clear dns cache and resolve configured dns names
	_ = cfg.RegisterConfigChangeHandler(func() {
		ResolverClear()
		for _, n := range cfg.AppCfg.Names() {
			ResolverEnqueue(n)
		}
	},
	)
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

	go resolver(chanResolver)

	go refresher(chanRefresher)
}

func Deinit() {
	close(chanResolver)

	chanRefresher <- struct{}{}
	close(chanRefresher)
}

func ResolverEnqueue(domainName string) {
	chanResolver <- domainName
}

func ResolverClear() {
	cache.clear()
}
