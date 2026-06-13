---
name: spike-findings-bgp-dns
description: Implementation blueprint from spike experiments. Requirements, proven patterns, and verified knowledge for building bgp-dns. Auto-loaded during implementation work.
---

<context>
## Project: bgp-dns

bgp-dns is a DNS resolver with BGP route advertisement. It maintains a domainlist file of FQDNs, resolves them via upstream DNS, caches the results, and advertises the resolved IPs via BGP. Spiked adding regex support to domainlist entries so that pattern-matching domains (not just exact FQDNs) can be intercepted and resolved by the DNS server.

Spike sessions wrapped: 2026-06-13
</context>

<requirements>
## Requirements

- Must support mixing exact domain entries and regex patterns in the same list file
- Regex patterns must be pre-compiled and cached for performance (no per-query compilation)
- Must not degrade performance for existing exact-match entries
</requirements>

<findings_index>
## Feature Areas

| Area | Reference | Key Finding |
|------|-----------|-------------|
| Regex Domainlist | references/regex-domainlist.md | Custom regexServeMux wraps miekg/dns — exact > wildcard > regex > catch-all priority |

## Source Files

Original spike source files are preserved in `sources/` for complete reference.
</findings_index>

<metadata>
## Processed Spikes

- 001-regex-dns-intercept
</metadata>
