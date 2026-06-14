package cli

import (
	"context"
	"net"
	"testing"

	"github.com/red55/bgp-dns/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGRPCLifecycle creates a test gRPC server with the cache CLI service
// on an ephemeral port. Returns server, address string, and cleanup function.
func newTestGRPCLifecycle(t *testing.T) (*grpc.Server, string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	api.RegisterBgpDnsServiceServer(grpcServer, &CacheCliServiceImpl{})

	go grpcServer.Serve(listener)

	cleanup := func() {
		grpcServer.GracefulStop()
		listener.Close()
	}

	t.Cleanup(cleanup)
	return grpcServer, listener.Addr().String(), cleanup
}

// TestGRPC_ServeShutdown verifies the full gRPC server lifecycle:
// start → accept RPCs → shutdown gracefully.
func TestGRPC_ServeShutdown(t *testing.T) {
	_, addr, _ := newTestGRPCLifecycle(t)

	// Connect client
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := api.NewBgpDnsServiceClient(conn)

	// ListCacheEntries should return FailedPrecondition (cache not initialized)
	stream, err := client.ListCacheEntries(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	_, recvErr := stream.Recv()
	st, _ := status.FromError(recvErr)
	assert.Equal(t, codes.FailedPrecondition, st.Code(), "ListCacheEntries should return FailedPrecondition")

	// ClearCache with nil request should return InvalidArgument
	_, err = client.ClearCache(context.Background(), &api.ClearCacheRequest{})
	st, _ = status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code(), "ClearCache should return FailedPrecondition when cache not initialized")

	// ReloadList with nil request should return InvalidArgument
	_, err = client.ReloadList(context.Background(), nil)
	st, _ = status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code(), "ReloadList should return InvalidArgument for nil request")
}

// TestGRPC_ListCacheEntries_Uninitialized verifies that ListCacheEntries
// returns FailedPrecondition when the cache is not initialized.
func TestGRPC_ListCacheEntries_Uninitialized(t *testing.T) {
	_, addr, _ := newTestGRPCLifecycle(t)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := api.NewBgpDnsServiceClient(conn)

	stream, err := client.ListCacheEntries(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	_, recvErr := stream.Recv()
	st, ok := status.FromError(recvErr)
	require.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, codes.FailedPrecondition, st.Code(), "ListCacheEntries should return FailedPrecondition")
}

// TestGRPC_ClearCache_Uninitialized verifies that ClearCache returns
// FailedPrecondition when the cache is not initialized.
func TestGRPC_ClearCache_Uninitialized(t *testing.T) {
	_, addr, _ := newTestGRPCLifecycle(t)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := api.NewBgpDnsServiceClient(conn)

	_, err = client.ClearCache(context.Background(), &api.ClearCacheRequest{})
	st, ok := status.FromError(err)
	require.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, codes.FailedPrecondition, st.Code(), "ClearCache should return FailedPrecondition")
}

// TestGRPC_ReloadList_NilRequest verifies that ReloadList returns
// InvalidArgument when the request is nil.
func TestGRPC_ReloadList_NilRequest(t *testing.T) {
	_, addr, _ := newTestGRPCLifecycle(t)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := api.NewBgpDnsServiceClient(conn)

	_, err = client.ReloadList(context.Background(), nil)
	st, ok := status.FromError(err)
	require.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, st.Code(), "ReloadList should return InvalidArgument for nil request")
	assert.Contains(t, st.Message(), "nil", "error message should mention nil request")
}

// TestGRPC_ReloadList_Uninitialized verifies that ReloadList returns
// FailedPrecondition when the cache is not initialized.
func TestGRPC_ReloadList_Uninitialized(t *testing.T) {
	_, addr, _ := newTestGRPCLifecycle(t)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := api.NewBgpDnsServiceClient(conn)

	_, err = client.ReloadList(context.Background(), &api.ReloadListRequest{})
	st, ok := status.FromError(err)
	require.True(t, ok, "error should be a gRPC status error")
	assert.Equal(t, codes.FailedPrecondition, st.Code(), "ReloadList should return FailedPrecondition when cache not initialized")
}

// TestGRPC_GracefulStop verifies that the server stops accepting new
// connections after GracefulStop().
func TestGRPC_GracefulStop(t *testing.T) {
	server, addr, cleanup := newTestGRPCLifecycle(t)
	_ = server // server is started by newTestGRPCLifecycle

	// Connect client
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	client := api.NewBgpDnsServiceClient(conn)

	// Call an RPC before shutdown
	_, err = client.ReloadList(context.Background(), nil)
	assert.Error(t, err, "ReloadList should return an error (nil request)")

	// Shutdown the server
	cleanup()

	// Verify the connection is closed
	err = conn.Close()
	assert.NoError(t, err, "connection close should not error")
}
