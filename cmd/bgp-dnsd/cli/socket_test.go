package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/red55/bgp-dns/internal/app"
	"github.com/red55/bgp-dns/internal/log"
	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp builds a minimal *app.Application wired to a no-op logger so the
// test never depends on process-wide log initialization. Target is passed the
// way Serve() expects it (unix:// prefix).
func newTestApp(target string) *app.Application {
	nop := zerolog.Nop()
	return &app.Application{
		Log:   log.NewLog(&nop, "test-cli"),
		Flags: &app.GlobalFlags{Target: target},
	}
}

// serveUnixAndAssert0600 drives the real Serve() against a unix target under
// t.TempDir() (never the default /run paths) and asserts the socket ends up
// with permission bits 0600 exactly. preStale pre-creates a world-readable
// regular file at the socket path to pin the stale-socket removal path:
// Serve() must remove it, rebind, and still land on 0600.
//
// Serve-based tests must run sequentially (Serve clobbers the package globals
// _cancel/_listener/_listFile), so there is deliberately no t.Parallel() here.
// The globals are restored after the server is stopped.
func serveUnixAndAssert0600(t *testing.T, sockPath string, preStale bool) {
	t.Helper()

	if preStale {
		f, err := os.OpenFile(sockPath, os.O_CREATE|os.O_WRONLY, 0o644)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		// Force an exact 0644 stale file regardless of the runner's umask so
		// the scenario is deterministic (umask independence, Pitfall 1).
		require.NoError(t, os.Chmod(sockPath, 0o644))
	}

	a := newTestApp("unix://" + sockPath)

	// Save the globals Serve() overwrites; restore them in cleanup so later
	// tests in this package see a clean slate.
	prevCancel := _cancel
	prevListener := _listener
	prevListFile := _listFile
	t.Cleanup(func() {
		l := _listener
		_ = Shutdown(a) // GracefulStop; nils _cancel
		if l != nil {
			_ = l.Close()
		}
		_cancel = prevCancel
		_listener = prevListener
		_listFile = prevListFile
	})

	// A dummy, nonexistent list file is safe: Serve() only records the path
	// and touches it on a ReloadList RPC at the earliest, which this test
	// never sends.
	require.NoError(t, Serve(a, filepath.Join(t.TempDir(), "my.lst")))

	fi, err := os.Stat(sockPath)
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode()&os.ModeSocket, "expected a unix socket at %s", sockPath)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm(),
		"socket permission bits must be exactly 0600 (owner rw only)")
}

// TestServe_UnixSocketMode0600 pins SEC-01: cli.Serve() on a unix:// target
// yields a socket whose stat mode is exactly 0600, both on a fresh bind and
// after removing a stale pre-existing file.
func TestServe_UnixSocketMode0600(t *testing.T) {
	for _, tc := range []struct {
		name     string
		preStale bool
	}{
		{name: "fresh", preStale: false},
		{name: "stale", preStale: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			serveUnixAndAssert0600(t, filepath.Join(t.TempDir(), "bgp-dnsd.sock"), tc.preStale)
		})
	}
}
