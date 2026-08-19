package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/red55/bgp-dns/api"
	"github.com/red55/bgp-dns/internal/app"
	"github.com/red55/bgp-dns/internal/dns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type CacheCliServiceImpl struct {
	api.BgpDnsServiceServer
}

var (
	_cancel    func()
	_listener  net.Listener
	_listFile  string
)

func Serve(_app *app.Application, listFile string) (e error) {
	_listFile = listFile
	var target string
	var proto string

	if strings.HasPrefix(_app.Flags.Target, "unix://") || strings.HasPrefix(_app.Flags.Target, "/") {
		target = strings.TrimPrefix(_app.Flags.Target, "unix://")
		proto = "unix"
		if _, e = os.Stat(target); !os.IsNotExist(e) {
			if e := os.Remove(target); e != nil {
				_app.L().Fatal().Err(e)
				return e
			}
		}
		/*
			if e = os.MkdirAll(target, 0700); !os.IsExist(e) {
				_app.L().Fatal().Err(e)
				return e
			}*/
	} else {
		target = _app.Flags.Target
		proto = "tcp"
	}

	if _listener, e = net.Listen(proto, target); e != nil {
		_app.L().Fatal().Err(e).Msgf("CLI Service: Failed to bind to %s", target)
		return e
	}

	// SEC-01: net.Listen creates the socket honoring the process umask; force
	// owner-only access before the server accepts any connection. Unix targets
	// only — tcp listeners keep their existing behavior.
	if proto == "unix" {
		if e = os.Chmod(target, 0o600); e != nil {
			_app.L().Fatal().Err(e).Msgf("CLI Service: Failed to set mode 0600 on %s", target)
			return e
		}
	}

	// SEC-02: unified validation gate. ONE shared audit handler is chained for
	// both the unary and the stream paths: every RPC emits exactly one
	// structured event (method + elapsed time) and is forwarded untouched. It
	// makes no request-shape decisions — gRPC serializes a nil request into an
	// empty message that cannot be distinguished here, so nil-rejection stays
	// authoritative in the per-handler checks below.
	auditRPC := func(method string, start time.Time) {
		_app.L().Debug().Str("rpc", method).Dur("elapsed", time.Since(start)).Msg("CLI Service: gRPC call")
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			start := time.Now()
			resp, err := handler(ctx, req)
			auditRPC(info.FullMethod, start)
			return resp, err
		}),
		grpc.ChainStreamInterceptor(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			start := time.Now()
			err := handler(srv, ss)
			auditRPC(info.FullMethod, start)
			return err
		}),
	)
	cliServer := &CacheCliServiceImpl{}
	api.RegisterBgpDnsServiceServer(grpcServer, cliServer)
	_app.L().Info().Msgf("CLI Service: gRPC server started on %s", target)

	_cancel = func() {
		grpcServer.GracefulStop()
	}

	go func(l net.Listener) {
		// A stopped server reports ErrServerStopped from Serve — that is the
		// normal shutdown path (GracefulStop), not a startup failure.
		if e = grpcServer.Serve(l); e != nil && !errors.Is(e, grpc.ErrServerStopped) {
			_app.L().Fatal().Err(e).Msgf("CLI Service: Failed to start gRPC server on %s", target)
		}

	}(_listener)

	return e
}

func (CacheCliServiceImpl) ListCacheEntries(unused *emptypb.Empty, stream grpc.ServerStreamingServer[api.ListCacheEntriesResponse]) (e error) {
	if stream == nil {
		return status.Error(codes.InvalidArgument, "stream cannot be nil")
	}
	err := dns.DumpCache(func(qtype uint16, fqdn string, fails uint64, ips []string, ttl time.Duration, expiration time.Time, gen uint64) error {
		resp := api.ListCacheEntriesResponse{
			Type:       dns.QTypeToString[qtype],
			Fqdn:       fqdn,
			Addr:       ips,
			Generation: fmt.Sprintf("%d", gen),
			Ttl:        int64(ttl),
			Expiration: expiration.UnixMilli(),
			Fails:      fails,
		}
		return stream.Send(&resp)
	})
	if err != nil {
		if errors.Is(err, dns.ENotInitialized) {
			return status.Errorf(codes.FailedPrecondition, "failed to list cache: %v", err)
		}
		return status.Errorf(codes.Internal, "failed to list cache: %v", err)
	}
	return nil
}

func (CacheCliServiceImpl) ClearCache(ctx context.Context, req *api.ClearCacheRequest) (*api.ClearCacheResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	count, err := dns.ClearCache()
	if err != nil {
		if errors.Is(err, dns.ENotInitialized) {
			return nil, status.Errorf(codes.FailedPrecondition, "failed to clear cache: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to clear cache: %v", err)
	}
	return &api.ClearCacheResponse{
		ClearedCount: count,
	}, nil
}

func (CacheCliServiceImpl) ReloadList(ctx context.Context, req *api.ReloadListRequest) (*api.ReloadListResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if e := dns.Load(_listFile); e != nil {
		if errors.Is(e, dns.ENotInitialized) {
			return nil, status.Errorf(codes.FailedPrecondition, "failed to reload list: %v", e)
		}
		return nil, status.Errorf(codes.Internal, "failed to reload list: %v", e)
	}
	return &api.ReloadListResponse{}, nil
}

func Shutdown(_app *app.Application) error {
	if _cancel != nil {
		_cancel()
		_cancel = nil
		_app.L().Info().Msg("CLI Service: gRPC server shutdown completed")
	}
	return nil
}
