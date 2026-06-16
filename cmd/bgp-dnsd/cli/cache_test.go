package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/red55/bgp-dns/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestReloadList_NilRequest(t *testing.T) {
	s := &CacheCliServiceImpl{}
	_, err := s.ReloadList(nil, nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestReloadList_UninitializedCache(t *testing.T) {
	// Create a temporary list file to avoid OS/environment-dependent paths
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-list.lst")
	if err := os.WriteFile(tmpFile, []byte{}, 0600); err != nil {
		t.Fatalf("failed to create temp list file: %v", err)
	}
	_listFile = tmpFile

	s := &CacheCliServiceImpl{}
	_, err := s.ReloadList(nil, &api.ReloadListRequest{})

	// We expect an error because dns._cache is nil in unit tests
	// The error should be codes.FailedPrecondition (from dns.ENotInitialized)
	if err == nil {
		t.Error("expected error when cache is not initialized")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Error("expected gRPC status error")
	}
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", st.Code())
	}
}
