# bpg-dns-peer
The daemon that acts as caching DNS resolver and announce resolved A records to BGP peers. 
## Configuration

```YAML
Timeouts:
  DefaultTTL: 60
  TtlForZero: 30
  Ttl4ZeroJitter: 10 # Must be less than TtlForZero
```