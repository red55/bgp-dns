package bgp

import (
	"context"
	bgpapi "github.com/osrg/gobgp/v3/api"
	bgpsrv "github.com/osrg/gobgp/v3/pkg/server"
	"github.com/cornelk/hashmap"
	"github.com/red55/bgp-dns/internal/config"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/loop"
	"sync"
	"sync/atomic"
)

type bgpSrv struct {
	loop.Loop
	bgp *bgpsrv.BgpServer
	ipRefCounter  *hashmap.Map[string, *atomic.Uint64]
	cancel context.CancelFunc
	wg sync.WaitGroup
	asn uint32
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
		Loop:         loop.NewLoop(1),
		bgp:          bgpsrv.NewBgpServer(bgpsrv.LoggerOption(newZeroLogger(cfg.Log.Level))),
		ipRefCounter: hashmap.New[string, *atomic.Uint64](),
		asn: cfg.Bgp.Asn,
	}
	go func () {
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
		log.L().Panic().Err(e).Msg("Failed to start BGP instance")
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
					NeighborAddress: peer.Addr.IP.String(),
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
				log.L().Fatal().Err(e).Msgf("Failed to add peer %s", peer.Addr.String())
			}
	}

	go _bgp.loop(ctx)

	return nil
}
func Shutdown(ctx context.Context) (e error) {
	if e = _bgp.bgp.StopBgp(ctx,  &bgpapi.StopBgpRequest{}); e != nil {
		log.L().Panic().Err(e).Msg("Failed to shutdown BGP instance")
	}
	_bgp.cancel()
	_bgp.bgp.Stop()
	_bgp.wg.Wait()
	return nil
}

func Advance(ips []string) error {
	return _bgp.Operation(func () (e error) {
		for _, ip := range ips {
			counter := new(atomic.Uint64)
			refs, _ := _bgp.ipRefCounter.GetOrInsert(ip, counter)
			c := refs.Add(1)
			if  c == 1 {
				log.L().Debug().Msgf("Advance IPs: %v", ip)
				prefix := &bgpapi.IPAddressPrefix{
					PrefixLen: 32,
					Prefix:    ip,
				}
				e = _bgp.add(prefix, _bgp.asn)
			} else {
				log.L().Debug().Msgf("No need to change BGP, %v(%d)", ip, c)
			}
		}
		return
	})
}

func Withdraw(ips []string) error {
	return _bgp.Operation( func () (e error) {
		for _, ip := range ips {
			if refs, exists := _bgp.ipRefCounter.Get(ip); exists {
				c := refs.Add(^uint64(0))
				if c < 1 {
					log.L().Debug().Msgf("Withdraw IPs: %v", ip)
					prefix := &bgpapi.IPAddressPrefix{
						PrefixLen: 32,
						Prefix:    ip,
						}
						if e = _bgp.remove(prefix, _bgp.asn); e != nil {
							log.L().Error().Err(e)
						}
						_bgp.ipRefCounter.Del(ip)
				} else {
					log.L().Debug().Msgf("No need to change BGP, %v(%d)", ip, c)
				}
			}
		}
		return
	})
}