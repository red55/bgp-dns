# Code Patterns and Conventions

**Analysis Date:** 2026-06-13

## Error Handling Patterns

### Startup Failures — Panic
The daemon entrypoint (`cmd/bgp-dnsd/main.go`) uses `panic(e)` for all initialization failures:

```go
// cmd/bgp-dnsd/main.go:35-88
if cfg, e = config.Init(configPath); e != nil { panic(e) }
if e = bgp.Serve(ctx); e != nil { panic(e) }
if e = cli.Serve(_app, cfg.Dns.List.File); e != nil { panic(e) }
if e = dns.Serve(ctx); e != nil { panic(e) }
if e = dns.Load(cfg.Dns.List.File); e != nil { panic(e) }
if e = fswatcher.Serve(ctx); e != nil { panic(e) }
```

### Fatal Logging via zerolog
Subsystem bind/listen failures use `log.L().Fatal()`:

```go
// internal/dns/main.go:51
log.L().Fatal().Str("m", "dns").Err(err).Msg("Failed to bind DNS resolver")

// internal/bgp/main.go:66
_bgp.L().Panic().Err(e).Msg("Failed to start BGP instance")

// cmd/bgp-dnsd/cli/cache.go:56
_app.L().Fatal().Err(e).Msgf("CLI Service: Failed to bind to %s", target)
```

### gRPC Error Responses
The gRPC handler uses `status.Error()` with appropriate gRPC status codes:

```go
// cmd/bgp-dnsd/cli/cache.go:81
return status.Error(codes.InvalidArgument, "stream cannot be nil")

// cmd/bgp-dnsd/cli/cache.go:104
return status.Errorf(codes.FailedPrecondition, "failed to clear cache: %v", err)

// cmd/bgp-dnsd/cli/cache.go:106
return status.Errorf(codes.Internal, "failed to clear cache: %v", err)
```

### Sentinel Errors
Package-level sentinel errors for specific failure modes:

```go
// internal/dns/main.go:22-23
EInvalidFQDN    = errors.New("invalid FQDN")
ENotInitialized = errors.New("cache subsystem is not initialized")

// internal/dns/resolvers.go:16-17
ErrNoResolvers = errors.New("no resolvers available")
ErrEmptyAnswer = errors.New("empty answer from resolver")
```

### Error Wrapping
Standard `fmt.Errorf` with `%w` wrapping for config errors:

```go
// internal/config/main.go:29
return nil, fmt.Errorf("unable to read application configuration: %w", err)
```

## Configuration Approach

### Viper YAML Config
Configuration is loaded via Viper from a YAML file:

```go
// internal/config/main.go:17-30
func Init(path string) (*AppCfg, error) {
    viper.SetConfigFile(path)
    viper.SetConfigType("yaml")
    viper.AddConfigPath(path)  // or "."
    viper.ReadInConfig()
    // ...
}
```

### Custom Decode Hooks
Viper's `DecodeHook` handles non-standard types:

```go
// internal/config/main.go:33-51
decodeHook := func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
    if from.Kind() == reflect.String {
        if to == reflect.TypeOf(net.IP{}) {
            s := data.(string)
            s = strings.Trim(s, "\"'")
            return net.ParseIP(s), nil
        }
        if to == reflect.TypeOf(zerolog.DebugLevel) {
            if l, e := zerolog.ParseLevel(data.(string)); e == nil {
                return l, nil
            }
        }
    }
    return data, nil
}
```

### Context-Based Config Passing
Config is injected into context with a string key:

```go
// cmd/bgp-dnsd/main.go:49
ctx = context.WithValue(ctx, "cfg", cfg)

// internal/dns/main.go:29
var cfg = ctx.Value("cfg").(*config.AppCfg)
```

### Domain List File Format
Plain text, one domain per line. Comments start with `#` or `;`:

```
# sample/my.lst
cloudflare.com
google.com
; svoboda.org
```

## Logging Patterns

### Zerolog Structured Logging
All packages use a wrapped `log.Log` struct that embeds `*zerolog.Logger` with a module name field:

```go
// internal/log/main.go:39-48
func NewLog(l *zerolog.Logger, moduleName string) (r Log) {
    mn := fmt.Sprintf("%-10s", moduleName)
    nl := l.With().Str("m", mn).Logger()
    r.setLogger(&nl)
    return
}
```

### Module Prefix Format
Log output includes a fixed-width module name:

```
2026-06-13T00:00:00Z [m: bgp-dnsd ] INFO Failed to bind DNS resolver
```

### Level Propagation
Log level is set globally and propagated to subsystems:

```go
// cmd/bgp-dnsd/main.go:41-42
_app.SetLevel(cfg.Log.Level)
log.SetLevel(cfg.Log.Level)
```

### BGP Logger Bridge
GoBGP's log interface is bridged to zerolog:

```go
// internal/bgp/zerologger.go:31-48
func (h *zeroLogger) Info(msg string, fields bgplog.Fields) {
    withFields(h.L().Info(), fields).Msg(msg)
}
```

## Concurrency Patterns

### Single-Threaded Operation Loop (`internal/loop`)
A buffered channel serializes operations on shared state:

```go
// internal/loop/main.go:12-15
type Loop struct {
    opCh chan *loopOp  // buffered channel for operations
    l    *log.Log
}

// internal/loop/main.go:28-47
func (l *Loop) Operation(f func() error, ret bool) (e error) {
    ec := make(chan error)  // if ret == true
    l.opCh <- &loopOp{f: f, errCh: ec}
    if ret { e = <-ec }
    return
}
```

Both BGP (`internal/bgp/main.go:136`) and cache notifications (`internal/dns/cache.go:242`) use this pattern to serialize mutations.

### Context Cancellation
All long-running goroutines use `context.WithCancel` for shutdown:

```go
// internal/dns/main.go:34
ctx, _cancel = context.WithCancel(ctx)

// internal/bgp/main.go:51
ctx, _bgp.cancel = context.WithCancel(ctx)
```

### WaitGroup for Goroutine Synchronization
```go
// internal/dns/main.go:17
_wg sync.WaitGroup

// internal/dns/main.go:46-47
_wg.Add(1)
defer _wg.Done()
```

### Atomic Operations
- `cache.gen atomic.Uint64` — generation counter (`internal/dns/cache.go:32`)
- `cacheEntry.gen atomic.Uint64` — per-entry generation (`internal/dns/cacheEntry.go:30`)
- `resolver.okay atomic.Bool` — health tracking (`internal/dns/resolvers.go:22`)
- `cacheEntry.failures atomic.Uint64` — failure counter (`internal/dns/cacheEntry.go:34`)
- `ipRefCounter map[string]*atomic.Uint64` — BGP reference counts (`internal/bgp/main.go:20`)
- `Log.l atomic.Pointer[zerolog.Logger]` — thread-safe logger swap (`internal/log/main.go:13`)

### Signal Handling
```go
// cmd/bgp-dnsd/main.go:53-54
c := make(chan os.Signal, 1)
signal.Notify(c, os.Interrupt)
```

### Select-Based Event Loops
All loops use `select` for goroutine cancellation:

```go
// internal/dns/loop.go:62-76
select {
case o := <-c.ChanOp():
    cancelTimeout()
    c.HandleOp(o)
    continue
case <-timeout.Done():
    cancelTimeout()
    continue
case <-ctx.Done():
    cancelTimeout()
    break L
}
```

## CLI Structure

### Cobra Command Hierarchy
**bgp-dnsd:**
```
bgp-dnsd
  └─ (root command with flags: --target, --config)
```

**bgp-dnsctl:**
```
bgp-dnsctl
  ├─ cache
  │   ├─ list      — stream all cache entries
  │   └─ clear     — clear all cache entries
  └─ list
      └─ reload    — trigger domain list file reload
```

### Flag Pattern
Persistent flags shared across subcommands:

```go
// cmd/bgp-dnsd/commands/root.go:19-24
cmd.PersistentFlags().StringVarP(&_app.Flags.Target, "target", "t",
    app.DefaultTarget(), app.TargetDescription)
cmd.PersistentFlags().StringVarP(&_app.Flags.Config, "config", "c",
    "appsettings.yml", "Path to configuration file.")
```

### gRPC Client Connection
The CLI establishes a gRPC connection in `PersistentPreRunE`:

```go
// cmd/bgp-dnsctl/commands/root.go:24-37
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    conn, e = grpc.NewClient(_app.Flags.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
    client = api.NewBgpDnsServiceClient(conn)
    // ...
}
```

### Context Timeout for CLI Commands
Commands use a 30-second timeout (or no timeout under debugger):

```go
// cmd/bgp-dnsctl/commands/cache.go:85-89
if _app.UnderDebugger {
    ctx, cancel = context.WithCancel(cmd.Context())
} else {
    ctx, cancel = context.WithTimeout(cmd.Context(), 30*time.Second)
}
```

## gRPC Service Design

### Service Definition
```protobuf
// proto/api/bgp-dns.proto
service BgpDnsService {
  rpc ListCacheEntries(google.protobuf.Empty) returns (stream ListCacheEntriesResponse);
  rpc ClearCache(ClearCacheRequest) returns (ClearCacheResponse);
  rpc ReloadList(ReloadListRequest) returns (ReloadListResponse);
}
```

### Server Implementation
```go
// cmd/bgp-dnsd/cli/cache.go:21-23
type CacheCliServiceImpl struct {
    api.BgpDnsServiceServer  // embeds UnimplementedBgpDnsServiceServer
}
```

### Streaming Response Pattern
```go
// cmd/bgp-dnsd/cli/cache.go:79-95
func (CacheCliServiceImpl) ListCacheEntries(unused *emptypb.Empty, stream grpc.ServerStreamingServer[api.ListCacheEntriesResponse]) error {
    return dns.DumpCache(func(qtype uint16, fqdn string, ...) error {
        resp := api.ListCacheEntriesResponse{...}
        return stream.Send(&resp)
    })
}
```

### Interface Embedding for Forward Compatibility
```go
// api/bgp-dns_grpc.pb.go:87-91
type BgpDnsServiceServer interface {
    ListCacheEntries(...) error
    ClearCache(...) (*ClearCacheResponse, error)
    ReloadList(...) (*ReloadCacheResponse, error)
    mustEmbedUnimplementedBgpDnsServiceServer()
}
```

## Idiomatic Go Patterns Used

### Build Tags for Platform-Specific Code
```go
//go:build linux
//go:build darwin
//go:build windows
//go:build !linux && !windows && !darwin
```

### Helper Functions with `t.Helper()`
```go
// internal/dns/cache_test.go:17
func newTestCache(t *testing.T) *cache {
    t.Helper()
    // ...
}
```

### Interface Embedding
```go
// internal/log/main.go:12
type Log struct {
    l   atomic.Pointer[zerolog.Logger]
    lvl atomic.Value
}

// internal/dns/cache.go:23
type cache struct {
    loop.Loop   // embeds the Loop struct
    log.Log     // embeds the Log struct
    // ...
}
```

### Named Return Values
```go
// internal/dns/cache.go:35
func newCache(max int, minTtl time.Duration, rs *resolvers, l *zerolog.Logger) (r *cache) {
    r = &cache{...}
    return
}
```

### Deferred Cleanup
```go
// internal/dns/cache.go:199-204
defer func(f *os.File) {
    err := f.Close()
    if err != nil { log.L().Warn().Msgf("Failed to close file: %v", err) }
}(f)
```

---

*Patterns analysis: 2026-06-13*
