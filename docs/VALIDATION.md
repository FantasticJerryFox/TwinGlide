# Validation

This file records observed outcomes, not planned tests.

## Alpha.5 Windows native redesign, 2026-11-17

- Replaced the Windows top navigation and independently spaced pages with a
  fixed sidebar, shared field columns, semantic light/dark colors and a persistent
  save area. The screen preview distinguishes the keyboard source and the other
  computer, including vertical arrangements. Native controls and system window
  chrome remain; no browser or web view was introduced.
- Applied the public Anthropic frontend-design skill to the existing WinForms
  application. The design contract and source link are in WINDOWS_UI.md.
- Added .NET Framework 4.7.2 targeting, a Windows 10 compatibility manifest and
  the documented PerMonitorV2 configuration. These are included in the build
  and source export. Changing system DPI or a contrast theme was not part of
  this workstation test; those configurations still need hardware acceptance.
- The Windows build, static analysis, 25 ordinary coordinator tests and the
  separate existing-engine read-only test passed. Audio framing self-test passed
  without opening an audio device. Mac code was not redesigned or rebuilt in
  this update; the previously verified Mac package remains alpha.4.
- UI checks used two isolated Windows coordinator profiles with input/audio
  disabled. The matching-code state, connection and offline states were shown.
  The native screen-direction selector changed the preview to vertical; clicking
  Save synchronized the direction to the other test coordinator. The final
  alpha.5 launcher reconnected and retained this setting after restart.
- Visually inspected the four native pages in light and dark appearance. At
  the 840-by-660 minimum outer window size, labels and fields remained aligned;
  long settings pages scrolled while the save action stayed visible. The final
  dark controls and title bar were checked after rebuilding the launcher.
  Saving the appearance through the native UI persisted dark mode. Saving
  receive volume at 95% synchronized that value to both isolated coordinators;
  no system audio was captured or played during these UI checks.
- Stopped both isolated test coordinators and the native test window, then
  removed their temporary firewall rules. Original keyboard/audio services
  remained running with their original startup modes, and all three original
  Windows configuration/source hashes matched the pre-test baseline. The Mac
  installation and running processes were not changed during this UI update.

## Alpha.4 permissions and Windows-source acceptance, 2026-11-17

- Reproduced the Mac outbound failure with a normal LaunchServices app launch
  and isolated profiles, while command-line TCP probes reached the same Windows
  listener. After the user allowed Local Network access and the Mac app was
  restarted, the Mac initiated the real TLS pairing connection successfully.
- Switched to actual input with Windows as source and Mac on its right. TLS
  succeeded, but Mac initially reported `failed to create quartz event tap` and
  `process is not trusted anymore`. After the user granted Accessibility access,
  the client automatically retried and both native input engines connected.
- The user physically tested the Windows-attached mouse and confirmed crossing
  into Mac, movement, clicking and returning to Windows. The native engine logs
  also recorded three completed handoff/return cycles. Combined with alpha.3,
  both Mac-source and Windows-source physical mouse use have now been accepted.
- Added a Mac Local Network shortcut and permission guide. Connection errors
  distinguish unreachable/permission-related failures from timeouts, without
  treating a socket error alone as proof of a privacy denial. Accessibility
  errors now retain their specific recovery instruction after the input child
  exits. Native windows clear a previous connection error after recovery.
- Mac login-agent definitions now associate `org.twinglide.desktop` as their
  responsible app; the nested input app includes a network usage description.
  A complete OS-login/reboot test remains outstanding.
- All 26 coordinator tests passed on Windows and Intel macOS, including the
  existing-engine read-only check and four new error/permission/login-agent
  regressions. Static analysis passed on both; the Mac race run reported no race.
- Final alpha.4 native builds were checked with input/audio disabled. The Mac
  initiated the encrypted connection from a normal LaunchServices launch, and
  restarting it reconnected the saved pair without confirmation. A deliberately
  refused local connection displayed the Windows recovery message; successful
  reconnection cleared it. Both native device pages and the new Mac permission
  button were visually checked. The Mac initially showed a network-access error
  after launch, then recovered; no additional permission change was made by the
  test automation during this final check.
- Original sharing processes/services were restored after the physical test.
  Original file hashes and Mac system volume matched the fresh pre-test snapshot.
  Temporary test firewall rules were removed. Only the new TwinGlide permissions
  explicitly granted by the user were retained.

Two physical Windows computers are still unavailable. Windows↔Windows has
coordinator pairing/role tests, and the Windows input engine has been exercised
as both source and receiver against Mac; this is not two-Windows hardware acceptance.

## Alpha.3 native desktop and real I/O, 2026-11-17

- Replaced the browser UI and Mac web view with native WinForms and AppKit
  windows. The local agent serves authenticated JSON only; the root URL is 404.
- First-launch random nicknames persist without pairing and preserve later
  custom names. Nickname-based LAN discovery needs no IP entry; manual IP is an
  advanced troubleshooting option.
- Native UI acceptance used fresh, isolated profiles on Windows and Intel Mac.
  Windows displayed the Mac's generated nickname; clicking it produced matching
  codes on both native windows. Confirming each window established the session.
  No IP address was entered. A Windows JSON-array decoding defect found during
  this check was fixed before packaging.
- Windows native controls saved an upward screen layout, audio output to the
  named Windows computer, a newly randomized nickname and light appearance.
  Mac received the layout, audio target and new name. Restarting the Windows
  desktop app preserved identity/settings and reconnected without confirmation.
  All these UI checks used `--no-input`; they did not activate input, audio or
  login registration. Light/dark Windows layouts, the 690-by-670 minimum window
  with scrollable settings, and the Mac minimum window were visually checked.
- Coordinator validation: all 22 tests (including the opt-in existing-engine
  read-only check) passed on Windows and Intel macOS. The Mac run used the race
  detector with no race reported. Static analysis passed on both platforms.
  Native audio/helper self-tests passed without opening an audio device.
- Both platforms now bundle the pinned official Deskflow 1.27.0. Updated the
  computer registry/configuration format and the input-source guard's event
  names. This fixed the actual Mac-to-Windows first-movement/return failure
  found with the former 1.26 Mac engine.
- Four real OS-event handoff tests passed with Mac as input source. Windows
  entry coordinates were left (2559,720), right (0,720), up (1280,1439), and down
  (1280,0). Eight subsequent 24-pixel motions produced exactly 192 pixels on the
  expected axis, and crossing the reciprocal edge returned to Mac. One downward
  trial differed during concurrent desktop activity; the isolated rerun passed.
  Concurrent input is a possible cause, not a proven diagnosis. These are real
  input-engine/desktop tests, not a physical USB fixture.
- The user then tested the physical Mac mouse with Windows on the left and
  confirmed crossing, continuous movement, clicking and returning all worked.
- Actual stereo audio passed in both directions. A five-second 48 kHz stereo
  tone (523.25 Hz left / 659.25 Hz right) was played through the source system
  output, captured by the production helper, encrypted across the LAN, played
  by the receiver, and independently captured at the receiver's system output.
  Both tones were detected at about 0.0419 amplitude with 35% receive gain.
  Signal-to-comparison-tone ratios exceeded 56 dB on both channels. No microphone
  was used. This verifies the output pipeline, not acoustic speaker response or
  latency across all devices.
- Windows input-server startup succeeded. Mac client connection in that role
  reported "No route to host"; role reversal is **not** an accepted real-machine
  result in this original run. This failure is resolved by the alpha.4 acceptance
  above; clean-install coverage remains outstanding.
- The same-platform integration test runs two coordinators of the current OS,
  pairs with mutual approval, selects the initiating device as source, and
  synchronizes reversed input/audio roles. On Windows this covers Windows↔Windows
  control behavior. A second physical Windows machine was not available, so no
  two-Windows hardware acceptance is claimed.
- The original sharing software was retained. Its Mac parent and input-source
  guard were suspended temporarily to preserve in-memory preferences; its
  Windows services were stopped temporarily. After the test, original Mac
  processes resumed, both Windows services were running with their original
  startup modes, old keyboard/audio connections recovered, configuration hashes
  and Mac system volume were unchanged, and temporary firewall rules were removed.

The physical test used already authorized remote-login permissions on the Mac.
It does not replace first-install Accessibility/local-network/audio permission
testing through a normal Finder launch.

## Alpha.2 settings and audio, 2026-11-17

- Windows x64 and Intel macOS: 19 coordinator tests passed, including the
  existing-engine read-only check. The Mac race-detector run passed.
- Both native audio helpers compiled and passed framing self-tests without
  opening any audio device.
- Eleven live LAN assertions passed: bidirectional discovery, saved-pair
  reconnection, all four screen directions, native synthetic Windows→Mac and
  Mac→Windows audio with exact sample verification, audio restoration after
  restarting the Mac agent, stopping both helpers when forwarding is disabled,
  and device-name synchronization without copying local appearance preferences.
- Actual microphone/system-audio capture and speaker playback were not used.
  Production audio device compatibility, permission dialogs, actual latency and
  audible quality remain unverified. Login entries were not changed by tests.
- Both native helpers exited automatically after their agent was forcibly
  terminated during active synthetic transmission; the orphan-process case found
  in the first Windows trial was fixed with explicit parent-process monitoring.
- Screen, Audio and General tabs were exercised through the real settings UI.
  Direction preview/save, audio destination/buffer save and appearance save
  succeeded. Light and dark appearance and the 620×540 minimum window were
  visually checked; no horizontal document overflow was present.

## Alpha.1 baseline, 2026-11-17

Windows x64 and an Intel Mac were used. All coordinator integration runs used
dedicated profiles and the explicit `--no-input` flag.

| Check | Outcome |
| --- | --- |
| Go static analysis | Passed |
| Windows coordinator tests | 13 passed, including an opt-in read-only detection of the existing input engine |
| Intel macOS coordinator tests with race detector | Same 13 passed; no race reported |
| Windows C# tray and native Go build | Passed |
| Intel macOS Swift window, Go agent and input-source helper builds | Passed |
| Input-source helper pure self-tests | Passed |
| macOS ad-hoc signature verification | Passed; this is not Developer ID notarization |
| Settings page at regular and minimum window widths | Visually inspected |
| Existing sharing application | Original processes kept running; three baseline files on each OS retained identical SHA-256 hashes |

The coordinator tests cover two-sided approval, rejection, stale and expired
confirmation, cancellation after one approval, preserved identity across restart,
pinned-identity mismatch, settings synchronization, forgetting a pair, local
HTTP authorization/origin/host checks, damaged stored identity, generated input
configuration, and refusing an unrelated nonempty profile directory.

## Real two-machine checks

Seven assertions passed on the live LAN: discovery in both directions, matching
TLS-bound codes, no trust after only one confirmation, connection after both
confirmations, synchronized source/side/pause settings, automatic reconnection
after restarting the Mac test agent, and no-input mode remaining active throughout.

Temporary test processes were stopped and their Windows firewall rules removed.
No installation, startup entry, certificate or setting of the existing sharing
application was replaced. Private test identities, host names, LAN addresses,
logs and screenshots are excluded from source publication.

## Not yet accepted for a stable release

- Emergency-return shortcut, typing/layout coverage, and two physical Windows
  computers. Physical mouse handoff in both source roles is covered above.
- Fresh-install/uninstall behavior on clean systems, full OS reboot/wake,
  changed DHCP addresses and multiple-monitor edge cases.
- Apple Silicon native runtime and GitHub-hosted CI execution. Build paths are
  provided, but no successful run on those environments is claimed here.
- Public binary source-compliance bundle, Windows signing and macOS notarization.

Windows UAC/login/lock-screen input and clipboard sharing remain outside this
alpha's supported scope. Audio forwarding was added in alpha.2 with the test
limits above. Review RELEASING.md before public binaries.
