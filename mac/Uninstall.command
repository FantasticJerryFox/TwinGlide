#!/bin/zsh
set -euo pipefail
if /usr/bin/pgrep -x TwinGlide >/dev/null; then
  print "请先从菜单栏退出 TwinGlide。"; exit 1
fi
target="$HOME/Applications/TwinGlide.app"
agent="$HOME/Library/LaunchAgents/org.twinglide.desktop.plist"
/bin/launchctl bootout "gui/$(id -u)/org.twinglide.desktop" 2>/dev/null || true
# Only these exact TwinGlide paths are removed. Pairing data is retained.
[[ "$target" == "$HOME/Applications/TwinGlide.app" ]] || exit 1
/bin/rm -rf -- "$target"
/bin/rm -f -- "$agent"
print "双游已卸载；配对数据保留在 Library/Application Support/TwinGlide。"
