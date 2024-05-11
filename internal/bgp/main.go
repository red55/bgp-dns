package bgp

import (
	"context"
	"net"
	"slices"
	"strconv"

	bgpapi "github.com/osrg/gobgp/v3/api"
	bgpsrv "github.com/osrg/gobgp/v3/pkg/server"
	"github.com/red55/bgp-dns/internal/cfg"
	"github.com/red55/bgp-dns/internal/dns"
	"github.com/red55/bgp-dns/internal/krt"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/red55/bgp-dns/internal/utils"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	server *bgpsrv.BgpServer
)

func init() {
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
	}

	if e := server.WatchEvent(context.Background(), &bgpapi.WatchEventRequest{
		//Peer: &bgpapi.WatchEventRequest_Peer{},
		Table: &bgpapi.WatchEventRequest_Table{
			Filters: []*bgpapi.WatchEventRequest_Table_Filter{
				{
					Type: bgpapi.WatchEventRequest_Table_Filter_BEST,
				},
			},
		},
	}, func(r *bgpapi.WatchEventResponse) {
		if p := r.GetPeer(); p != nil && p.Type == bgpapi.WatchEventResponse_PeerEvent_STATE {
			log.L().Infof("PeerEvent_STATE: %v", p)
		} else if t := r.GetTable(); t != nil {
			for _, p := range t.Paths {
				var cs = new(bgpapi.CommunitiesAttribute)
				var idx = slices.IndexFunc(p.Pattrs, func(a *anypb.Any) bool {
					return a.TypeUrl == "type.googleapis.com/apipb.CommunitiesAttribute"
				})

				if idx == -1 {
					log.L().Debugf("Skiping path %v as it doesn't have community attr", p)
					continue
				}

				if e := p.Pattrs[idx].UnmarshalTo(cs); e != nil {
					log.L().Panicf("Failed to unmarshall communities: %v", e)
				}

				comms := cfg.AppCfg.Routing().Kernel().Inject().Communities()
				communitiesMatched := false
				for _, c := range comms {
					comm, _ := strconv.Atoi(c)
					if slices.Index(cs.Communities, uint32(comm)) == -1 {
						communitiesMatched = true
						break
					}
				}

				if !communitiesMatched {
					log.L().Debugf("Skiping path %v as it's not marked with communities %v", p, comms)
					continue
				}

				var prefix = new(bgpapi.IPAddressPrefix)
				if e := p.Nlri.UnmarshalTo(prefix); e != nil {
					log.L().Panicf("Failed to unmarshal prefix: %v", e)
				}

				idx = slices.IndexFunc(p.Pattrs, func(a *anypb.Any) bool {
					return a.TypeUrl == "type.googleapis.com/apipb.NextHopAttribute"
				})

				if idx == -1 {
					log.L().Warnf("Next hop attribute not found for prefix %v", prefix)
					return
				}

				var nha = new(bgpapi.NextHopAttribute)
				if e := p.Pattrs[idx].UnmarshalTo(nha); e != nil {
					log.L().Panicf("Failed to unmarshal NextHopAttribute: %v", e)
				}
				var n = &net.IPNet{
					IP:   net.ParseIP(prefix.Prefix),
					Mask: net.CIDRMask(int(prefix.PrefixLen), int(prefix.PrefixLen)),
				}

				var m = cfg.AppCfg.Routing().Kernel().Inject().Metric()
				var nh = net.ParseIP(nha.NextHop)
				if p.IsWithdraw {
					krt.Withdraw(n, nh, m)
				} else /*if p.Best*/ {
					krt.Advance(n, nh, m)
				}

				log.L().Infof("Path: %v", p)
			}
		}
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
				Transport: &bgpapi.Transport{
					PassiveMode: peer.PassiveMode(),
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
			log.L().Panicf("Failed to add peer: %v", e)
		}
	}
}

func Init() {
	_ = cfg.RegisterConfigChangeHandler(onConfigChange)
	_ = dns.RegisterDnsCallback(onDnsResolved)

	onConfigChange()

	go loop(cmdChannel)
}
func Add(ip net.IP, length uint32) {
	cmdChannel <- &bgpOp{
		op: opAdd,
		prefix: &bgpapi.IPAddressPrefix{
			PrefixLen: length,
			Prefix:    ip.String(),
		},
	}
}
func Remove(ip net.IP) {
	cmdChannel <- &bgpOp{
		op: opRemove,
		prefix: &bgpapi.IPAddressPrefix{
			Prefix: ip.String(),
		},
	}
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

func Deinit() {
	log.L().Infof("Rt->Deinit()")
	cmdChannel <- &bgpOp{
		op: opQuit,
	}
	close(cmdChannel)

	server.Stop()
}
