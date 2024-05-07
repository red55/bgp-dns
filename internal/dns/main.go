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
}

var _resolvers, _defaultResolvers resolversT

func init() {
	cache = &Cache{}

	chanRefresher = make(chan *msg)
}

func (r *resolversT) setResolvers(resolvers []*net.UDPAddr) {
	r.m.Lock()
	defer r.m.Unlock()

	l := len(resolvers)
	r.resolvers = ring.New(l)

	for i := 0; i < l; i++ {
		r.resolvers.Value = &resovlerT{
			resolvers[i],
			true,
		}
		r.resolvers = r.resolvers.Next()
	}
}

func Init() {
	// Clear dns cache and resolve configured dns names
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

	go refresher(chanRefresher)

	resolveOnConfigChange()
	responderOnConfigChange()
}

func Deinit() {

	chanRefresher <- &msg{
		op: opQuit,
	}
	close(chanRefresher)
}

func CacheEnqueue(fqdn string) {
	chanRefresher <- &msg{
		op:   opAdd,
		fqdn: fqdn,
	}
}

func CacheClear() {
	chanRefresher <- &msg{
		op: opClear,
	}
}
