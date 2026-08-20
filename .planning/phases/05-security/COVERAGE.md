# API Coverage — Phase 05: Security

No external API integration: first-party security hardening only — gRPC admin socket, DNS qtype policy, BGP TCP-MD5 via GoBGP. No third-party API/SDK/service is integrated.

## Why this is declared rather than enumerated

The verify:pre detector fired on the clause "(surface) → api", which is first-party
surface naming in this phase's docs, not an external/third-party integration. Phase 5
hardens the daemon's own protocol surfaces only:

- **SEC-01 / SEC-02** — gRPC-over-Unix-socket admin channel: socket forced owner-only
  0600 before serve; unified server-side interceptor audit gate; per-handler nil rejection.
- **SEC-03** — DNS served-qtype policy for listed domains: A/AAAA/HTTPS answered,
  everything else REFUSED before upstream.
- **SEC-04** — optional BGP peer `AuthPassword` mapped into the GoBGP spec for opt-in
  TCP-MD5 session keys; no credential ever logged.

All three operate on first-party protocol endpoints and the local config file. There is
no external host, vendor SDK, or remote service whose capability surface needs deciding,
so a capability matrix would fabricate rows. This reasoned declaration overrules the
detector's false-positive signal.
