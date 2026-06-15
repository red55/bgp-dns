package bgp

import (
	"context"
	"io"
	"testing"

	bgpapi "github.com/osrg/gobgp/v3/api"
	bgpsrv "github.com/osrg/gobgp/v3/pkg/server"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"github.com/rs/zerolog"
	"net"
	"sync"
	"sync/atomic"
)

type bgpSrv struct {
	loop.Loop
	log.Log
	bgp *bgpsrv.BgpServer
	//ipRefCounter  *hashmap.Map[string, *atomic.Uint64]
	ipRefCounter map[string]*atomic.Uint64
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	asn          uint32
	id           net.IP
}

var (
	_bgp *bgpSrv

	_v4Family = &bgpapi.Family{
		Afi:  bgpapi.Family_AFI_IP,
		Safi: bgpapi.Family_SAFI_UNICAST,
	}
)

func Serve(ctx context.Context) (e error) {
	cfg := ctx.Value("cfg").(*config.AppCfg)
	_bgp = &bgpSrv{
		Loop: loop.NewLoop(1),
		Log:  log.NewLog(log.L(), "bgp"),
		bgp:  bgpsrv.NewBgpServer(bgpsrv.LoggerOption(newZeroLogger(cfg.Log.Level))),
		//ipRefCounter: hashmap.New[string, *atomic.Uint64](),
		ipRefCounter: make(map[string]*atomic.Uint64),
		asn:          cfg.Bgp.Asn,
		id:           cfg.Bgp.Id,
	}
	go func() {
		_bgp.bgp.Serve()
	}()

	ctx, _bgp.cancel = context.WithCancel(ctx)

	if e = _bgp.bgp.StartBgp(ctx, &bgpapi.StartBgpRequest{
		Global: &bgpapi.Global{
			Asn:             _bgp.asn,
			RouterId:        cfg.Bgp.Id.String(),
			ListenAddresses: []string{cfg.Bgp.Listen.IP.String()},
			ListenPort:      int32(cfg.Bgp.Listen.Port),
			ApplyPolicy: &bgpapi.ApplyPolicy{
				ExportPolicy: &bgpapi.PolicyAssignment{
					DefaultAction: bgpapi.RouteAction_ACCEPT,
				},
			},
		},
	}); e != nil {
		_bgp.L().Panic().Err(e).Msg("Failed to start BGP instance")
	}

	for _, peer := range cfg.Bgp.Peers {
		pol := &bgpapi.ApplyPolicy{
			ImportPolicy: &bgpapi.PolicyAssignment{
				Direction:     bgpapi.PolicyDirection_IMPORT,
				DefaultAction: bgpapi.RouteAction_REJECT,
			},
			ExportPolicy: &bgpapi.PolicyAssignment{
				Direction:     bgpapi.PolicyDirection_EXPORT,
				DefaultAction: bgpapi.RouteAction_ACCEPT,
			},
		}

		if e = _bgp.bgp.AddPeer(ctx, &bgpapi.AddPeerRequest{
			Peer: &bgpapi.Peer{
				ApplyPolicy: pol,
				Conf: &bgpapi.PeerConf{
					NeighborAddress: peer.Address.IP.String(),
					PeerAsn:         peer.Asn,
				},
				EbgpMultihop: &bgpapi.EbgpMultihop{
					Enabled:     peer.Multihop,
					MultihopTtl: 254,
				},
				Timers: &bgpapi.Timers{
					Config: &bgpapi.TimersConfig{
						HoldTime: 240,
					},
				},
				Transport: &bgpapi.Transport{
					PassiveMode:  peer.PassiveMode,
					MtuDiscovery: true,
					LocalAddress: cfg.Bgp.Listen.IP.String(),
				},
				RouteServer: &bgpapi.RouteServer{
					RouteServerClient: false,
					SecondaryRoute:    false,
				},

				AfiSafis: []*bgpapi.AfiSafi{
					{
						Config: &bgpapi.AfiSafiConfig{
							Family:  _v4Family,
							Enabled: true,
						},
					},
				},
			},
		}); e != nil {
			_bgp.L().Fatal().Err(e).Msgf("Failed to add peer %s", peer.Address.String())
		}
	}

	go _bgp.loop(ctx)

	return nil
}
func Shutdown(ctx context.Context) (e error) {
	if e = _bgp.bgp.StopBgp(ctx, &bgpapi.StopBgpRequest{}); e != nil {
		_bgp.L().Panic().Err(e).Msg("Failed to shutdown BGP instance")
	}
	_bgp.cancel()
	_bgp.bgp.Stop()
	_bgp.wg.Wait()
	return nil
}

func Advance(ips []string) error {
	return _bgp.Operation(func() (e error) {
		for _, ip := range ips {
			counter := new(atomic.Uint64)
			refs, ok := _bgp.ipRefCounter[ip]
			if !ok {
				refs = counter
				_bgp.ipRefCounter[ip] = counter
			}
			c := refs.Add(1)
			if c == 1 {
				_bgp.L().Debug().Msgf("Advance IPs: %s", ip)
				prefix := &bgpapi.IPAddressPrefix{
					PrefixLen: 32,
					Prefix:    ip,
				}
				e = _bgp.add(prefix, _bgp.asn)
			} else {
				_bgp.L().Debug().Msgf("Advance IPs: No need to change BGP, %v(%d)", ip, c)
			}
		}
		return
	}, true)
}

// SetBgpForTest replaces the global _bgp with the provided instance.
// This is a test helper to allow dns package tests to control BGP state.
func SetBgpForTest(s *bgpSrv) {
	_bgp = s
}

// NewBgpSrvForTest creates a bgpSrv suitable for E2E testing from other packages.
// Returns (srv, cancel) — caller should call cancel in t.Cleanup.
func NewBgpSrvForTest(t testing.TB) (*bgpSrv, context.CancelFunc) {
	t.Helper()
	l := zerolog.New(io.Discard).Level(zerolog.WarnLevel)
	srv := &bgpSrv{
		Loop:         loop.NewLoop(1),
		Log:          log.NewLog(&l, "bgp"),
		ipRefCounter: make(map[string]*atomic.Uint64),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go srv.loop(ctx)
	t.Cleanup(cancel)
	return srv, cancel
}

// GetBgpRefCounter returns a copy of the current ipRefCounter map.
// This is a test helper to verify reference counts from outside the bgp package.
func GetBgpRefCounter() map[string]uint64 {
	if _bgp == nil {
		return nil
	}
	result := make(map[string]uint64)
	for ip, counter := range _bgp.ipRefCounter {
		result[ip] = counter.Load()
	}
	return result
}

func Withdraw(ips []string) error {
	return _bgp.Operation(func() (e error) {
		for _, ip := range ips {
			if refs, exists := _bgp.ipRefCounter[ip]; exists {
				c := refs.Add(^uint64(0)) // Decrement the counter by 1
				if c < 1 {
					_bgp.L().Debug().Msgf("Withdraw IPs: %v", ip)
					prefix := &bgpapi.IPAddressPrefix{
						PrefixLen: 32,
						Prefix:    ip,
					}
					if e = _bgp.remove(prefix, _bgp.asn); e != nil {
						_bgp.L().Error().Err(e)
					}
					delete(_bgp.ipRefCounter, ip)
				} else {
					_bgp.L().Debug().Msgf("Withdraw IPs: No need to change BGP, %v(%d)", ip, c)
				}
			}
		}
		return
	}, true)
}
