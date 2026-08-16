# Requirements: bgp-dns

**Defined:** 2026-06-13
**Core Value:** Resolve domains from a configurable list and advertise their IPs via BGP — fast, correct route propagation with zero manual intervention.

## v1 Requirements

### Regex Domainlist Support

- [x] **REGEX-01**: Daemon accepts regex patterns in domainlist file using `regex:` prefix syntax
- [x] **REGEX-02**: DNS query matching priority: exact FQDN > wildcard > regex > catch-all proxy
- [x] **REGEX-03**: Regex patterns are compiled at domainlist load time, not per-query
- [x] **REGEX-04**: Invalid regex patterns are rejected immediately with clear error message
- [x] **REGEX-05**: Existing exact-match and wildcard entries continue to work without performance regression

### Test Coverage

- [x] **TEST-01**: BGP reference counting logic has unit tests (advance/withdraw transitions)
- [x] **TEST-02**: DNS resolver ring failover has unit tests (round-robin, health tracking)
- [ ] **TEST-03**: Cache eviction on generation change has unit tests
- [ ] **TEST-04**: gRPC server lifecycle (Serve/Shutdown) has integration tests
- [x] **TEST-05**: End-to-end DNS query → cache → BGP announcement flow has integration test

### Code Quality Refactor

- [ ] **REFACTOR-01**: Package-level singleton variables replaced with explicit struct parameters
- [ ] **REFACTOR-02**: Context-based config passing uses typed context key instead of string key
- [ ] **REFACTOR-03**: Panic-on-init replaced with returned errors for graceful shutdown
- [ ] **REFACTOR-04**: Commented-out dead code removed from resolvers.go and bgp/main.go

### Reliability

- [ ] **RELIAB-01**: DNS queries have configurable timeout (default 5s)
- [ ] **RELIAB-02**: Set difference optimized from O(n²) to O(n) using map-based sets
- [ ] **RELIAB-03**: BGP operations use cancellable context instead of context.Background()
- [ ] **RELIAB-04**: BGP Advance/Withdraw errors logged at Warn level instead of silently ignored

### Security

- [ ] **SEC-01**: gRPC Unix socket file permissions set to 0600 (owner-only)
- [ ] **SEC-02**: gRPC server validates incoming requests (nil checks preserved/enhanced)
- [ ] **SEC-03**: DNS query type filtering (only A, AAAA, HTTPS, NXDOMAIN allowed for listed domains)
- [ ] **SEC-04**: BGP peer configuration supports optional AuthPassword field

## v2 Requirements

### Monitoring

- **MON-01**: Prometheus metrics endpoint for DNS query count, cache hit rate, BGP peer state
- **MON-02**: Structured JSON log output option for log aggregation
- **MON-03**: Health check endpoint (gRPC or HTTP) returning daemon status

### Advanced Features

- **ADV-01**: Wildcard domain entries (e.g., `*.example.com`) in domainlist
- **ADV-02**: Multiple domainlist files with merge support
- **ADV-03**: DNSSEC validation for upstream responses
- **ADV-04**: Rate limiting per-source IP to prevent amplification

### Operational

- **OPS-01**: systemd service file with watchdog and restart policy
- **OPS-02**: Docker container image with non-root user
- **OPS-03**: Configuration hot-reload without domainlist restart

## Out of Scope

| Feature | Reason |
|---------|--------|
| IPv6-only deployments | Project targets IPv4 BGP advertisement; IPv6 can be added later |
| Multi-instance HA | Global state architecture makes this non-trivial; not a current requirement |
| DNSSEC validation | Complex crypto dependencies; not core to bgp-dns's purpose |
| Web dashboard | CLI + gRPC interface sufficient for operational needs |
| OAuth/API auth for gRPC | Unix socket with file permissions is adequate for local management |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| REGEX-01 | Phase 1 | Complete |
| REGEX-02 | Phase 1 | Complete |
| REGEX-03 | Phase 1 | Complete |
| REGEX-04 | Phase 1 | Complete |
| REGEX-05 | Phase 1 | Complete |
| TEST-01 | Phase 2 | Complete |
| TEST-02 | Phase 2 | Complete |
| TEST-03 | Phase 2 | Pending |
| TEST-04 | Phase 2 | Pending |
| TEST-05 | Phase 2 | Complete |
| REFACTOR-01 | Phase 3 | Pending |
| REFACTOR-02 | Phase 3 | Pending |
| REFACTOR-03 | Phase 3 | Pending |
| REFACTOR-04 | Phase 3 | Pending |
| RELIAB-01 | Phase 4 | Pending |
| RELIAB-02 | Phase 4 | Pending |
| RELIAB-03 | Phase 4 | Pending |
| RELIAB-04 | Phase 4 | Pending |
| SEC-01 | Phase 5 | Pending |
| SEC-02 | Phase 5 | Pending |
| SEC-03 | Phase 5 | Pending |
| SEC-04 | Phase 5 | Pending |

**Coverage:**

- v1 requirements: 22 total
- Mapped to phases: 22
- Unmapped: 0 ✓

---
*Requirements defined: 2026-06-13*
*Last updated: 2026-06-13 after initial definition*
