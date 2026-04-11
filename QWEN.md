# bgp-dns

## Project Overview

**bgp-dns** is a Go daemon that acts as a caching DNS resolver and announces resolved A records to BGP peers. It consists of two main binaries:

- **`bgp-dnsd`** — The main daemon that:
  - Listens for DNS queries on a configurable port
  - Resolves queries using upstream resolvers
  - Caches DNS responses with configurable TTL and entry limits
  - Announces resolved A records (IPv4) to BGP peers using GoBGP
  - Watches a domain list file for changes and reloads it automatically
  - Exposes a gRPC/Unix socket interface for CLI control

- **`bgp-dnsctl`** — A CLI tool to interact with and control the `bgp-dnsd` daemon

### Architecture

The project is organized into the following packages:

| Package | Purpose |
|---------|---------|
| `cmd/bgp-dnsd` | Daemon entrypoint — wires together BGP, DNS, file watcher, and CLI subsystems |
| `cmd/bgp-dnsctl` | CLI entrypoint — provides commands to interact with the daemon |
| `internal/app` | Application bootstrap, global flags, logging helpers |
| `internal/bgp` | BGP server wrapper using GoBGP — manages peer connections and route announcements |
| `internal/dns` | DNS server using `miekg/dns` — handles query resolution, caching, and upstream forwarding |
| `internal/config` | Configuration loading and parsing (YAML via Viper) |
| `internal/fswatcher` | File system watcher that monitors the domain list file for changes |
| `internal/loop` | Concurrency primitive for running operations in a controlled single-threaded loop |
| `internal/log` | Zerolog-based logging infrastructure |
| `internal/version` | Build-time version/commit metadata injection |
| `api/` | Generated gRPC/protobuf Go code |
| `proto/` | Protobuf definitions with Buf build configuration |

### Key Dependencies

- **GoBGP** (`github.com/osrg/gobgp/v3`) — BGP protocol implementation
- **miekg/dns** (`github.com/miekg/dns`) — DNS protocol library
- **spf13/cobra + viper** — CLI framework and configuration management
- **gRPC + protobuf** — Daemon-to-CLI communication
- **fsnotify** — File system event watching
- **gcache** — DNS response caching
- **zerolog** — Structured logging

## Building and Running

### Prerequisites

- Go 1.24+
- `buf` CLI tool (for protobuf code generation)

### Build Commands

```bash
# Build both binaries
make all

# Build only the daemon
make bgp-dnsd

# Build only the CLI
make bgp-dnsctl

# Generate protobuf code
make pb

# Clean build artifacts
make clean
```

### Running the Daemon

```bash
./bgp-dnsd --config appsettings.yml
```

### Configuration

The daemon is configured via a YAML file (`appsettings.yml` by default):

```yaml
Log:
  Level: Trace        # Log level: Trace, Debug, Info, Warn, Error, Fatal

Bgp:
  Asn: 65530          # Local BGP AS number
  Id: 127.0.0.1       # BGP router ID
  Listen:
    Ip: 0.0.0.0       # BGP listen address
    Port: 8179        # BGP listen port
  Peers:
    - Asn: 65530
      Address:
        Ip: 192.168.151.44
        Port: 179

Dns:
  Listen:
    Ip: 0.0.0.0
    Port: 5354        # DNS listen port
  Resolvers:
    - Ip: 77.88.8.8   # Upstream DNS resolvers
      Port: 53
    - Ip: 77.88.8.1
      Port: 53
  Cache:
    MinTtl: 10        # Minimum TTL in seconds
    MaxEntries: 5000  # Maximum cache entries
  List:
    File: sample/my.lst   # Path to domain list file
    Resolvers:            # Override resolvers for listed domains
      - Ip: 1.1.1.1
        Port: 53
      - Ip: 8.8.8.8
        Port: 53

Timeouts:
  DefaultTTL: 60
  TtlForZero: 30
  Ttl4ZeroJitter: 10     # Must be less than TtlForZero
```

### Domain List File

The domain list file (e.g., `sample/my.lst`) contains one domain per line. Domains in this list are resolved using the dedicated resolvers specified under `Dns.List.Resolvers`.

## Release

The project uses **GoReleaser** for automated releases. Configuration is in `.goreleaser.yaml`. Both binaries are built for `linux/amd64` with version metadata injected via ldflags.

## Project Conventions

- **Logging**: Uses `zerolog` with structured logging. Log levels are configurable.
- **Error handling**: Fatal errors panic; recoverable errors are logged and handled gracefully.
- **Concurrency**: Uses `sync.WaitGroup`, contexts for cancellation, and a controlled single-threaded loop pattern (`internal/loop`) for BGP/DNS operations that must be serialized.
- **gRPC**: The daemon exposes a gRPC service for CLI communication; protobuf definitions are managed with Buf.
- **Code style**: Standard Go formatting. Run `go mod tidy` and `go generate ./...` before committing.
