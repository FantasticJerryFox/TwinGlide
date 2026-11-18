# TwinGlide 双游

[简体中文](README.md) · **English**

**One keyboard and mouse, across two computers.** TwinGlide is a native desktop
application for sharing input and forwarding system audio over a local network.
Install it on a Windows PC and a Mac, or on two Windows PCs. Discover the other
computer by its nickname, confirm the pairing code on both screens, and reconnect
automatically on subsequent launches. No account or cloud relay is required.

[Deskflow](https://github.com/deskflow/deskflow) supplies the input engine.
TwinGlide adds device discovery, mutual pairing approval, saved identities,
reconnection, synchronized settings, audio transport and native desktop controls.

> **Current stage: 0.1.0-alpha.5, source preview.** This repository publishes source
> and build scripts; public prebuilt installers are not available yet. Windows
> alpha.5 and the previously validated local Mac alpha.4 package use the same
> protocol. Read the [support matrix](#support-and-validation) and
> [validation record](docs/VALIDATION.md) before deployment.

## Features

Designed for a desk with two computers, one set of input devices, and an optional
shared destination for system sound.

| Capability | Behavior |
| --- | --- |
| Local discovery | Displays nearby computers running TwinGlide by friendly nickname; the initial random nickname persists and can be edited or regenerated |
| Mutual pairing | Requires both computers to approve the same complete 12-hex-digit code before saving trust |
| Automatic reconnection | Remembers certificate identities and reconnects when the paired computer becomes available |
| Cross-screen input | Uses the keyboard and mouse attached to the chosen source; crossing the configured edge transfers control, and the reciprocal edge returns it |
| Screen arrangement | Places the other computer left, right, above or below the input source, with a visual preview |
| Switching preferences | Configurable 0–1000 ms edge delay and an emergency-return shortcut using F10, F11 or F12 |
| System audio | Forwards the other computer's system output to the selected destination through an encrypted connection; either direction is selectable, one at a time |
| Audio controls | Independent receive gain from 0–100% and a 40, 80 or 160 ms target buffer; forwarding is off by default |
| Personalization | Device names, light/dark/system appearance and current-user login startup; tray or menu-bar operation after closing the window |

Windows uses WinForms and macOS uses AppKit. There is no webpage, embedded web
view or browser dependency. The application UI is currently in Simplified Chinese;
these English instructions do not imply an English UI language option.

## Support and validation

| Environment | Current evidence |
| --- | --- |
| Windows 10/11 x64 ↔ Intel Mac, macOS 13+ | Physical mouse handoff, movement, clicking and return accepted with either computer as source; actual system-audio capture, encrypted transport and receiver output verified in both directions |
| Windows ↔ Windows | Pairing, source selection and settings synchronization implemented and covered by coordinator tests; acceptance on two physical Windows computers remains outstanding |
| Apple Silicon Mac | Native build path provided; physical runtime acceptance remains outstanding |
| Linux | Coordinator test environment only; no Linux desktop client or installer |

Windows requires .NET Framework 4.7.2 or later. Mac requires Local Network and
Accessibility permission, plus system-audio/screen-recording permission when
sending audio.

Only **one pair of computers** is supported. Layout connects each computer's
complete desktop, rather than individual monitors in an arbitrary multi-monitor
map. Clipboard sharing, file transfer and monitor rotation are not implemented.
Input sharing operates in logged-in desktop sessions, not Windows UAC, lock
screens or pre-login secure desktops.

Clean install/uninstall, full reboot/sleep/wake, address changes, multiple monitors
and mixed display scaling still need broader hardware acceptance. This alpha is
not an independently audited, commercially signed or notarized stable release.

## Installation and first pairing

First produce a package using [Build from source](#build-from-source). These
instructions describe installing a locally built package, not a currently
available public installer.

1. On Windows, extract the package and run `Install.cmd`. On Mac, extract it and
   open `Install.command`.
2. Connect both computers to a mutually reachable local network. Allow the
   application's network access when prompted by the operating system.
3. Open TwinGlide on both computers. Each receives a persistent random nickname
   on first launch. In **附近的设备 / Nearby devices**, select the other computer's
   name; no IP entry is needed.
4. Compare the **entire 12-hex-digit pairing code** on both screens. Confirm on
   each computer only if the codes match.
5. In **屏幕与键鼠 / Screen & keyboard**, select the computer physically connected
   to the keyboard and mouse, then the other computer's relative position. Save.
6. On Mac, use the menu-bar shortcuts for Local Network and Accessibility
   settings. Allow TwinGlide and, if listed, TwinGlide Input. After changing Local
   Network access, quit and reopen the app. See the
   [Mac permission guide](docs/MAC_PERMISSIONS.md).

The pairing window initially stays open for five minutes. Reopen it in Advanced
connection options if necessary. Replacing a device identity or clearing one
side's data requires pairing again. Already paired computers reconnect without
asking for the code again.

Closing settings leaves TwinGlide in the tray/menu bar. Choose **退出双游 / Exit
TwinGlide** to stop it. Disabling input sharing and disabling audio forwarding
are separate operations.

## Screen and audio settings

Position always describes the other computer **relative to the input source**,
regardless of which computer's settings window you are using. For example, with
the keyboard attached to Windows and the Mac on its right, select Windows as
source and Right as position. Mac is the initial source for a Windows–Mac pair;
the pairing initiator is the initial source for a Windows–Windows pair. Both
defaults can be changed.

The emergency-return shortcut is `Control + Alt + Shift + F12` by default, with
F10 and F11 alternatives. On Mac, Alt corresponds to Option. The edge delay can
be set from 0 to 1000 ms; zero switches immediately.

The audio destination is independent of the input source. Selecting Mac as
destination plays the Windows computer's system audio through the Mac's current
default speakers/headphones. The source continues local playback. TwinGlide does
not mute the source, change system volume/default output, install a virtual
sound card or capture a microphone.

Only one audio direction is active at a time to prevent feedback loops. The
40/80/160 ms values are target buffers, not promises of total end-to-end latency.
Protected media, exclusive-mode audio and Bluetooth devices may affect capture
or latency. macOS uses ScreenCaptureKit capture and AVAudioEngine playback;
Windows uses WASAPI loopback capture and shared-mode playback.

Input and audio settings synchronize to the other computer. Appearance and login
startup stay local. Each settings page saves its own changes; switching pages
does not submit or discard a draft. Full details: [Settings](docs/SETTINGS.md).

## Networking and trust

- Discovery uses IPv4 LAN broadcasts. No cloud registration, relay or public
  port forwarding is required; internet remote access is not provided.
- Advertised names are discovery hints, not proof of identity. Trust is stored
  only after both ends approve a code bound to the current TLS connection.
- Control and audio use TLS 1.3 with saved SHA-256 certificate fingerprints.
  Input uses a separate TLS-enabled Deskflow process.
- Private keys and pairing data stay in the user's local profile. There is no
  telemetry upload or general remote-command endpoint.
- LAN ports: UDP 59120 for discovery, TCP 59121 for control/audio, and TCP 59122
  for input. Do not expose these ports to the public internet.

See the [protocol](docs/PROTOCOL.md) and [security policy](SECURITY.md).

## Troubleshooting

| Symptom | Check |
| --- | --- |
| No nearby computer | Open both apps; check mutual LAN reachability, guest Wi-Fi/client isolation, VLANs and app firewall rules. Manual IPv4 connection under Advanced options is a troubleshooting fallback |
| Pairing expires or completes on one side only | Reopen pairing on both ends, reconnect, compare the entire code and confirm on both |
| Mac is discovered but cannot connect | Allow TwinGlide in Privacy & Security → Local Network, then quit and reopen the app |
| Connected but unable to control Mac | Check Accessibility permission for TwinGlide and TwinGlide Input if listed |
| Existing input engine detected | TwinGlide pauses input takeover. Decide whether to exit the existing sharing app yourself; TwinGlide does not terminate or reconfigure it |
| No forwarded audio | Select and save a destination, check nonzero receive volume and Mac capture permission; protected/exclusive audio may not be captured |
| Audio stutters | Try an 80 or 160 ms buffer and check LAN stability and Bluetooth device latency |
| Connection survives closing settings | Expected tray/menu-bar behavior; use Exit TwinGlide to stop the application |

## Installation isolation and removal

TwinGlide has its own names, ports, certificates, data directories and login
entries. It does not import another application's identities, replace Deskflow
services or stop unrelated processes. Existing `deskflow-core` processes suspend
TwinGlide input takeover until the user exits the conflicting application.
Existing TwinGlide paired identities can be retained when upgrading.

| Item | Windows | macOS |
| --- | --- | --- |
| Application | `%LOCALAPPDATA%/Programs/TwinGlide` | `~/Applications/TwinGlide.app` |
| Private profile | `%LOCALAPPDATA%/TwinGlide` | `~/Library/Application Support/TwinGlide` |
| Login entry | Current-user `Startup/TwinGlide.lnk` | `org.twinglide.desktop.plist` |

Uninstall with `Uninstall.ps1` on Windows or `Uninstall.command` on Mac. Pairing
data is retained. For complete removal, exit the app first, then remove your own
private profile directory shown above.

## Build from source

Requirements: Go 1.24+ with a current security patch; Windows PowerShell and the
.NET Framework compiler for Windows; Python 3 and Xcode Command Line Tools for
Mac. Build on the target operating system. Network access to the official
Deskflow release downloads is required.

```sh
git clone https://github.com/FantasticJerryFox/TwinGlide.git
cd TwinGlide
```

Windows:

```powershell
go test ./...
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-windows.ps1
```

Mac:

```sh
go test -race ./...
python3 scripts/build-mac.py
```

Builds write to `dist/` and `engine/cache/`; they do not install the application
or activate input sharing. The Go coordinator has no third-party Go modules.
`engine/pins.json` fixes official input-engine versions and SHA-256 hashes.
Local packages include the engine and its required runtime files; end users of
such a package do not need Go, Python or a separate Deskflow installation.

For isolated coordinator development:

```sh
go run ./cmd/twinglide --data-dir .test-data/dev --no-input
```

`--no-input` disables physical input, audio capture/playback and login-registration
changes. For native-window testing, set `TWINGLIDE_DATA_DIR` to a dedicated test
directory and `TWINGLIDE_TEST_MODE=1` before launching the desktop executable.
The coordinator command alone does not open a GUI. `--audio-test` also forces
no-input mode and uses synthetic audio fixtures without opening audio devices.

Run `go fmt ./...`, `go vet ./...` and `go test ./...` for changes. Use the race
detector on Mac/Linux for coordination changes. Never commit private identities,
pairing state, endpoint files, credentials or raw logs.

## Project structure

| Directory | Responsibility |
| --- | --- |
| `cmd/twinglide/` | Coordinator entry point |
| `internal/link/` | Discovery, identities, TLS pairing, reconnection, settings, engine lifecycle and authenticated local API |
| `windows/` | WinForms window/tray, WASAPI helper, installation and app-scoped firewall configuration |
| `mac/` | AppKit window/menu bar, input-source restoration and native audio helpers |
| `assets/` | Original SVG, PNG and ICO assets |
| `scripts/` | Native builds, icon generation and allowlisted source export |
| `docs/` | Protocol, settings, permissions, validation and release process |

## License and contributions

TwinGlide application code is **GPL-2.0-only**. Upstream components retain their
own terms; see [LICENSE](LICENSE) and [third-party notices](THIRD_PARTY_NOTICES.md).
Deskflow is a separate upstream input engine, not an original TwinGlide component.
Public bundled binaries require the corresponding dependency source/build
materials described in the [release process](docs/RELEASING.md).

Issues should include both OS versions and architectures, app version,
reproduction steps and a redacted error summary. See [Contributing](CONTRIBUTING.md).
Report vulnerabilities privately through the repository's security reporting
channel; do not put credentials or exploitable details in public issues.
