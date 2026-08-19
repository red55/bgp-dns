# bpg-dns-peer
The daemon that acts as caching DNS resolver and announce resolved A records to BGP peers.
## Configuration

```YAML
Timeouts:
  DefaultTTL: 60
  TtlForZero: 30
  Ttl4ZeroJitter: 10 # Must be less than TtlForZero
```

### BGP peer authentication

Each entry under `Bgp.Peers` accepts an optional `AuthPassword` key. When set,
the session is authenticated with TCP-MD5 (RFC 2385/5925) using the value as
the session key; GoBGP enforces it on the peering connection. The key must
match the one configured on the peer router, otherwise the session will not
establish.

When omitted (or empty), peering behaves exactly as before — no session
authentication. This makes the feature opt-in per peer: existing deployments
need no changes.

Security note: the password lives in your `appsettings.yml`. If you enable it,
restrict the file's permissions (e.g. `chmod 600`) so only the daemon user can
read it. The daemon never logs credential values.