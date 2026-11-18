#!/bin/zsh
set -euo pipefail
cd -- "${0:A:h}"
if /usr/bin/pgrep -x TwinGlide >/dev/null; then
  print "请先退出 TwinGlide 后再安装。"; exit 1
fi
mkdir -p "$HOME/Applications" "$HOME/Library/LaunchAgents"
/usr/bin/ditto "TwinGlide.app" "$HOME/Applications/TwinGlide.app"
# Login startup belongs only to TwinGlide, in the current user's GUI session.
agent="$HOME/Library/LaunchAgents/org.twinglide.desktop.plist"
state="$HOME/Library/Application Support/TwinGlide/state.json"
start_at_login=true
if [[ -f "$state" ]]; then
  saved=$(/usr/bin/plutil -extract preferences.startAtLogin raw -o - "$state" 2>/dev/null || true)
  [[ "$saved" == false ]] && start_at_login=false
fi
if [[ "$start_at_login" == true ]]; then
/usr/bin/plutil -create xml1 "$agent"
/usr/libexec/PlistBuddy -c "Add :Label string org.twinglide.desktop" "$agent"
/usr/libexec/PlistBuddy -c "Add :AssociatedBundleIdentifiers array" "$agent"
/usr/libexec/PlistBuddy -c "Add :AssociatedBundleIdentifiers:0 string org.twinglide.desktop" "$agent"
/usr/libexec/PlistBuddy -c "Add :ProgramArguments array" "$agent"
/usr/libexec/PlistBuddy -c "Add :ProgramArguments:0 string $HOME/Applications/TwinGlide.app/Contents/MacOS/TwinGlide" "$agent"
/usr/libexec/PlistBuddy -c "Add :ProgramArguments:1 string --background" "$agent"
/usr/libexec/PlistBuddy -c "Add :RunAtLoad bool true" "$agent"
/usr/libexec/PlistBuddy -c "Add :LimitLoadToSessionType string Aqua" "$agent"
/usr/libexec/PlistBuddy -c "Add :ProcessType string Interactive" "$agent"
/bin/chmod 644 "$agent"
else
  /bin/rm -f -- "$agent"
fi
/usr/bin/open "$HOME/Applications/TwinGlide.app"
print "双游已安装。请在两台电脑上确认配对；Mac 需允许辅助功能。"
