package link

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func (a *App) updatePreferences(name string, p Preferences) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 40 || cleanName(name) != name {
		return errors.New("设备名称需要 1–40 个字符，不能包含控制字符")
	}
	if p.Theme != "system" && p.Theme != "light" && p.Theme != "dark" {
		return errors.New("无效的外观设置")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.config
	changed := p.StartAtLogin != old.Preferences.StartAtLogin
	// No-input mode is a full physical-I/O sandbox, including login registration.
	if changed && !a.opt.NoInput {
		if err := setStartAtLogin(p.StartAtLogin); err != nil {
			return fmt.Errorf("无法修改登录启动：%w", err)
		}
	}
	a.config.Name = name
	a.config.Preferences = p
	if err := a.saveLocked(); err != nil {
		a.config = old
		if changed && !a.opt.NoInput {
			_ = setStartAtLogin(old.Preferences.StartAtLogin)
		}
		return fmt.Errorf("无法保存通用设置：%w", err)
	}
	return nil
}

func setStartAtLogin(enabled bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "windows":
		ui := filepath.Join(filepath.Dir(exe), "TwinGlide.exe")
		if _, err = os.Stat(ui); err != nil {
			return err
		}
		quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
		script := "$ErrorActionPreference='Stop'; $path=Join-Path ([Environment]::GetFolderPath('Startup')) 'TwinGlide.lnk';"
		if enabled {
			script += "$s=(New-Object -ComObject WScript.Shell).CreateShortcut($path);$s.TargetPath=" + quote(ui) + ";$s.Arguments='--background';$s.WorkingDirectory=" + quote(filepath.Dir(exe)) + ";$s.Save()"
		} else {
			script += "if(Test-Path -LiteralPath $path){Remove-Item -LiteralPath $path}"
		}
		var b bytes.Buffer
		for _, u := range utf16.Encode([]rune(script)) {
			_ = binary.Write(&b, binary.LittleEndian, u)
		}
		c := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b.Bytes()))
		hideCommand(c)
		if output, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("%w: %s", err, output)
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path := filepath.Join(home, "Library", "LaunchAgents", "org.twinglide.desktop.plist")
		if !enabled {
			err = os.Remove(path)
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		ui := filepath.Join(filepath.Dir(exe), "TwinGlide")
		if _, err = os.Stat(ui); err != nil {
			return err
		}
		text := macLoginAgentPlist(ui)
		return atomicWrite(path, []byte(text))
	default:
		return errors.New("当前系统没有桌面登录启动支持")
	}
	return nil
}

// The association lets macOS attribute a login agent's network requests to the
// same application whose Local Network choice the user approved.
func macLoginAgentPlist(ui string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?><!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\"><plist version=\"1.0\"><dict><key>Label</key><string>org.twinglide.desktop</string><key>AssociatedBundleIdentifiers</key><array><string>org.twinglide.desktop</string></array><key>ProgramArguments</key><array><string>" + html.EscapeString(ui) + "</string><string>--background</string></array><key>RunAtLoad</key><true/><key>LimitLoadToSessionType</key><string>Aqua</string><key>ProcessType</key><string>Interactive</string></dict></plist>"
}
