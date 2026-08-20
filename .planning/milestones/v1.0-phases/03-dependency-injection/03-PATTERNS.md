# Phase 03: Dependency Injection - Pattern Map

**Mapped:** 2026-06-16
**Files analyzed:** 10
**Analogs found:** 7 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/dns/main.go` | controller | request-response | `internal/bgp/main.go` (bgpSrv struct) | exact |
| `internal/bgp/main.go` | controller | CRUD | `internal/fswatcher/main.go` (fsWatcher struct) | exact |
| `internal/fswatcher/main.go` | controller | file-I/O | `internal/bgp/main.go` (bgpSrv struct) | exact |
| `cmd/bgp-dnsd/main.go` | config | startup-sequence | `internal/bgp/main.go` (Serve function) | role-match |
| `internal/config/main.go` | config | init | `internal/config/test.go` (TestConfig) | exact |
| `internal/loop/main.go` | utility | event-driven | `internal/log/main.go` (NewLog) | role-match |
| `internal/dns/resolvers.go` | service | request-response | `internal/dns/resolvers.go` (newResolvers) | exact |
| `internal/dns/loop.go` | service | event-driven | `internal/bgp/loop.go` (bgpSrv.loop) | exact |
| `internal/fswatcher/loop.go` | service | event-driven | `internal/dns/loop.go` (cache.loop) | exact |
| `internal/bgp/main.go` (bgp.go) | model | CRUD | `internal/bgp/bgp.go` (bgpSrv.add/remove) | exact |

## Pattern Assignments

### `internal/dns/main.go` (controller, request-response)

**Analog:** `internal/bgp/main.go` — bgpSrv struct with embedded Loop, Log, and global `_bgp`

**Struct pattern with embedding** (from `internal/bgp/main.go:19-29`):
```go
type bgpSrv struct {
    loop.Loop
    log.Log
    bgp *bgpsrv.BgpServer
    ipRefCounter map[string]*atomic.Uint64
    cancel       context.CancelFunc
    wg           sync.WaitGroup
    asn          uint32
    id           net.IP
}
```
**Apply to DNS:** Replace globals `_server`, `_wg`, `_resolvers`, `_cancel`, `_cache` with a `Service` struct:
```go
type Service struct {
    loop      loop.Loop
    log.Log
    cache     *cache
    resolvers *resolvers
    cancel    context.CancelFunc
    server    *dns.Server
    wg        sync.WaitGroup
}
```

**Global variable pattern** (from `internal/bgp/main.go:31-38`):
```go
var (
    _bgp *bgpSrv
    _v4Family = &bgpapi.Family{...}
)
```
**Apply to DNS:** Replace individual globals with single `_dns *Service`:
```go
var _dns *Service
```

**Constructor pattern** (target — no existing analog, use bgpSrv struct as template):
```go
func NewDns(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*Service, error) {
    s := &Service{
        cfg:    cfg,
        loop:   l,
        logger: log.NewLog(logger, "dns"),
    }
    s.resolvers = newResolvers(cfg.Dns.Resolvers, logger)
    s.cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, s.resolvers, logger)
    mux := newRegexServeMux()
    s.cache.SetMux(mux)
    mux.SetCatchAll(s.resolvers.proxyQuery)
    if err := s.cache.serve(context.Background()); err != nil {
        return nil, fmt.Errorf("dns: cache serve failed: %w", err)
    }
    return s, nil
}
```

**Wrapper function pattern** (from `internal/bgp/main.go:40-128`, `Serve`):
```go
// Current bgp.Serve reads from ctx, creates global, returns error or panics
func Serve(ctx context.Context) (e error) {
    cfg := ctx.Value("cfg").(*config.AppCfg)
    _bgp = &bgpSrv{...}
    // ...
}
```
**Apply to DNS wrapper:**
```go
func Serve(ctx context.Context) error {
    cfg := ctx.Value(configKey{}).(*config.AppCfg)
    l := loop.NewLoop(1)
    s, err := NewDns(cfg, l, log.L())
    if err != nil {
        return err
    }
    _dns = s
    return nil
}
```

**Shutdown pattern** (from `internal/bgp/main.go:129-137`, `Shutdown`):
```go
func Shutdown(ctx context.Context) (e error) {
    if e = _bgp.bgp.StopBgp(ctx, &bgpapi.StopBgpRequest{}); e != nil {
        _bgp.L().Panic().Err(e).Msg("Failed to shutdown BGP instance")
    }
    _bgp.cancel()
    _bgp.bgp.Stop()
    _bgp.wg.Wait()
    return nil
}
```
**Apply to DNS Shutdown** (lines 61-83 in current file): The current `Shutdown` is already clean — just update it to use `_dns` global instead of individual `_cache`, `_server`, `_wg`, `_cancel` globals.

**Forward-compat wrapper pattern** (from `internal/bgp/main.go:139-162`, `Advance`):
```go
func Advance(ips []string) error {
    return _bgp.Operation(func() (e error) { ... }, true)
}
```
**Apply to DNS wrappers** (`Load`, `DumpCache`, `ClearCache`):
```go
func Load(fn string) error {
    if _dns == nil || _dns.cache == nil {
        return ENotInitialized
    }
    return _dns.cache.load(fn)
}
```

**Test helper preservation pattern** (from `internal/bgp/main.go:164-197`):
- `SetBgpForTest(s *bgpSrv)` → no DNS equivalent needed (DNS tests use `newCache`/`newResolvers` directly)
- `NewBgpSrvForTest(t)` → no DNS equivalent needed
- `GetBgpRefCounter()` → no DNS equivalent needed
- **Critical:** E2E tests call `newCache()` and `newResolvers()` directly — keep their signatures unchanged

---

### `internal/bgp/main.go` (controller, CRUD)

**Analog:** `internal/fswatcher/main.go` — fsWatcher struct with embedded Loop, Log, and global `_watcher`

**Struct pattern** (from `internal/fswatcher/main.go:14-20`):
```go
type fsWatcher struct {
    loop.Loop
    log.Log
    w *fsnotify.Watcher
    wg sync.WaitGroup
    cancel context.CancelFunc
}
```
**Apply to BGP:** The `bgpSrv` struct already exists (lines 19-29). The key change is adding a `NewBgp()` constructor that returns `(*bgpSrv, error)` instead of panicking.

**Constructor pattern** (target):
```go
func NewBgp(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*bgpSrv, error) {
    s := &bgpSrv{
        Loop:         l,
        Log:          log.NewLog(logger, "bgp"),
        bgp:          bgpsrv.NewBgpServer(bgpsrv.LoggerOption(newZeroLogger(cfg.Log.Level))),
        ipRefCounter: make(map[string]*atomic.Uint64),
        asn:          cfg.Bgp.Asn,
        id:           cfg.Bgp.Id,
    }
    go s.bgp.Serve()
    ctx, s.cancel = context.WithCancel(context.Background())
    
    if e := s.bgp.StartBgp(ctx, &bgpapi.StartBgpRequest{...}); e != nil {
        return nil, fmt.Errorf("bgp: start failed: %w", e)
    }
    // ... peer setup ...
    go s.loop(ctx)
    return s, nil
}
```

**Panic-to-error conversion** (from `internal/bgp/main.go:69-71`):
```go
// Current: panic on BGP start failure
if e = _bgp.bgp.StartBgp(ctx, ...); e != nil {
    _bgp.L().Panic().Err(e).Msg("Failed to start BGP instance")
}
```
**Target:** Return error instead:
```go
if e := s.bgp.StartBgp(ctx, ...); e != nil {
    return nil, fmt.Errorf("bgp: start failed: %w", e)
}
```

**Panic-to-error in peer setup** (from `internal/bgp/main.go:120-122`):
```go
// Current: Fatal on peer add failure
if e = _bgp.bgp.AddPeer(ctx, ...); e != nil {
    _bgp.L().Fatal().Err(e).Msgf("Failed to add peer %s", peer.Address.String())
}
```
**Target:** Return error with peer info:
```go
if e := s.bgp.AddPeer(ctx, ...); e != nil {
    return nil, fmt.Errorf("bgp: add peer %s failed: %w", peer.Address, e)
}
```

**Panic in Shutdown** (from `internal/bgp/main.go:129-137`):
```go
// Current: panic on stop failure
if e = _bgp.bgp.StopBgp(ctx, ...); e != nil {
    _bgp.L().Panic().Err(e).Msg("Failed to shutdown BGP instance")
}
```
**Target:** Return error:
```go
func Shutdown(ctx context.Context) error {
    if e := _bgp.bgp.StopBgp(ctx, &bgpapi.StopBgpRequest{}); e != nil {
        return fmt.Errorf("bgp: shutdown failed: %w", e)
    }
    _bgp.cancel()
    _bgp.bgp.Stop()
    _bgp.wg.Wait()
    return nil
}
```

**Dead code removal:**
- Line 19: `//ipRefCounter *hashmap.Map[string, *atomic.Uint64]` — remove commented line
- Line 46: `//ipRefCounter: hashmap.New[string, *atomic.Uint64](),` — remove commented line

---

### `internal/fswatcher/main.go` (controller, file-I/O)

**Analog:** `internal/bgp/main.go` — bgpSrv struct with global `_bgp` and Serve/Shutdown wrappers

**Struct pattern** (from `internal/fswatcher/main.go:14-20`):
```go
type fsWatcher struct {
    loop.Loop
    log.Log
    w *fsnotify.Watcher
    wg sync.WaitGroup
    cancel context.CancelFunc
}
```
**Target:** Add config field to struct:
```go
type fsWatcher struct {
    loop.Loop
    log.Log
    w      *fsnotify.Watcher
    wg     sync.WaitGroup
    cancel context.CancelFunc
    cfg    *config.AppCfg
}
```

**Constructor pattern** (target):
```go
func NewFsWatcher(cfg *config.AppCfg, l loop.Loop, logger *zerolog.Logger) (*fsWatcher, error) {
    w := &fsWatcher{
        Loop:   l,
        Log:    log.NewLog(logger, "fswatcher"),
        cfg:    cfg,
        wg:     sync.WaitGroup{},
        cancel: nil,
    }
    if w.w, e = fsnotify.NewWatcher(); e != nil {
        return nil, fmt.Errorf("fswatcher: create watcher failed: %w", e)
    }
    // ... file stat check ...
    // ... add watch ...
    return w, nil
}
```

**Wrapper pattern** (from `internal/bgp/main.go:40-128`):
```go
func Serve(ctx context.Context) error {
    cfg := ctx.Value(configKey{}).(*config.AppCfg)
    l := loop.NewLoop(1)
    s, err := NewFsWatcher(cfg, l, log.L())
    if err != nil {
        return err
    }
    _watcher = s
    return nil
}
```

---

### `cmd/bgp-dnsd/main.go` (config, startup-sequence)

**Analog:** `internal/bgp/main.go` — Serve function reads from context, creates global

**Current anti-pattern** (from `cmd/bgp-dnsd/main.go:35-88`):
```go
// 7 panic() calls — all must be replaced
panic(errors.New("wrong path to configuration file"))  // line 35
panic(e)  // line 39
panic(e)  // line 57
panic(e)  // line 66
panic(e)  // line 75
panic(e)  // line 84
panic(e)  // line 88
```

**Typed context key usage** (from `internal/config/main.go` — NEW):
```go
// cmd/bgp-dnsd/main.go line 49 — current:
ctx = context.WithValue(ctx, "cfg", cfg)
// target:
ctx = context.WithValue(ctx, configKey{}, cfg)
```

**Startup sequence pattern** (target — no existing analog, derive from bgp Serve pattern):
```go
func main() {
    _app = app.New(filepath.Base(os.Args[0]), zerolog.InfoLevel)
    // ... command parsing ...
    
    configPath, e := filepath.Abs(_app.Flags.Config)
    if e != nil {
        _app.StdErr(e, "cannot resolve config path")
        os.Exit(1)
    }
    
    cfg, e := config.Init(configPath)
    if e != nil {
        _app.StdErr(e, "failed to load configuration")
        os.Exit(1)
    }
    _app.SetLevel(cfg.Log.Level)
    log.SetLevel(cfg.Log.Level)
    
    ctx := context.Background()
    ctx = context.WithValue(ctx, configKey{}, cfg)
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    
    // BGP service — constructor returns error, not panic
    bgpSrv, e := bgp.NewBgp(cfg, loop.NewLoop(1), log.L())
    if e != nil {
        _app.StdErr(e, "failed to start BGP service")
        os.Exit(1)
    }
    defer func() {
        if e := bgpSrv.Shutdown(ctx); e != nil {
            _app.StdErr(e, "BGP shutdown failed")
        }
    }()
    
    // ... similar for CLI, DNS, FSWatcher ...
    
    _app.StdOut("Startup complete.")
    select {
    case <-c:
        _app.StdOut("Gracefully shutting down...")
    case <-ctx.Done():
        // ...
    }
}
```

**Shutdown order** — BGP first, then DNS, then FSWatcher (reverse of startup). Each service gets its own `defer` cleanup.

---

### `internal/config/main.go` (config, init)

**Analog:** `internal/config/test.go` — `TestConfig()` function

**Typed context key definition** (NEW — no existing analog):
```go
// internal/config/main.go — add after imports, before Init()
type configKey struct{}
```

**Usage in other files:**
```go
// cmd/bgp-dnsd/main.go:
ctx = context.WithValue(ctx, configKey{}, cfg)

// internal/dns/main.go wrapper:
cfg := ctx.Value(configKey{}).(*config.AppCfg)

// internal/bgp/main.go wrapper:
cfg := ctx.Value(configKey{}).(*config.AppCfg)

// internal/fswatcher/main.go wrapper:
cfg := ctx.Value(configKey{}).(*config.AppCfg)
```

---

### `internal/loop/main.go` (utility, event-driven)

**Analog:** `internal/log/main.go` — `NewLog(logger *zerolog.Logger, module string)` explicit logger injection

**Current anti-pattern** (from `internal/loop/main.go:17-23`):
```go
func NewLoop(bufSize int) Loop {
    l := log.NewLog(log.L(), "loop")  // ← calls global log.L() — anti-pattern
    return Loop{
        opCh: make(chan *loopOp, bufSize),
        l:    &l,
    }
}
```

**Target pattern** (explicit logger parameter):
```go
func NewLoop(bufSize int, logger *zerolog.Logger) Loop {
    l := log.NewLog(logger, "loop")
    return Loop{
        opCh: make(chan *loopOp, bufSize),
        l:    &l,
    }
}
```

**Call-site updates** (wherever `loop.NewLoop(1)` is called):
```go
// cmd/bgp-dnsd/main.go:
loop.NewLoop(1, log.L())

// internal/dns/main.go wrapper:
loop.NewLoop(1, logger)

// internal/bgp/main.go wrapper:
loop.NewLoop(1, logger)

// internal/fswatcher/main.go wrapper:
loop.NewLoop(1, logger)
```

---

### `internal/dns/resolvers.go` (service, request-response)

**Analog:** `internal/dns/resolvers.go:54-61` — current `newResolvers()` with internal `log.L()` call

**Current pattern** (from `internal/dns/resolvers.go:54-61`):
```go
func newResolvers(c []*net.UDPAddr) *resolvers {
    r := &resolvers{
        Log: log.NewLog(log.L(), "resolvers"),  // ← calls global
    }
    r.setResolvers(c)
    return r
}
```

**Target pattern** (explicit logger parameter):
```go
func newResolvers(c []*net.UDPAddr, logger *zerolog.Logger) *resolvers {
    r := &resolvers{
        Log: log.NewLog(logger, "resolvers"),
    }
    r.setResolvers(c)
    return r
}
```

**Call-site update** (from `internal/dns/main.go:36-37`):
```go
// Current:
_resolvers = newResolvers(cfg.Dns.Resolvers)
_cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, newResolvers(cfg.Dns.List.Resolvers), log.L())

// Target (in NewDns constructor):
s.resolvers = newResolvers(cfg.Dns.Resolvers, logger)
s.cache = newCache(cfg.Dns.Cache.MaxEntries, cfg.Dns.Cache.MinTtl, s.resolvers, logger)
```

**Dead code removal** (from `internal/dns/resolvers.go:127-148`):
Remove the entire commented block:
```go
/*
    cause := e
    if unwrap, ok := cause.(interface{ Unwrap() error }); ok {
        cause = unwrap.Unwrap()
    }
    // ... 22 lines of commented code ...
*/
```

**⚠️ Critical: Internal constructor signature change** — The `newResolvers()` signature changes from `newResolvers(c []*net.UDPAddr)` to `newResolvers(c []*net.UDPAddr, logger *zerolog.Logger)`. This will break E2E tests that call `newResolvers()` directly (see `dns_e2e_test.go:87`). The tests must be updated to pass a logger.

---

### `internal/dns/loop.go` (service, event-driven)

**Analog:** `internal/bgp/loop.go:6-19` — bgpSrv.loop() reads ctx.Done(), no config read

**Current anti-pattern** (from `internal/dns/loop.go:17`):
```go
func (c *cache) loop(ctx context.Context) {
    cfg := ctx.Value("cfg").(*config.AppCfg)  // ← reads from context
    // ...
    if sleepUntil.After(ceExp) {
        // line 58: uses cfg.Dns.Cache.MinTtl
    }
    // line 77: log.L().Error().Err(ctx.Err())
}
```

**Target pattern** (config from struct field, not context):
```go
func (c *cache) loop(ctx context.Context) {
    // cfg is now c.cfg — set in newCache() constructor
    // ...
    if sleepUntil.After(ceExp) {
        sleepUntil = now.Add(c.cfg.Dns.Cache.MinTtl * time.Second)
    }
    // line 77: use c.L() instead of log.L()
}
```

**Cache struct update** (from `internal/dns/cache.go:23-34`):
```go
type cache struct {
    loop.Loop
    log.Log
    m       sync.RWMutex
    wg      sync.WaitGroup
    entries gcache.Cache
    cancel  context.CancelFunc
    rs      *resolvers
    minTtl  time.Duration
    gen     atomic.Uint64
    mux     *regexServeMux
    cfg     *config.AppCfg  // ← ADD this field
}
```

**Cache constructor update** (from `internal/dns/cache.go:36-49`):
```go
func newCache(max int, minTtl time.Duration, rs *resolvers, l *zerolog.Logger, cfg *config.AppCfg) (r *cache) {
    r = &cache{
        Loop:   loop.NewLoop(1),
        Log:    log.NewLog(l, "dns"),
        cancel: nil,
        rs:     rs,
        minTtl: minTtl,
        gen:    atomic.Uint64{},
        cfg:    cfg,  // ← set from constructor param
    }
    // ...
}
```

---

### `internal/fswatcher/loop.go` (service, event-driven)

**Analog:** `internal/dns/loop.go:13-81` — same pattern of reading config from context

**Current anti-pattern** (from `internal/fswatcher/loop.go:11`):
```go
func (w *fsWatcher) loop(ctx context.Context) {
    var cfg = ctx.Value("cfg").(*config.AppCfg)  // ← reads from context
    // ...
    if e := dns.Load(cfg.Dns.List.File); e != nil {  // line 27
        // ...
    }
}
```

**Target pattern** (config from struct field):
```go
func (w *fsWatcher) loop(ctx context.Context) {
    // cfg is now w.cfg — set in NewFsWatcher() constructor
    if e := dns.Load(w.cfg.Dns.List.File); e != nil {
        w.L().Error().Err(e).Msg("Failed to load domainlist")
    }
}
```

**fsWatcher struct update** (from `internal/fswatcher/main.go:14-20`):
```go
type fsWatcher struct {
    loop.Loop
    log.Log
    w      *fsnotify.Watcher
    wg     sync.WaitGroup
    cancel context.CancelFunc
    cfg    *config.AppCfg  // ← ADD this field
}
```

---

### `internal/bgp/main.go` — bgp.go dead code removal

**Analog:** N/A — this is a pure removal, not a pattern.

**Remove from `internal/bgp/main.go:19`:**
```go
//ipRefCounter *hashmap.Map[string, *atomic.Uint64]
```

---

## Shared Patterns

### Typed Context Key
**Source:** `internal/config/main.go` (NEW addition)
**Apply to:** `cmd/bgp-dnsd/main.go`, `internal/dns/main.go`, `internal/bgp/main.go`, `internal/fswatcher/main.go`
```go
// Definition in internal/config/main.go
type configKey struct{}

// Usage in cmd/bgp-dnsd/main.go
ctx = context.WithValue(ctx, configKey{}, cfg)

// Usage in internal/dns/main.go wrapper
cfg := ctx.Value(configKey{}).(*config.AppCfg)
```

### Explicit Logger Injection
**Source:** `internal/log/main.go:39-49` (NewLog already exists)
**Apply to:** `internal/loop/main.go`, `internal/dns/resolvers.go`, all constructor calls
```go
// Pattern: caller passes *zerolog.Logger, constructor wraps it
log.NewLog(logger, "module-name")
```

### Service Struct with Embedded Dependencies
**Source:** `internal/bgp/main.go:19-29` (bgpSrv), `internal/fswatcher/main.go:14-20` (fsWatcher)
**Apply to:** `internal/dns/main.go` (Service), `internal/bgp/main.go` (NewBgp), `internal/fswatcher/main.go` (NewFsWatcher)
```go
type Service struct {
    loop.Loop      // embedded
    log.Log        // embedded
    // ... other fields ...
}
```

### Wrapper Function for Backward Compatibility
**Source:** `internal/bgp/main.go:40-128` (Serve), `internal/bgp/main.go:139-162` (Advance)
**Apply to:** All three services (dns, bgp, fswatcher)
```go
func Serve(ctx context.Context) error {
    cfg := ctx.Value(configKey{}).(*config.AppCfg)
    l := loop.NewLoop(1, logger)
    s, err := NewXxx(cfg, l, logger)
    if err != nil {
        return err
    }
    _global = s
    return nil
}
```

### Error Returns Instead of Panics
**Source:** `internal/log/main.go:17-49` (NewLog doesn't panic)
**Apply to:** All constructors, main.go startup sequence
```go
// Before: log.L().Fatal().Err(e).Msg("...")
// After:  return nil, fmt.Errorf("pkg: operation failed: %w", e)
```

### Test Helper Preservation
**Source:** `internal/bgp/main.go:164-197` (NewBgpSrvForTest, SetBgpForTest, GetBgpRefCounter)
**Apply to:** No changes needed — these helpers operate on the existing `_bgp` global which continues to exist. The `bgpSrv` struct itself doesn't change, only a `NewBgp()` constructor is added.

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `cmd/bgp-dnsd/main.go` (startup sequence with error handling) | config | startup-sequence | No existing error-based startup pattern; current code uses 7 panic() calls |
| `internal/config/main.go` (configKey struct) | config | init | Typed context key is a new addition; no existing analog in config package |

---

## Metadata

**Analog search scope:** `internal/dns/`, `internal/bgp/`, `internal/fswatcher/`, `internal/config/`, `internal/loop/`, `internal/log/`, `cmd/bgp-dnsd/`
**Files scanned:** 10 source files + 7 test files
**Pattern extraction date:** 2026-06-16
