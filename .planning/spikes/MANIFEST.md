# Spike Manifest

## Idea
Add regex support to domainlists in bgp-dns, allowing domain list entries to be regular expressions that intercept DNS queries matching the pattern — not just exact FQDN matches.

## Requirements
- [ ] Must support mixing exact domain entries and regex patterns in the same list file
- [ ] Regex patterns must be pre-compiled and cached for performance (no per-query compilation)
- [ ] Must not degrade performance for existing exact-match entries

## Spikes

| # | Name | Type | Validates | Verdict | Tags |
|---|------|------|-----------|---------|------|
| 001 | regex-dns-intercept | standard | Given a DNS query for any subdomain matching a regex pattern in domainlist, when the query arrives, then the DNS server returns a cached BGP-learned IP instead of proxying to upstream | VALIDATED ✓ | [regex, dns, domainlist] |
