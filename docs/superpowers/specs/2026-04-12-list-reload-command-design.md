# Design: `bgp-dnsctl list reload` Command

## Overview

Add a CLI command `bgp-dnsctl list reload` that triggers the daemon to reload the configured domain list file (from `cfg.Dns.List.File`). This is the same operation the file watcher performs when the file changes, but triggered manually via CLI.

## Architecture

### Components

```
bgp-dnsctl list reload
    │
    ▼ (gRPC call)
BgpDnsService.ReloadList (empty request)
    │
    ▼
CacheCliServiceImpl.ReloadList()
    │
    ▼
dns.Load(cfg.Dns.List.File)
    │
    ▼
cache.load(fn) — reads file, registers domains, evicts stale entries
```

### Phase 1: Protobuf Definition

**File:** `proto/api/bgp-dns.proto`

Add to `BgpDnsService`:
```protobuf
rpc ReloadList(ReloadListRequest) returns (ReloadListResponse);
```

Add messages:
```protobuf
message ReloadListRequest {}
message ReloadListResponse {}
```

Run `make pb` to regenerate Go code.

### Phase 2: gRPC Server Handler

**File:** `cmd/bgp-dnsd/cli/cache.go`

Add method on `CacheCliServiceImpl`:
```go
func (s *CacheCliServiceImpl) ReloadList(ctx context.Context, req *api.ReloadListRequest) (*api.ReloadListResponse, error) {
    if e := dns.Load(cfg.Dns.List.File); e != nil {
        return nil, e
    }
    return &api.ReloadListResponse{}, nil
}
```

This calls the existing `dns.Load()` function which:
1. Opens the domain list file
2. Increments the cache generation
3. Registers each domain (resolves and announces to BGP)
4. Evicts entries from the previous generation

### Phase 3: CLI Command

**File:** `cmd/bgp-dnsctl/commands/list.go` (new)

Create a `list` command group with a `reload` subcommand:
```go
func newListCmd(_app *app.Application, client *api.BgpDnsServiceClient) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "list",
        Short: "Domain list management commands",
        Run: func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
    }
    cmd.AddCommand(newListReloadCmd(_app, client))
    return cmd
}

func newListReloadCmd(_app *app.Application, client *api.BgpDnsServiceClient) *cobra.Command {
    return &cobra.Command{
        Use:   "reload",
        Short: "Reload the domain list file",
        RunE: func(cmd *cobra.Command, args []string) error {
            _, err := (*client).ReloadList(cmd.Context(), &api.ReloadListRequest{})
            return err
        },
    }
}
```

**File:** `cmd/bgp-dnsctl/commands/root.go`

Add `newListCmd(_app, &client)` to the `cmd.AddCommand(...)` call.

### Phase 4: Unit Tests

- `internal/dns/cache_test.go` — Test `cache.load()` with temporary domain list files, verifying domains are registered and stale entries are evicted
- `cmd/bgp-dnsd/cli/cache_test.go` — Test gRPC `ReloadList` handler with mock DNS layer
- `cmd/bgp-dnsctl/commands/list_test.go` — Test CLI command structure and gRPC call

## Files to Modify

| File | Action |
|------|--------|
| `proto/api/bgp-dns.proto` | Add `ReloadList` RPC and messages |
| `api/bgp-dns.pb.go` | Regenerated via `make pb` |
| `api/bgp-dns_grpc.pb.go` | Regenerated via `make pb` |
| `cmd/bgp-dnsd/cli/cache.go` | Add `ReloadList` method |
| `cmd/bgp-dnsctl/commands/list.go` | New file — CLI commands |
| `cmd/bgp-dnsctl/commands/root.go` | Register `list` command |
| `internal/dns/cache_test.go` | Tests for cache.load |
| `cmd/bgp-dnsd/cli/cache_test.go` | Tests for gRPC handler |
| `cmd/bgp-dnsctl/commands/list_test.go` | Tests for CLI |

## Design Decisions

1. **Empty request/response** — No file path override; always reloads the configured file. Keeps the API simple and consistent with how the file watcher works.

2. **Reuse existing `dns.Load()`** — The generation-based reload is already idempotent and handles add/remove correctly. No new cache logic needed.

3. **Separate `list` command group** — Semantically separates domain list management from cache operations, even though both share the same gRPC service.
