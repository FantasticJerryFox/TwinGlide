# Third-party components

TwinGlide application code is GPL-2.0-only. Component licenses remain separate.

| Component | Use | License / source |
| --- | --- | --- |
| Deskflow 1.27.0 (macOS / Windows) | Separate, unmodified input-engine process | GPL-2.0-only WITH LicenseRef-OpenSSL-Exception; https://github.com/deskflow/deskflow |
| Go runtime / standard library | Compiled coordinator | BSD-3-Clause; https://go.dev |
| Qt and OpenSSL in official Deskflow binaries | Input engine runtime dependencies | Original upstream terms and notices; https://www.qt.io/licensing/ and https://www.openssl.org/source/ |
| Apple Cocoa, Carbon, ScreenCaptureKit, AVFoundation / Microsoft .NET Framework, WASAPI | Operating-system desktop APIs | System components, not redistributed by TwinGlide |

Official binary archives are pinned by SHA-256 in `engine/pins.json`.
Their license files are preserved in the packaged engine. Supplemental license
texts are in `licenses/`. No third-party binary is committed to the source repository.
The input engine is never presented as an original TwinGlide implementation.

Corresponding source tags:

- https://github.com/deskflow/deskflow/tree/v1.27.0

Before distributing a bundled public binary release, provide the corresponding
Deskflow/dependency source and build material with that release under the applicable
licenses. A project link alone is not the release source-compliance procedure.
See `docs/RELEASING.md`.

The original private prototype supplied the input-source restoration helper,
Windows WASAPI capture interop and macOS audio-output scheduling. They have been
adapted to explicit TwinGlide-owned processes and separate settings. Audio
transport, trust, reverse-direction capture/playback and synthetic tests are
implemented in this project; no former service credentials are reused.
AudioRelay, private pairing credentials and remote-administration scripts are not included.
