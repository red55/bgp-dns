package bgp

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	bgpapi "github.com/osrg/gobgp/v3/api"
	bgpsrv "github.com/osrg/gobgp/v3/pkg/server"
	"github.com/red55/bgp-dns-peer/internal/cfg"
	"github.com/red55/bgp-dns-peer/internal/dns"
	"github.com/red55/bgp-dns-peer/internal/log"
	"github.com/red55/bgp-dns-peer/internal/utils"
	"net"
	"slices"
)

var (
	server *bgpsrv.BgpServer
)

func init() {
	chanRefresher = make(chan *bgpOp)

	server = bgpsrv.NewBgpServer(bgpsrv.LoggerOption(NewZapLogrusOverride()))
	go server.Serve()
}

func onConfigChange() {
	peers := make([]*bgpapi.Peer, 0, len(cfg.AppCfg.Bgp.Peers))
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
			Asn:             cfg.AppCfg.Bgp.Asn,
			RouterId:        cfg.AppCfg.Bgp.Id,
			ListenAddresses: []string{cfg.AppCfg.Bgp.Listen.IP.String()},
			ListenPort:      int32(cfg.AppCfg.Bgp.Listen.Port),
		},
	}); e != nil {
		log.L().Panic("Failed to start bgp server %v", e)
	}
	for _, peer := range cfg.AppCfg.Bgp.Peers {
		var pol *bgpapi.ApplyPolicy

		if peer.Asn == cfg.AppCfg.Bgp.Asn {
			pol = &bgpapi.ApplyPolicy{
				ImportPolicy: &bgpapi.PolicyAssignment{
					DefaultAction: bgpapi.RouteAction_ACCEPT,
					Direction:     bgpapi.PolicyDirection_IMPORT,
				},
				ExportPolicy: &bgpapi.PolicyAssignment{
					DefaultAction: bgpapi.RouteAction_ACCEPT,
					Direction:     bgpapi.PolicyDirection_EXPORT,
				},
			}
		} else {
			pol = &bgpapi.ApplyPolicy{
				ImportPolicy: &bgpapi.PolicyAssignment{
					DefaultAction: bgpapi.RouteAction_ACCEPT,
					Direction:     bgpapi.PolicyDirection_IMPORT,
				},
				ExportPolicy: &bgpapi.PolicyAssignment{
					Direction:     bgpapi.PolicyDirection_EXPORT,
					DefaultAction: bgpapi.RouteAction_REJECT,
				},
			}
		}
		if len(peer.Communities) > 0 {
			portBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(portBytes, uint64(peer.Addr.Port))
			m5 := md5.Sum(append(peer.Addr.IP, portBytes...))
			hx := hex.EncodeToString(m5[:])
			name := fmt.Sprintf("%s_export_pol_1", hx)

			if e := server.AddStatement(context.Background(), &bgpapi.AddStatementRequest{
				Statement: &bgpapi.Statement{
					Name: fmt.Sprintf("%s_stmt_1", name),
					Actions: &bgpapi.Actions{
						RouteAction: bgpapi.RouteAction_ACCEPT,
						Community: &bgpapi.CommunityAction{
							Type:        bgpapi.CommunityAction_ADD,
							Communities: peer.Communities,
						},
					},
				},
			}); e != nil {
				log.L().Panic("Failed to add statement %v", e)
			}

			policy := bgpapi.AddPolicyRequest{
				Policy: &bgpapi.Policy{
					Name: name,
					Statements: []*bgpapi.Statement{
						{
							Conditions: &bgpapi.Conditions{
								AfiSafiIn: []*bgpapi.Family{v4Family},
							},
							Name: fmt.Sprintf("%s_stmt_1", name),
						},
					},
				},
				ReferExistingStatements: true,
			}
			if e := server.AddPolicy(context.Background(), &policy); e != nil {
				log.L().Panic("Failed to add policy %v", e)
			}

			pol.ExportPolicy.Policies = []*bgpapi.Policy{
				{
					Name: name,
				},
			}
		}
		if e := server.AddPeer(context.Background(), &bgpapi.AddPeerRequest{
			Peer: &bgpapi.Peer{
				ApplyPolicy: pol,
				Conf: &bgpapi.PeerConf{
					NeighborAddress: peer.Addr.IP.String(),
					PeerAsn:         peer.Asn,
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

}

func Init() {
	_ = cfg.RegisterConfigChangeHandler(onConfigChange)
	_ = dns.RegisterDnsCallback(onDnsResolved)

	onConfigChange()

	go loop(chanRefresher)
}
func alreadyInBgp(prefixes []string) (bool, error) {
	// Assume all prefixes are /32
	prfxs := make([]string, len(prefixes))
	for i, prefix := range prefixes {
		prfxs[i] = fmt.Sprintf("%s/32", prefix)
	}
	filter := make([]*bgpapi.TableLookupPrefix, len(prefixes))
	for i, p := range prfxs {
		filter[i] = &bgpapi.TableLookupPrefix{
			Prefix: p,
			Type:   bgpapi.TableLookupPrefix_EXACT,
		}
	}

	found := false
	e := server.ListPath(context.Background(), &bgpapi.ListPathRequest{
		Family:   v4Family,
		Prefixes: filter,
		SortType: bgpapi.ListPathRequest_PREFIX,
	}, func(dst *bgpapi.Destination) {
		idx := slices.Index(prfxs, dst.Prefix)
		if idx >= 0 {
			found = true
		}
	})

	return found, e
}
func onDnsResolved(fqdn string, previps, ips []string) {
	var gone = utils.Difference(previps, ips)
	var arrived = utils.Difference(ips, previps)

	//No changes in IPs
	if gone == nil && arrived == nil {
		b, e := alreadyInBgp(ips)
		if e != nil {
			log.L().Error("Failed search in BGP tables: %v", e)
		}
		if b {
			return
		} else {
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
	log.L().Infof("Bgp->Deinit()")
	chanRefresher <- &bgpOp{
		op: opQuit,
	}
	close(chanRefresher)

	server.Stop()
}
