package bgp

import (
	"context"
	"fmt"
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
	bgp          *bgpsrv.BgpServer
	ipRefCounter map[string]*atomic.Uint64
	cancel       context.CancelFunc
	ctx          context.Context
	wg           sync.WaitGroup
	asn          uint32
	id           net.IP
	peers        []string
}

var (
	_bgp *bgpSrv

	_v4Family = &bgpapi.Family{
		Afi:  bgpapi.Family_AFI_IP,
		Safi: bgpapi.Family_SAFI_UNICAST,
	}
)

// peerSpecInput carries the per-peer scalars needed to assemble a GoBGP
// Peer spec. Private; mirrors the config fields buildPeerSpec consumes.
type peerSpecInput struct {
	Asn                uint32
	NeighborAddress    string
	Multihop           bool
	PassiveMode        bool
	ListenLocalAddress string
	AuthPassword       string
}

// buildPeerSpec assembles the full per-peer *bgpapi.Peer for AddPeer. Pure
// function (no server, no state) so the config→spec mapping is unit-testable
// without CAP_NET_ADMIN. The AuthPassword field is the optional RFC 2385/5925
// TCP-MD5 session key — GoBGP applies it via setTCPMD5SigSockopt when
// non-empty; empty leaves sessions unauthenticated (pre-phase behavior).
func buildPeerSpec(in peerSpecInput) (peer *bgpapi.Peer) {
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

	peer = &bgpapi.Peer{
		ApplyPolicy: pol,
		Conf: &bgpapi.PeerConf{
			NeighborAddress: in.NeighborAddress,
			PeerAsn:         in.Asn,
			AuthPassword:    in.AuthPassword,
		},
		EbgpMultihop: &bgpapi.EbgpMultihop{
			Enabled:     in.Multihop,
			MultihopTtl: 254,
		},
		Timers: &bgpapi.Timers{
			Config: &bgpapi.TimersConfig{
				HoldTime: 240,
			},
		},
		Transport: &bgpapi.Transport{
			PassiveMode:  in.PassiveMode,
			MtuDiscovery: true,
			LocalAddress: in.ListenLocalAddress,
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
	}
	return
}

// NewBgp creates a BGP service with explicit dependencies.
// The provided ctx becomes the parent of the service's internal context:
// cancelling it stops the operation loop and reaches every GoBGP API call.
// Returns (bgpSrv, error) — no panics.
func NewBgp(ctx context.Context, cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*bgpSrv, error) {
	s := &bgpSrv{
		Loop:         l,
		Log:          log.NewLog(logger, "bgp"),
		bgp:          bgpsrv.NewBgpServer(bgpsrv.LoggerOption(newZeroLogger(cfg.Log.Level))),
		ipRefCounter: make(map[string]*atomic.Uint64),
		asn:          cfg.Bgp.Asn,
		id:           cfg.Bgp.Id,
	}
	go s.bgp.Serve()

	cctx, cancel := context.WithCancel(ctx)
	s.ctx = cctx
	s.cancel = cancel

	if e := s.bgp.StartBgp(cctx, &bgpapi.StartBgpRequest{
		Global: &bgpapi.Global{
			Asn:             s.asn,
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
		return nil, fmt.Errorf("bgp: start failed: %w", e)
	}

	peers := make([]string, 0, len(cfg.Bgp.Peers))
	for _, p := range cfg.Bgp.Peers {
		peers = append(peers, p.Address.IP.String())
	}
	s.peers = peers

	for _, peer := range cfg.Bgp.Peers {
		spec := buildPeerSpec(peerSpecInput{
			Asn:                  peer.Asn,
			NeighborAddress:      peer.Address.IP.String(),
			Multihop:             peer.Multihop,
			PassiveMode:          peer.PassiveMode,
			ListenLocalAddress:   cfg.Bgp.Listen.IP.String(),
			AuthPassword:         peer.AuthPassword,
		})

		if e := s.bgp.AddPeer(cctx, &bgpapi.AddPeerRequest{Peer: spec}); e != nil {
			return nil, fmt.Errorf("bgp: add peer %s failed: %w", peer.Address.String(), e)
		}
	}

	go s.loop(cctx)
	return s, nil
}

// Serve creates the BGP service and stores it in the package-level _bgp global.
// This wrapper exists for backward compatibility with Phase 2 tests.
func Serve(ctx context.Context) (e error) {
	cfg := ctx.Value(config.ConfigKey{}).(*config.AppCfg)
	l := loop.NewLoop(1, log.L())
	s, err := NewBgp(ctx, cfg, l, log.L())
	if err != nil {
		return err
	}
	_bgp = s
	return nil
}

// Shutdown shuts down the BGP service.
func Shutdown(ctx context.Context) (e error) {
	if _bgp == nil {
		return nil
	}
	return _bgp.Shutdown(ctx)
}

// Shutdown shuts down this BGP service instance.
func (s *bgpSrv) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if e := s.bgp.StopBgp(ctx, &bgpapi.StopBgpRequest{}); e != nil {
		return fmt.Errorf("bgp: shutdown failed: %w", e)
	}
	s.cancel()
	s.bgp.Stop()
	s.wg.Wait()
	return nil
}

func Advance(ips []string) error {
	if _bgp == nil {
		return fmt.Errorf("bgp: not initialized")
	}
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
				if e != nil {
					_bgp.L().Warn().
						Str("op", "advance").
						Str("ip", ip).
						Str("router_id", _bgp.id.String()).
						Strs("peers", _bgp.peers).
						Err(e).
						Msg("BGP advance failed")
					return
				}
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
		Loop:         loop.NewLoop(1, &l),
		Log:          log.NewLog(&l, "bgp"),
		ipRefCounter: make(map[string]*atomic.Uint64),
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv.ctx = ctx
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
	if _bgp == nil {
		return fmt.Errorf("bgp: not initialized")
	}
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
						_bgp.L().Warn().
							Str("op", "withdraw").
							Str("ip", ip).
							Str("router_id", _bgp.id.String()).
							Strs("peers", _bgp.peers).
							Err(e).
							Msg("BGP withdraw failed")
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
