package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestInit_DnsTimeout verifies the flat Dns.Timeout field parsing, the 5s
// default for absent/non-positive values, and the 3s-30s range enforcement.
func TestInit_DnsTimeout(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		want    time.Duration
	}{
		{
			name: "absent key defaults to 5s",
			yaml: "Dns: {}\n",
			want: 5 * time.Second,
		},
		{
			name: "explicit 5s",
			yaml: "Dns:\n  Timeout: 5s\n",
			want: 5 * time.Second,
		},
		{
			name: "lower boundary 3s accepted",
			yaml: "Dns:\n  Timeout: 3s\n",
			want: 3 * time.Second,
		},
		{
			name: "upper boundary 30s accepted",
			yaml: "Dns:\n  Timeout: 30s\n",
			want: 30 * time.Second,
		},
		{
			name:    "below range 2s rejected",
			yaml:    "Dns:\n  Timeout: 2s\n",
			wantErr: true,
		},
		{
			name:    "above range 31s rejected",
			yaml:    "Dns:\n  Timeout: 31s\n",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "appsettings.yml")
			if err := os.WriteFile(p, []byte(tc.yaml), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			cfg, err := Init(p)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (parsed timeout: %v)", cfg.Dns.Timeout)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Dns.Timeout != tc.want {
				t.Errorf("Timeout = %v, want %v", cfg.Dns.Timeout, tc.want)
			}
		})
	}
}