package bgp

import "testing"

// TestBuildPeerSpec_AuthPassword pins the SEC-04 config→spec mapping: a set
// password propagates into PeerConf verbatim; an unset one stays "" so GoBGP
// leaves TCP-MD5 off (pre-phase behavior). Guard-rail assertions on untouched
// identity fields catch mis-extraction regressions from buildPeerSpec.
func TestBuildPeerSpec_AuthPassword(t *testing.T) {
	tests := []struct {
		name           string
		in             peerSpecInput
		wantAuth       string
		wantNeighbor   string
		wantAsn        uint32
	}{
		{
			name: "password propagated",
			in: peerSpecInput{
				Asn:                65001,
				NeighborAddress:    "127.0.0.2",
				Multihop:           true,
				PassiveMode:        false,
				ListenLocalAddress: "127.0.0.1",
				AuthPassword:       "k-123",
			},
			wantAuth:     "k-123",
			wantNeighbor: "127.0.0.2",
			wantAsn:      65001,
		},
		{
			name: "unset stays empty",
			in: peerSpecInput{
				Asn:                65001,
				NeighborAddress:    "127.0.0.2",
				Multihop:           false,
				PassiveMode:        false,
				ListenLocalAddress: "127.0.0.1",
			},
			wantAuth:     "",
			wantNeighbor: "127.0.0.2",
			wantAsn:      65001,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := buildPeerSpec(tc.in)

			if got := spec.Conf.AuthPassword; got != tc.wantAuth {
				t.Errorf("AuthPassword = %q, want %q", got, tc.wantAuth)
			}
			// Guard rails: pre-existing identity fields survive extraction verbatim.
			if got := spec.Conf.NeighborAddress; got != tc.wantNeighbor {
				t.Errorf("NeighborAddress = %q, want %q", got, tc.wantNeighbor)
			}
			if got := spec.Conf.PeerAsn; got != tc.wantAsn {
				t.Errorf("PeerAsn = %d, want %d", got, tc.wantAsn)
			}
			if got := spec.Transport.LocalAddress; got != tc.in.ListenLocalAddress {
				t.Errorf("Transport.LocalAddress = %q, want %q", got, tc.in.ListenLocalAddress)
			}
			if got := spec.EbgpMultihop.Enabled; got != tc.in.Multihop {
				t.Errorf("EbgpMultihop.Enabled = %v, want %v", got, tc.in.Multihop)
			}
		})
	}
}
