package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInit_BgpAuthPassword verifies optional BGP peer authentication parsing
// (SEC-04): an absent AuthPassword never produces an error and stays "",
// while an explicit value round-trips through viper decode verbatim.
func TestInit_BgpAuthPassword(t *testing.T) {
	const baseYaml = `
Bgp:
  Asn: 65001
  Id: 127.0.0.1
  Listen:
    Ip: 0.0.0.0
    Port: 8179
  Peers:
    - Asn: 65002
      Addressess:
        Ip: 127.0.0.2
        Port: 179
`
	tests := []struct {
		name string
		extra string // appended under the peer entry
		want string
	}{
		{
			name:  "absent key defaults to empty",
			extra: "",
			want:  "",
		},
		{
			name:  "explicit value preserved",
			extra: "\n      AuthPassword: s3cr3t-key",
			want:  "s3cr3t-key",
		},
		{
			name:  "explicit empty string stays empty",
			extra: `\n      AuthPassword: ""`,
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "appsettings.yml")
			if err := os.WriteFile(p, []byte(baseYaml+tc.extra), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			cfg, err := Init(p)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cfg.Bgp.Peers) != 1 {
				t.Fatalf("expected 1 peer, got %d", len(cfg.Bgp.Peers))
			}
			if got := cfg.Bgp.Peers[0].AuthPassword; got != tc.want {
				t.Errorf("AuthPassword = %q, want %q", got, tc.want)
			}
		})
	}
}
