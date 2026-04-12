package cli

import (
	"testing"

	"github.com/red55/bgp-dns/api"
)

func TestReloadList_NilRequest(t *testing.T) {
	s := &CacheCliServiceImpl{}
	_, err := s.ReloadList(nil, nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestReloadList_UninitializedCache(t *testing.T) {
	// Set a valid list file path — cache won't be initialized in unit tests
	_listFile = "/tmp/test-list.lst"

	s := &CacheCliServiceImpl{}
	_, err := s.ReloadList(nil, &api.ReloadListRequest{})

	// We expect an error because dns._cache is nil in unit tests
	// The error should be codes.Internal (from dns.ENotInitialized)
	if err == nil {
		t.Error("expected error when cache is not initialized")
	}
}
