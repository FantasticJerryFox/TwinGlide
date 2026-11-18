package link

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacLoginAgentAssociatesTheUserApprovedApplication(t *testing.T) {
	path := "/Applications/Work & Play/TwinGlide.app/Contents/MacOS/TwinGlide"
	raw := macLoginAgentPlist(path)
	var plist struct {
		Dict struct {
			Keys   []string `xml:"key"`
			Arrays []struct {
				Strings []string `xml:"string"`
			} `xml:"array"`
		} `xml:"dict"`
	}
	if err := xml.Unmarshal([]byte(raw), &plist); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "<key>AssociatedBundleIdentifiers</key>") || len(plist.Dict.Arrays) != 2 ||
		len(plist.Dict.Arrays[0].Strings) != 1 || plist.Dict.Arrays[0].Strings[0] != "org.twinglide.desktop" ||
		len(plist.Dict.Arrays[1].Strings) != 2 || plist.Dict.Arrays[1].Strings[0] != path || plist.Dict.Arrays[1].Strings[1] != "--background" {
		t.Fatal("login agent must associate the approved app and preserve its executable path")
	}
}

func TestConnectionFailureDistinguishesNetworkAccessFromOffline(t *testing.T) {
	err := fmt.Errorf("dial failed: %w", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: no route to host")})
	mac := connectionFailure(err, "darwin")
	if !strings.Contains(mac, "本地网络") || !strings.Contains(mac, "同一局域网") || strings.Contains(mac, "等待另一台电脑上线") {
		t.Fatal("missing actionable permission and network guidance", mac)
	}
	win := connectionFailure(err, "windows")
	if strings.Contains(win, "Mac") || !strings.Contains(win, "防火墙") {
		t.Fatal("incorrect platform guidance", win)
	}
	if s := connectionFailure(os.ErrDeadlineExceeded, "darwin"); !strings.Contains(s, "连接超时") {
		t.Fatal("timeout must not be labeled a permission denial", s)
	}
}

func TestInputStatusReportsLatestConnectionOutcome(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0700); err != nil {
		t.Fatal(err)
	}
	e := &engine{app: &App{opt: Options{DataDir: dir}}}
	path := filepath.Join(dir, "logs", "keyboard.log")
	failed := "INFO: connected to server\nWARNING: failed to connect: No route to host\n"
	if err := os.WriteFile(path, []byte(failed), 0600); err != nil {
		t.Fatal(err)
	}
	if s := e.readStatus(); s == "键鼠已连接" || !strings.Contains(s, "局域网") {
		t.Fatal("stale success or missing network guidance", s)
	}
	if err := os.WriteFile(path, []byte(failed+"INFO: connected to server\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if s := e.readStatus(); s != "键鼠已连接" {
		t.Fatal("recovery must replace the prior failure", s)
	}
}

func TestInputStatusExplainsMissingAccessibilityPermission(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "logs", "keyboard.log")
	log := "INFO: clipboard sharing disabled by server\nERROR: failed to create quartz event tap\nFATAL: process is not trusted anymore, quitting\nERROR: core stopped with error code: 1\n"
	if err := os.WriteFile(path, []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	e := &engine{app: &App{opt: Options{DataDir: dir}}}
	if s := e.readStatus(); !strings.Contains(s, "辅助功能授权") || !strings.Contains(s, "TwinGlide") {
		t.Fatal("the actual permission failure must remain actionable after exit", s)
	}
}
