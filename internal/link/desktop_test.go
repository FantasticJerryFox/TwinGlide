package link

import (
	"net/http"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestFirstLaunchNicknamePersistsWithoutPairing(t *testing.T) {
	dir := t.TempDir()
	a, err := readConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile("^[^[:ascii:]]{4}-[A-Z2-7]{4}$").MatchString(a.Name) {
		t.Fatal("first launch must use a friendly nickname")
	}
	b, err := readConfig(dir)
	if err != nil || a.Name != b.Name {
		t.Fatal("nickname changed after restart", err)
	}
	b.Name = "My chosen workstation"
	if err := writeJSON(filepath.Join(dir, "state.json"), b); err != nil {
		t.Fatal(err)
	}
	c, err := readConfig(dir)
	if err != nil || c.Name != b.Name {
		t.Fatal("existing nickname overwritten", err)
	}
}

// On Windows this runs two Windows coordinators, with actual mutual TLS and
// two-sided approval. It does not claim two physical Windows input desktops.
func TestSamePlatformPairingAndRoleReversal(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	approve(t, b)
	paired(t, a, b)
	for _, h := range []*harness{a, b} {
		h.a.mu.Lock()
		source, peerOS := h.a.config.Shared.SourceID, h.a.config.Peer.OS
		h.a.mu.Unlock()
		if source != a.a.id || peerOS != localOS() {
			t.Fatal("same-platform initiator must be the initial input source")
		}
	}
	settings := defaultSettings()
	settings.SourceID, settings.Side = b.a.id, "right"
	if a.post(t, "settings", map[string]any{
		"sourceID": settings.SourceID, "side": settings.Side, "enabled": true,
		"audioTarget": a.a.id, "audioVolume": 35, "audioBufferMS": 80,
	}) != 200 {
		t.Fatal("same-platform settings rejected")
	}
	waitFor(t, time.Second, func() bool {
		b.a.mu.Lock()
		defer b.a.mu.Unlock()
		return b.a.config.Shared.SourceID == b.a.id &&
			b.a.config.Shared.AudioTarget == a.a.id && b.a.config.Shared.Side == "right"
	})
}

func TestNativeAPIHasNoWebSurface(t *testing.T) {
	h := startApp(t, "", 0)
	res, err := http.Get(h.ep.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatal("desktop agent must not serve a webpage")
	}
	before, err := readConfig(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	if h.post(t, "nickname", map[string]any{}) != 200 {
		t.Fatal("nickname suggestion failed")
	}
	after, err := readConfig(h.dir)
	if err != nil || before.Name != after.Name {
		t.Fatal("nickname suggestion must wait for Save before renaming")
	}
}
