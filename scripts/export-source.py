#!/usr/bin/env python3
"""Produce an allowlisted source archive; no .git, credentials, caches or binaries."""
from pathlib import Path
import hashlib
import re
import zipfile

root = Path(__file__).resolve().parents[1]
roots = ["cmd", "internal", "mac", "windows", "scripts", "licenses", "docs", ".github", "assets"]
files = ["go.mod", ".gitignore", ".gitattributes", "LICENSE", "README.md", "README.en.md",
         "CONTRIBUTING.md", "SECURITY.md", "THIRD_PARTY_NOTICES.md", "engine/pins.json"]
paths = [root / name for name in files]
for name in roots:
    paths.extend(p for p in (root / name).rglob("*") if p.is_file() and "__pycache__" not in p.parts)
allowed = {".go", ".swift", ".cs", ".ps1", ".cmd", ".command", ".py", ".md",
           ".json", ".yml", ".yaml", ".txt", ".mod", ".svg", ".png", ".ico", ".manifest", ".config"}
for path in paths:
    if path.name not in {"LICENSE", ".gitignore", ".gitattributes"} and path.suffix not in allowed:
        raise SystemExit("Unexpected export file: " + str(path.relative_to(root)))
    data = path.read_bytes()
    if re.search(rb"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----", data):
        raise SystemExit("Private key found: " + str(path.relative_to(root)))
    if re.search(rb"10[.]10[.]50[.]|/Users/[a-z0-9_-]+/(?:Projects|Library)|C:[\\/]Users[\\/][A-Z]{3}[\\/]", data):
        raise SystemExit("Private device reference found: " + str(path.relative_to(root)))
out = root / "dist/TwinGlide-0.1.0-alpha.5-source.zip"
out.parent.mkdir(exist_ok=True)
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as archive:
    for path in sorted(paths):
        archive.write(path, "TwinGlide/" + path.relative_to(root).as_posix())
print(out.name, hashlib.sha256(out.read_bytes()).hexdigest(), len(paths), "files")
