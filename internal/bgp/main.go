package bgp

import (
	"context"
	bgpapi "github.com/osrg/gobgp/v3/api"
	bgpsrv "github.com/osrg/gobgp/v3/pkg/server"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/dns"
	"github.com/red55/bgp-dns-peer/internal/log"
	"github.com/red55/bgp-dns-peer/internal/utils"
	"net"
)

var (
	server *bgpsrv.BgpServer
)

func init() {
	chanRefresher = make(chan *bgpOp)

	server = bgpsrv.NewBgpServer(bgpsrv.LoggerOption(NewZapLogrusOverride()))
	_ = server.SetLogLevel(context.Background(), &bgpapi.SetLogLevelRequest{
		Level: 0xFFFFFFF,
	})

	go server.Serve()
}

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
	} /*
		table := &bgpapi.WatchEventRequest_Table{
			Filters: []*bgpapi.WatchEventRequest_Table_Filter{
				{
					Type: bgpapi.WatchEventRequest_Table_Filter_ADJIN,
					Init: true,
				},
			},
		}
	*/
	if e := server.WatchEvent(context.Background(), &bgpapi.WatchEventRequest{
		Peer: &bgpapi.WatchEventRequest_Peer{},
		Table: &bgpapi.WatchEventRequest_Table{
			Filters: []*bgpapi.WatchEventRequest_Table_Filter{
				{
					Type: bgpapi.WatchEventRequest_Table_Filter_ADJIN,
				},
			},
		},
	}, func(event *bgpapi.WatchEventResponse) {
		log.L().Debug(event)
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

				Transport: &bgpapi.Transport{
					PassiveMode: true,
				},
				RouteServer: &bgpapi.RouteServer{
					RouteServerClient: false,
					SecondaryRoute:    false,
				},

				AfiSafis: []*bgpapi.AfiSafi{
					{
						Config: &bgpapi.AfiSafiConfig{
							Family:  v4Family,
							Enabled: true,
						},
					},
				},
			},
		}); e != nil {
			log.L().Panic("Failed to add peer %v", e)
		}
	}

	_ = server.ListPolicyAssignment(context.Background(), &bgpapi.ListPolicyAssignmentRequest{},
		func(pa *bgpapi.PolicyAssignment) {
			log.L().Debug(pa)
		})
}

func Init() {
	_ = cfg.RegisterConfigChangeHandler(onConfigChange)
	_ = dns.RegisterDnsCallback(onDnsResolved)

	onConfigChange()

	go loop(chanRefresher)
}
func knownNlri(prefixes []string) (bool, error) {
	// Assume all prefixes are /32
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
func onDnsResolved(fqdn string, previps, ips []string) {
	_ = fqdn
	var gone = utils.Difference(previps, ips)
	var arrived = utils.Difference(ips, previps)

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
func loop(ch chan *bgpOp) {
	for op := range ch {
		switch op.op {
		case opAdd:
			if e := add(op.prefix); e != nil {
				log.L().Errorf("Failed to add prefix: %v", e)
			}
		case opRemove:
			if e := remove(op.prefix); e != nil {
				log.L().Errorf("Failed to remove prefix: %v", e)
			}
		case opQuit:
			return
		}
	}
}

func Deinit() {
	log.L().Infof("Rt->Deinit()")
	chanRefresher <- &bgpOp{
		op: opQuit,
	}
	close(chanRefresher)

	server.Stop()
}
