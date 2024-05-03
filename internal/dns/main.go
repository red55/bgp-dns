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
	cache = &Cache{}

	chanRefresher = make(chan *msg)
}

func setResolvers(resolvers []*net.UDPAddr) {
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
	_ = cfg.RegisterConfigChangeHandler(resolverOnConfigChange)
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

	go refresher(chanRefresher)

	resolverOnConfigChange()
	responderOnConfigChange()
}

func Deinit() {

	chanRefresher <- &msg{
		op: opQuit,
	}
	close(chanRefresher)
}

func ResolverEnqueue(fqdn string) {
	chanRefresher <- &msg{
		op:   opAdd,
		fqdn: fqdn,
	}
}

func ResolverClear() {
	chanRefresher <- &msg{
		op: opClear,
	}
}
