# TwinGlide protocol v2

## Discovery

UDP 59120 broadcasts an advertisement every three seconds on each active IPv4
interface: protocol=twinglide/2, device certificate SHA-256, display name, OS and
TCP control port. Advertisements expire after 15 seconds; at most 64 are retained.
Packets are limited to 1,400 bytes. The observed packet source supplies the address.
No keys, tokens or pairing codes are advertised. An advertisement can be spoofed;
it never grants trust. IPv6/mDNS discovery is not implemented in this alpha.

## Identity and initial pairing

Each private profile owns a randomly generated RSA-3072 self-signed certificate.
The SHA-256 of the DER certificate is the device identity. The certificate has a
ten-year lifetime. A missing identity is generated once; a corrupt identity fails
closed instead of silently replacing a trusted device.

TCP 59121 uses TLS 1.3 and requests a certificate from both endpoints.
ALPN twinglide-control/2 selects JSON control; hello version is 2. Both sides
must run alpha.2 or newer compatible software. Stored v1 identities remain valid.
Self-signatures and validity dates are checked. Expected discovered/saved IDs
must equal the actual certificate ID.

The initial pairing window lasts five minutes; each pending session lasts two
minutes. An incoming unpaired handshake is rate-limited, only one session may
be pending, and the TLS handshake has a six-second deadline.

Both sides display 6 bytes (48 bits, 12 hexadecimal digits) from TLS
ExportKeyingMaterial with label EXPORTER-TwinGlide-Pair-v1 and empty context.
The code binds the user decision to this specific TLS connection, not merely
names, IPs or a public certificate. Both users must approve their current session.
No trust is saved and no input process runs after a single approval.

After both approvals, each endpoint atomically saves the peer identity and
sends ready. An interrupted commit may leave one side trusted; the next
connection then requires both approvals again rather than silently escalating
trust. Reopen pairing on the endpoint that already saved the identity.

## Known peers, settings and recovery

When both hello messages report the actual TLS identity as trusted, confirmation
is skipped. The lexicographically smaller device ID initiates periodic reconnects,
avoiding duplicate sessions. Discovery updates the peer's address; a last known
address is a fallback, and its TLS pin is still checked.

Newline-delimited JSON frames are capped at 16 KiB. A ping is sent every three
seconds; missing reads time out after twelve seconds. Settings carry a monotonic
revision and actor ID, with deterministic conflict resolution. Source IDs must
belong to this pair and positions are enumerated. Shared settings include source,
side, keyboard enable, edge delay (0–1000 ms), return key (F10/F11/F12), audio
target (off or one of the two identities), volume (0–100) and buffer (40/80/160 ms).
Device names update through authenticated ping frames. Theme and login startup
are local preferences and are never copied to the peer.

An active authenticated session gates input-engine lifetime. Disconnect, pause
or removal stops only children owned by this TwinGlide instance.

## Input transport

Deskflow is a separate process on TCP 59122, using the profile's certificate and
the verified peer fingerprint. Windows uses an isolated private portable engine
directory so its TLS database cannot fall back to a global Deskflow directory.
The launch environment also isolates initial Qt settings/state paths.
Clipboard is disabled. Display names never enter the input configuration:
computer identifiers are generated from certificate hashes.

## Audio transport

ALPN twinglide-audio/2 uses a separate TLS connection on TCP 59121 and the same
mutually checked identities. Its dialer is the current control-session dialer,
independent of audio direction. An incoming audio connection is accepted only
with an active, confirmed control session, explicit nonempty audio target,
matching peer pin, and no other audio connection. A one-byte 0xA2 acknowledgement
precedes PCM data; the configured source writes while the selected target reads.

Native helpers receive no keys, sockets or network addresses. They monitor their
agent parent process and exit if it disappears, including forced termination.
They use stdio
frames with a 4-byte little-endian length, capped at 65537 bytes. Type 1 is a
7-byte format record: type, uint32 sample rate (8000–192000), stereo channels (2),
reserved zero. Type 2 is interleaved little-endian float32 stereo (8 bytes/frame).
Type 3 is a one-byte heartbeat. Repeated formats, malformed lengths and unknown
types close the connection. PCM is clamped and nonfinite samples sanitized by
native helpers. This alpha uses uncompressed audio on the local network.

The receiver allows 90 seconds for initial format/OS capture authorization,
then a four-second frame timeout. Writes time out after three seconds. The
source emits heartbeats even when the audio device is silent. A 150 ms watcher
cancels only owned helpers when the control session ends or the route changes.
Native audio never runs merely because a peer is discoverable or paired.
No-input mode blocks all physical audio; audio-test mode selects only synthetic
fixture/verification helpers and also forces no-input.

The native desktop window calls an API bound only to 127.0.0.1 on a random port.
A capability token is read from the private endpoint file after matching the
owned agent PID, and sent in an HTTP header. No webpage or web assets are served;
the root route returns 404. Cross-origin requests and unexpected Host headers are
denied. The capability is regenerated on every start.

Friendly nicknames are generated with OS cryptographic randomness on first
launch and persisted before discovery begins. They are labels, not identities:
trust is always tied to certificate fingerprints. Nickname suggestions do not
rename a device until the user saves. Same-platform peers, including two Windows
machines, use the same protocol; the first pairing initiator is their initial
input source. Saved role choices survive reconnection.
