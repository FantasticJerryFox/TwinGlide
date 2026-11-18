package link

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersonalSettingsPersistWithoutLeakingLocalPreferences(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	approve(t, b)
	paired(t, a, b)
	if a.post(t, "preferences", map[string]any{"name": "工作电脑", "theme": "dark", "startAtLogin": false}) != 200 {
		t.Fatal("preferences rejected")
	}
	waitFor(t, 5*time.Second, func() bool { b.a.mu.Lock(); defer b.a.mu.Unlock(); return b.a.config.Peer.Name == "工作电脑" })
	a.a.mu.Lock()
	p := a.a.config.Preferences
	a.a.mu.Unlock()
	if p.Theme != "dark" || p.StartAtLogin {
		t.Fatal("local preferences not applied")
	}
	b.a.mu.Lock()
	remote := b.a.config.Preferences
	b.a.mu.Unlock()
	if remote.Theme != "system" || !remote.StartAtLogin {
		t.Fatal("local preferences leaked to peer")
	}
	c, err := readConfig(a.dir)
	if err != nil || c.Name != "工作电脑" || c.Preferences != p {
		t.Fatal("preferences not persisted", err)
	}
	if a.post(t, "preferences", map[string]any{"name": "", "theme": "dark"}) != 409 {
		t.Fatal("empty name accepted")
	}
}
func TestExtendedSettingsSyncAndRejectInvalidAudio(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	approve(t, b)
	paired(t, a, b)
	body := map[string]any{"sourceID": a.a.id, "side": "up", "enabled": false, "edgeDelayMS": 250, "returnKey": "F10", "audioTarget": b.a.id, "audioVolume": 35, "audioBufferMS": 160}
	if a.post(t, "settings", body) != 200 {
		t.Fatal("settings rejected")
	}
	waitFor(t, 3*time.Second, func() bool {
		b.a.mu.Lock()
		defer b.a.mu.Unlock()
		s := b.a.config.Shared
		return s.Side == "up" && s.EdgeDelayMS == 250 && s.ReturnKey == "F10" && s.AudioTarget == b.a.id && s.AudioVolume == 35 && s.AudioBufferMS == 160
	})
	for _, field := range []string{"audioTarget", "audioVolume", "audioBufferMS", "edgeDelayMS", "returnKey"} {
		invalid := map[string]any{}
		for k, v := range body {
			invalid[k] = v
		}
		switch field {
		case "audioTarget":
			invalid[field] = strings.Repeat("f", 64)
		case "audioVolume":
			invalid[field] = 101
		case "audioBufferMS":
			invalid[field] = 1
		case "edgeDelayMS":
			invalid[field] = -1
		case "returnKey":
			invalid[field] = "F12)\nend"
		}
		if a.post(t, "settings", invalid) != 409 {
			t.Fatal("invalid field accepted", field)
		}
	}
	a.a.mu.Lock()
	key, _, _ := a.a.audioRouteLocked()
	a.a.mu.Unlock()
	if key != "" {
		t.Fatal("no-input mode allowed physical audio")
	}
}
func TestLegacySettingsMigrationDefaultsToAudioOff(t *testing.T) {
	dir := t.TempDir()
	raw := "{\"version\":1,\"name\":\"Old device\",\"shared\":{\"side\":\"left\",\"enabled\":true}}"
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := readConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Shared.AudioTarget != "" || c.Shared.AudioVolume != 100 || c.Shared.AudioBufferMS != 80 || c.Shared.ReturnKey != "F12" || c.Preferences.Theme != "system" {
		t.Fatal("unsafe legacy defaults")
	}
}
func TestLayoutDirectionsDelayAndHotkey(t *testing.T) {
	dir := t.TempDir()
	if _, err := readIdentity(dir); err != nil {
		t.Fatal(err)
	}
	local, peer := strings.Repeat("a", 64), strings.Repeat("b", 64)
	for side, opposite := range map[string]string{"left": "right", "right": "left", "up": "down", "down": "up"} {
		s := defaultSettings()
		s.SourceID = local
		s.Actor = local
		s.Revision = 1
		s.Side = side
		s.EdgeDelayMS = 250
		s.ReturnKey = "F11"
		path, _, err := writeEngineConfig(dir, filepath.Join(dir, "runtime", "deskflow-core.exe"), local, Peer{ID: peer, Address: "192.0.2.10:59121"}, s)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, "input", "layout.conf"))
		config, _ := os.ReadFile(path)
		for _, expected := range []string{side + " = tg-" + peer[:16], opposite + " = tg-" + local[:16], "Control+Alt+Shift+F11"} {
			if !strings.Contains(string(raw), expected) {
				t.Fatal("layout missing", expected)
			}
		}
		if !strings.Contains(string(config), "enableSwitchDelay=true\nswitchDelay=250") {
			t.Fatal("engine setting not applied")
		}
		for _, id := range []string{local, peer} {
			name := "tg-" + id[:16]
			if !strings.Contains(string(config), "[computer_"+name+"]\nname="+name) {
				t.Fatal("Deskflow 1.27 computer registration absent")
			}
		}
		if strings.Contains(string(raw), "section: screens") {
			t.Fatal("obsolete Deskflow screen section")
		}
	}
}
func TestAudioFramingRejectsMalformedAndOversizedFrames(t *testing.T) {
	format := []byte{1, 0x80, 0xbb, 0, 0, 2, 0}
	pcm := make([]byte, 17)
	pcm[0] = 2
	var stream bytes.Buffer
	for _, b := range [][]byte{format, pcm, {3}} {
		if err := writeAudioFrame(&stream, b); err != nil {
			t.Fatal(err)
		}
	}
	seen := false
	for i := 0; i < 3; i++ {
		if _, err := readAudioFrame(&stream, &seen); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		frame []byte
		seen  bool
	}{{pcm, false}, {format, true}, {[]byte{2, 0}, true}, {[]byte{4}, true}, {[]byte{1, 0, 0, 0, 0, 2, 0}, false}} {
		stream.Reset()
		_ = writeAudioFrame(&stream, tc.frame)
		seen = tc.seen
		if _, err := readAudioFrame(&stream, &seen); err == nil {
			t.Fatal("malformed audio accepted")
		}
	}
	stream.Reset()
	_ = binary.Write(&stream, binary.LittleEndian, uint32(65538))
	seen = false
	if _, err := readAudioFrame(&stream, &seen); err == nil {
		t.Fatal("oversize accepted")
	}
}
func TestAudioRouteRequiresConfirmedPeerAndExplicitTarget(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	approve(t, b)
	paired(t, a, b)
	a.a.mu.Lock()
	defer a.a.mu.Unlock()
	a.a.opt.AudioTest = true
	key, _, _ := a.a.audioRouteLocked()
	if key != "" {
		t.Fatal("audio enabled by default")
	}
	a.a.config.Shared.AudioTarget = b.a.id
	key, _, _ = a.a.audioRouteLocked()
	if key == "" {
		t.Fatal("explicit synthetic route missing")
	}
	a.a.session.revoked = true
	key, _, _ = a.a.audioRouteLocked()
	if key != "" {
		t.Fatal("revoked peer authorized")
	}
}
