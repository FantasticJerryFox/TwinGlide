# Security

TwinGlide shares control of a logged-in desktop. Pair only computers you own or
are authorized to control. Compare the entire code on both screens; reject a
mismatch. Nearby announcements are untrusted discovery hints.

- TLS 1.3 control connections require a certificate on both endpoints.
- The first handshake displays 48 bits derived from the TLS exporter; both
  local confirmations are required before either side enables input sharing.
- Known certificates are pinned by SHA-256. A matching name or IP is insufficient.
- Pairing is time-limited and rate-limited. Nothing is trusted solely by discovery.
- Private keys remain local. Profiles are restricted to their user (and SYSTEM
  on Windows). The local settings API requires a per-run random capability,
  checks Host/Origin and does not enable CORS.
- Deskflow runs with TLS and reciprocal certificate fingerprints on a separate
  port. Only tracked child process handles are stopped.
- No telemetry, cloud registration or general remote-command endpoint.
- Audio is off by default. Explicit forwarding requires an active paired session
  and matching certificate pins on a separate ALPN connection. Losing pairing
  or changing the route stops only TwinGlide-owned audio helpers.
- Native audio helpers hold no credentials and access no microphone. Their
  framed stdio is bounded and validated before it reaches an output device.
- No-input testing disables physical capture/playback and startup registration;
  synthetic audio-test mode never opens an audio device.

This is an alpha implementation, not an independently audited security product.
Do not publish identity.pem, state.json, endpoint.json, access tokens or raw logs.
Use GitHub private vulnerability reporting once the repository is published.
Until that channel exists, contact the repository maintainer privately; do not
place exploitable details or credentials in a public issue.

See docs/PROTOCOL.md for the trust model and docs/RELEASING.md for release checks.
