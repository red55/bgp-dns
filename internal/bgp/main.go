package bgp

import (
	"context"
	bgpapi "github.com/osrg/gobgp/v3/api"
	bgpsrv "github.com/osrg/gobgp/v3/pkg/server"
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/dns"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/utils"
	"net"
	"sync"
)

var (
	server *bgpsrv.BgpServer
)
var wg sync.WaitGroup

func onConfigChange() {
	peers := make([]*bgpapi.Peer, 0, len(cfg.AppCfg.Routing().Bgp().Peers()))
	if e := server.ListPeer(context.Background(), &bgpapi.ListPeerRequest{}, func(peer *bgpapi.Peer) {
		peers = append(peers, peer)
	}); e != nil {
		log.L().Errorf("Error listing peers: %v", e)
	}
	if len(peers) > 0 {
		for _, peer := range peers {
			_ = server.DeletePeer(context.Background(), &bgpapi.DeletePeerRequest{
				Address: peer.Conf.NeighborAddress,
			})
		}
	}
	if e := server.StopBgp(context.Background(), &bgpapi.StopBgpRequest{}); e != nil {
		log.L().Panic("Failed to stop bgp server %v", e)
	}

	if e := server.StartBgp(context.Background(), &bgpapi.StartBgpRequest{
		Global: &bgpapi.Global{
			Asn:             cfg.AppCfg.Routing().Bgp().Asn(),
			RouterId:        cfg.AppCfg.Routing().Bgp().Id(),
			ListenAddresses: []string{cfg.AppCfg.Routing().Bgp().Listen().IP.String()},
			ListenPort:      int32(cfg.AppCfg.Routing().Bgp().Listen().Port),
			ApplyPolicy: &bgpapi.ApplyPolicy{
				ExportPolicy: &bgpapi.PolicyAssignment{
					DefaultAction: bgpapi.RouteAction_ACCEPT,
				},
			},
		},
	}); e != nil {
		log.L().Panic("Failed to start bgp server %v", e)
	}
	policy := bgpapi.AddPolicyRequest{
		Policy: &bgpapi.Policy{
			Name: "global-out",
			Statements: []*bgpapi.Statement{
				{
					Actions: &bgpapi.Actions{
						RouteAction: bgpapi.RouteAction_ACCEPT,
					},
				},
			},
		},
		ReferExistingStatements: false,
	}

	if len(cfg.AppCfg.Routing().Bgp().Communities()) > 0 {
		policy.Policy.Statements[0].Actions.Community = &bgpapi.CommunityAction{
			Type:        bgpapi.CommunityAction_ADD,
			Communities: cfg.AppCfg.Routing().Bgp().Communities(),
		}
	}

	if e := server.AddPolicy(context.Background(), &policy); e != nil {
		log.L().Panicf("Failed to add policy %v", e)
	}

	if e := server.SetPolicyAssignment(context.Background(), &bgpapi.SetPolicyAssignmentRequest{
		Assignment: &bgpapi.PolicyAssignment{
			Name:      "global",
			Direction: bgpapi.PolicyDirection_EXPORT,
			Policies: []*bgpapi.Policy{
				{
					Name: "global-out",
				},
			},
			DefaultAction: bgpapi.RouteAction_ACCEPT,
		},
	}); e != nil {
		log.L().Warnf("Failed to set global policy assignment: %v", e)
	}

	if e := server.WatchEvent(context.Background(), &bgpapi.WatchEventRequest{
		Table: &bgpapi.WatchEventRequest_Table{
			Filters: []*bgpapi.WatchEventRequest_Table_Filter{
				{
					Type: bgpapi.WatchEventRequest_Table_Filter_BEST,
				},
			},
		},
	}, func(r *bgpapi.WatchEventResponse) {
		onBgpEvent(r)
	}); e != nil {
		log.L().Warnf("Failed to watch table %v", e)
	}

	for _, peer := range cfg.AppCfg.Routing().Bgp().Peers() {
		var pol *bgpapi.ApplyPolicy

		if peer.Asn() == cfg.AppCfg.Routing().Bgp().Asn() {
			pol = &bgpapi.ApplyPolicy{
				ImportPolicy: &bgpapi.PolicyAssignment{
					Direction:     bgpapi.PolicyDirection_IMPORT,
					DefaultAction: bgpapi.RouteAction_ACCEPT,
				},
				ExportPolicy: &bgpapi.PolicyAssignment{
					Direction:     bgpapi.PolicyDirection_EXPORT,
					DefaultAction: bgpapi.RouteAction_ACCEPT,
				},
			}
		} else {
			pol = &bgpapi.ApplyPolicy{
				ImportPolicy: &bgpapi.PolicyAssignment{
					Direction:     bgpapi.PolicyDirection_IMPORT,
					DefaultAction: bgpapi.RouteAction_ACCEPT,
				},
				ExportPolicy: &bgpapi.PolicyAssignment{
					DefaultAction: bgpapi.RouteAction_REJECT,
					Direction:     bgpapi.PolicyDirection_EXPORT,
				},
			}
		}

		if e := server.AddPeer(context.Background(), &bgpapi.AddPeerRequest{
			Peer: &bgpapi.Peer{
				ApplyPolicy: pol,
				Conf: &bgpapi.PeerConf{
					NeighborAddress: peer.Addr().IP.String(),
					PeerAsn:         peer.Asn(),
				},
				EbgpMultihop: &bgpapi.EbgpMultihop{
					Enabled:     peer.Multihop(),
					MultihopTtl: 254,
				},
				Timers: &bgpapi.Timers{
					Config: &bgpapi.TimersConfig{
						HoldTime: 240,
					},
				},
				Transport: &bgpapi.Transport{
					PassiveMode:  peer.PassiveMode(),
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
			log.L().Fatalf("Failed to add peer: %v", e)
		}
	}
}

func Init() {
	server = bgpsrv.NewBgpServer(bgpsrv.LoggerOption(NewZapLogrusOverride()))
	_ = server.SetLogLevel(context.Background(), &bgpapi.SetLogLevelRequest{
		Level: zapLogLevelToBgp(cfg.AppCfg.Log().Level),
	})

	go server.Serve()

	_ = cfg.RegisterConfigChangeHandler(onConfigChange)
	_ = dns.RegisterDnsCallback(onDnsResolved)

	onConfigChange()

	go loop(_cmdChannel)

}
func Add(ip net.IP, length uint32) {
	_cmdChannel <- &bgpOp{
		op: opAdd,
		prefix: &bgpapi.IPAddressPrefix{
			PrefixLen: length,
			Prefix:    ip.String(),
		},
	}
}
func Remove(ip net.IP) {
	_cmdChannel <- &bgpOp{
		op: opRemove,
		prefix: &bgpapi.IPAddressPrefix{
			Prefix:    ip.String(),
			PrefixLen: 32,
		},
	}
}

func knownNlri(prefixes []string) (bool, error) {
	// TODO: Assume all prefixes are /32
	prfxs := make([]*bgpapi.IPAddressPrefix, len(prefixes))
	for i, prefix := range prefixes {
		prfxs[i] = &bgpapi.IPAddressPrefix{
			PrefixLen: 32,
			Prefix:    prefix,
		}
	}

	p, e := find(prfxs)
	return p != nil, e
}

func onDnsResolved(fqdn string, prevIps, ips []string) {
	_ = fqdn
	var gone = utils.Difference(prevIps, ips)
	var arrived = utils.Difference(ips, prevIps)

	//No changes in IPs
	if gone == nil && arrived == nil {
		b, e := knownNlri(ips)
		if e != nil {
			log.L().Error("Failed search in BGP tables: %v", e)
		}
		if b {
			log.L().Debugf("Prefixes already in global table")
			return
		} else {
			log.L().Debugf("Will add prefixes to global table")
			arrived = ips
		}
	}

	for _, prev := range gone {
		Remove(net.ParseIP(prev))
	}
	for _, ip := range arrived {
		Add(net.ParseIP(ip), 32)
	}

}

func Deinit() {
	log.L().Infof("Bgp->Deinit(): Enter")
	defer log.L().Infof("Bgp->Deinit(): Done")

	server.Stop()

	_cmdChannel <- &bgpOp{
		op: opQuit,
	}
	wg.Wait()

}
