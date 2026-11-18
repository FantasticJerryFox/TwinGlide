#!/usr/bin/env python3
"""Build only inside dist/. Never install or activate a sharing process."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import plistlib
import shutil
import subprocess
import tempfile
import urllib.request

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument("--go", default="go")
args = parser.parse_args()
if platform.system() != "Darwin":
    raise SystemExit("Build the macOS desktop package on macOS.")
arch = "arm64" if platform.machine() == "arm64" else "amd64"
swift_arch = "arm64" if arch == "arm64" else "x86_64"
pin = json.loads((root / "engine/pins.json").read_text())[f"darwin-{arch}"]
cache = root / "engine/cache"
cache.mkdir(parents=True, exist_ok=True)
asset = cache / pin["asset"]
if not asset.exists():
    urllib.request.urlretrieve(f'https://github.com/deskflow/deskflow/releases/download/v{pin["version"]}/{pin["asset"]}', asset)
if hashlib.sha256(asset.read_bytes()).hexdigest() != pin["sha256"]:
    raise SystemExit("Deskflow archive checksum mismatch")
out = root / "dist" / f"TwinGlide-macos-{arch}"
app = out / "TwinGlide.app"
contents = app / "Contents"
binary = contents / "MacOS"
resources = contents / "Resources"
binary.mkdir(parents=True, exist_ok=True)
resources.mkdir(exist_ok=True)
def run(command, **kwargs):
    subprocess.run(command, check=True, cwd=root, **kwargs)
run([args.go, "test", "./..."])
run([args.go, "build", "-trimpath", "-ldflags", "-s -w", "-o", str(binary / "twinglide-agent"), "./cmd/twinglide"])
run(["xcrun", "swiftc", "-swift-version", "5", "-O", "-target", f"{swift_arch}-apple-macosx13.0",
     str(root / "mac/TwinGlide.swift"), "-o", str(binary / "TwinGlide"), "-framework", "Cocoa"])
run(["xcrun", "swiftc", "-swift-version", "5", "-O", "-target", f"{swift_arch}-apple-macosx13.0",
     str(root / "mac/InputSourceGuard.swift"), "-o", str(binary / "InputSourceGuard"), "-framework", "Cocoa", "-framework", "Carbon"])
run([str(binary / "InputSourceGuard"), "--self-test"])
run(["xcrun", "swiftc", "-swift-version", "5", "-O", "-target", f"{swift_arch}-apple-macosx13.0",
     str(root / "mac/Audio.swift"), "-o", str(binary / "TwinGlideAudio"),
     "-framework", "AVFoundation", "-framework", "ScreenCaptureKit", "-framework", "CoreMedia"])
run([str(binary / "TwinGlideAudio"), "--self-test"])
info = dict(CFBundleName="TwinGlide", CFBundleDisplayName="TwinGlide 双游",
            CFBundleIdentifier="org.twinglide.desktop", CFBundleExecutable="TwinGlide",
            CFBundlePackageType="APPL", CFBundleVersion="5", CFBundleShortVersionString="0.1.0", CFBundleIconFile="TwinGlide",
            LSMinimumSystemVersion="13.0", LSUIElement=True,
            NSLocalNetworkUsageDescription="发现并连接你确认配对的电脑，共享键盘与鼠标。",
            NSAudioCaptureUsageDescription="仅在你开启声音转发时，把系统声音发送到已配对的电脑。",
            NSAppTransportSecurity={"NSAllowsLocalNetworking": True},
            NSHighResolutionCapable=True)
(contents / "Info.plist").write_bytes(plistlib.dumps(info))
with tempfile.TemporaryDirectory(prefix="twinglide-icon-") as icon_temp:
    iconset = Path(icon_temp) / "TwinGlide.iconset"
    iconset.mkdir()
    for size in [16, 32, 128, 256, 512]:
        for scale in [1, 2]:
            dest = iconset / f"icon_{size}x{size}{'@2x' if scale == 2 else ''}.png"
            run(["sips", "-z", str(size * scale), str(size * scale), str(root / "assets/TwinGlide.png"), "--out", str(dest)], stdout=subprocess.DEVNULL)
    run(["iconutil", "-c", "icns", str(iconset), "-o", str(resources / "TwinGlide.icns")])
helper = contents / "Helpers/InputEngine.app/Contents"
helper.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory(prefix="twinglide-dmg-") as mount:
    run(["hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", mount, str(asset)],
        input=b"Y\n", stdout=subprocess.DEVNULL)
    try:
        source = Path(mount) / "Deskflow.app/Contents"
        for name in ["Frameworks", "PlugIns", "Resources"]:
            run(["ditto", str(source / name), str(helper / name)])
        (helper / "MacOS").mkdir(exist_ok=True)
        shutil.copy2(source / "MacOS/deskflow-core", helper / "MacOS/deskflow-core")
        (helper / "Info.plist").write_bytes(plistlib.dumps(dict(CFBundleName="TwinGlide Input",
            CFBundleIdentifier="org.twinglide.input", CFBundleExecutable="deskflow-core",
            CFBundlePackageType="APPL", CFBundleVersion=pin["version"], LSUIElement=True,
            NSLocalNetworkUsageDescription="连接你在双游中确认配对的电脑，接收或发送键盘鼠标操作。")))
    finally:
        run(["hdiutil", "detach", mount], stdout=subprocess.DEVNULL)
for name in ["LICENSE", "THIRD_PARTY_NOTICES.md", "README.md"]:
    shutil.copy2(root / name, resources / name)
shutil.copytree(root / "licenses", resources / "licenses", dirs_exist_ok=True)
(resources / "input-engine.json").write_text(json.dumps(pin, indent=2)+"\n")
# Ad-hoc development signature. A public notarized release uses Developer ID in CI.
run(["codesign", "--force", "--sign", "-", "--identifier", "org.twinglide.input", str(helper.parent)])
for name in ["twinglide-agent", "InputSourceGuard", "TwinGlideAudio", "TwinGlide"]:
    run(["codesign", "--force", "--sign", "-", str(binary / name)])
run(["codesign", "--force", "--sign", "-", "--identifier", "org.twinglide.desktop", str(app)])
run(["codesign", "--verify", "--deep", "--strict", str(app)])
for name in ["Install.command", "Uninstall.command"]:
    shutil.copy2(root / "mac" / name, out / name)
    (out / name).chmod(0o755)
for name in ["LICENSE", "THIRD_PARTY_NOTICES.md", "README.md", "README.en.md", "SECURITY.md"]:
    shutil.copy2(root / name, out / name)
for name in ["licenses", "docs"]:
    shutil.copytree(root / name, out / name, dirs_exist_ok=True)
archive = root / "dist" / f"TwinGlide-0.1.0-alpha.5-macos-{arch}.zip"
run(["ditto", "-c", "-k", "--sequesterRsrc", "--keepParent", str(out), str(archive)])
print(str(archive), hashlib.sha256(archive.read_bytes()).hexdigest())
