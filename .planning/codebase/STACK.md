# Technology Stack

**Analysis Date:** 2026-06-13

## Languages

**Primary:**
- Go 1.24.4 - All application code, protobuf definitions, and build scripts

**Secondary:**
- YAML (v3) - Configuration files, CI workflows, protobuf lint config
- Shell (bash) - Makefile commands

## Runtime

**Environment:**
- Go 1.24.4 (from `go.mod`)

**Package Manager:**
- Go modules (go.sum with 132 dependency lines)
- Lockfile: Present (`go.sum`)

## Frameworks

**Core:**
- `github.com/miekg/dns` v1.1.67 - DNS protocol library (server, resolver, message handling)
- `github.com/osrg/gobgp/v3` v3.37.0 - BGP protocol implementation (server, peer management, route announcements)
- `github.com/spf13/cobra` v1.9.1 - CLI framework for both `bgp-dnsd` and `bgp-dnsctl`
- `github.com/spf13/viper` v1.20.1 - Configuration management (YAML parsing, env vars, decode hooks)
- `google.golang.org/grpc` v1.73.0 - gRPC for daemon-CLI communication (server streaming + unary RPCs)
- `google.golang.org/protobuf` v1.36.6 - Protocol buffers runtime

**Testing:**
- `github.com/stretchr/testify` v1.10.0 - Assertion library (used in `*_test.go` files)
- Go standard library `testing` package

**Build/Dev:**
- `buf` (v2 schema) - Protobuf code generation (`make pb` → `buf generate --path proto/api`)
- GoReleaser v2 (`.goreleaser.yaml`) - Release automation (ldflags injection, tar.gz archives)
- `go build` - Direct compilation via `Makefile`

## Key Dependencies

**Critical:**
- `github.com/miekg/dns` v1.1.67 - DNS server implementation, handles UDP listener, query routing, A/AAAA/HTTPS record parsing
- `github.com/osrg/gobgp/v3` v3.37.0 - BGP server, manages peer connections, route advertisement via gRPC API
- `github.com/bluele/gcache` v0.0.2 - LFU cache with eviction callbacks for DNS responses
- `github.com/fsnotify/fsnotify` - File system event watching for domain list file changes

**Infrastructure:**
- `github.com/rs/zerolog` v1.34.0 - Structured logging with console output
- `github.com/sourcegraph/conc` v0.3.0 - `iter.ForEach` for concurrent iteration in resolver setup
- `github.com/go-viper/mapstructure/v2` v2.3.0 - Viper's struct decoding (indirect)
- `github.com/k-sone/critbitgo` v1.4.0 - Crit-trie for DNS domain matching (indirect, via miekg/dns)

**Platform:**
- `github.com/vishvananda/netlink` v1.3.1 - Linux network namespace/link manipulation (indirect, via gobgp)
- `github.com/vishvananda/netns` v0.0.5 - Network namespace handling (indirect, via gobgp)

## Configuration

**Environment:**
- Viper reads YAML config file (default: `appsettings.yml`)
- Custom decode hooks for `net.IP` (trim quotes, parse) and `zerolog.Level` (string → level)
- Config struct: `AppCfg` with `Log`, `Bgp`, `Dns` sections
- Domain list file path is resolved to absolute path on load

**Key configs required:**
- `Bgp.Asn` — local AS number
- `Bgp.Id` — router ID (IP)
- `Bgp.Listen` — BGP server bind address
- `Bgp.Peers[].Asn, .Address` — peer configuration
- `Dns.Listen` — DNS server bind address
- `Dns.Resolvers` — upstream DNS servers for general queries
- `Dns.List.File` — domain list file path
- `Dns.List.Resolvers` — upstream DNS servers for listed domains only
- `Dns.Cache.MaxEntries` — cache size limit
- `Dns.Cache.MinTtl` — minimum TTL in seconds
- `Log.Level` — zerolog level string

**Build:**
- `Makefile` targets: `all`, `pb`, `bgp-dnsd`, `bgp-dnsctl`, `clean`, `clean_bp`
- `buf generate` for protobuf code generation
- GoReleaser for release builds with ldflags version injection

## Platform Requirements

**Development:**
- Go 1.24.4+ toolchain
- `buf` CLI for protobuf generation
- Linux kernel (BGP server requires root/NET_ADMIN capabilities for socket binding)
- `/proc/self/status` access (Linux debugger detection)

**Production:**
- Linux amd64 (only target in `.goreleaser.yaml`)
- Root privileges (BGP TCP listen on port 179, DNS on port 5354, Unix socket in `/run/`)
- Unix socket at `/run/bgp-dnsd/bgp-dnsd.sock` (default target)
- Domain list file readable by the daemon process

## Protobuf / gRPC

**Service:** `api.BgpDnsService`
**Proto file:** `proto/api/bgp-dns.proto`
**Generated code:** `api/bgp-dns.pb.go`, `api/bgp-dns_grpc.pb.go`
**Build command:** `make pb` (runs `buf generate --path proto/api`)

**RPCs:**
| RPC | Type | Request | Response | Purpose |
|-----|------|---------|----------|---------|
| `ListCacheEntries` | Server streaming | `google.protobuf.Empty` | `ListCacheEntriesResponse` | Stream all cached DNS entries |
| `ClearCache` | Unary | `ClearCacheRequest` | `ClearCacheResponse` | Clear all cache entries, return count |
| `ReloadList` | Unary | `ReloadListRequest` | `ReloadListResponse` | Trigger domain list file reload |

---

*Stack analysis: 2026-06-13*
