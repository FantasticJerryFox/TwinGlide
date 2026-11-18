package link

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type harness struct {
	a      *App
	ep     Endpoint
	cancel context.CancelFunc
	done   chan error
	dir    string
}

func startApp(t *testing.T, dir string, port int) *harness {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	a, e := New(Options{DataDir: dir, PeerPort: port, DiscoveryPort: 59120, NoInput: true, NoDiscovery: true})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	h := &harness{a: a, cancel: cancel, done: done, dir: dir}
	select {
	case <-a.ready:
	case e := <-done:
		cancel()
		t.Fatalf("startup failed: %v", e)
	case <-time.After(8 * time.Second):
		cancel()
		t.Fatal("startup timeout")
	}
	waitFor(t, 8*time.Second, func() bool {
		raw, e := os.ReadFile(filepath.Join(dir, "endpoint.json"))
		return e == nil && json.Unmarshal(raw, &h.ep) == nil && h.ep.Token != ""
	})
	t.Cleanup(func() { h.stop(t) })
	return h
}
func (h *harness) stop(t *testing.T) {
	t.Helper()
	if h.cancel == nil {
		return
	}
	h.cancel()
	h.cancel = nil
	select {
	case e := <-h.done:
		if e != nil {
			t.Error(e)
		}
	case <-time.After(8 * time.Second):
		t.Error("shutdown timed out")
	}
}
func waitFor(t *testing.T, d time.Duration, p func() bool) {
	t.Helper()
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if p() {
			return
		}
		time.Sleep(35 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}
func (h *harness) post(t *testing.T, path string, body any) int {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", h.ep.URL+"/api/"+path, bytes.NewReader(raw))
	req.Header.Set("X-TwinGlide-Token", h.ep.Token)
	client := http.Client{Timeout: 3 * time.Second}
	r, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	_, _ = io.Copy(io.Discard, r.Body)
	return r.StatusCode
}
func (h *harness) session() (*session, bool) {
	h.a.mu.Lock()
	defer h.a.mu.Unlock()
	s := h.a.session
	if s == nil {
		return nil, false
	}
	copy := session{id: s.id, code: s.code, paired: s.paired, approved: s.approved, peer: s.peer}
	return &copy, true
}
func beginPair(t *testing.T, a, b *harness) {
	t.Helper()
	b.a.mu.Lock()
	port := b.a.peerPort
	b.a.mu.Unlock()
	go a.a.connect(context.Background(), Nearby{Address: fmtAddress(port)})
	waitFor(t, 4*time.Second, func() bool {
		x, ok := a.session()
		y, yes := b.session()
		return ok && yes && x.code != "" && y.code != ""
	})
	x, _ := a.session()
	y, _ := b.session()
	if x.code != y.code {
		t.Fatal("TLS exporter codes differ")
	}
	if len(x.code) != 14 {
		t.Fatal("expected 48-bit grouped pairing code")
	}
}
func fmtAddress(port int) string { return "127.0.0.1:" + strconvItoa(port) }
func strconvItoa(v int) string   { b, _ := json.Marshal(v); return string(b) }
func approve(t *testing.T, h *harness) {
	t.Helper()
	s, _ := h.session()
	if h.post(t, "confirm", map[string]any{"session": s.id, "match": true}) != 200 {
		t.Fatal("approval rejected")
	}
}
func paired(t *testing.T, a, b *harness) {
	t.Helper()
	waitFor(t, 4*time.Second, func() bool { x, ok := a.session(); y, yes := b.session(); return ok && yes && x.paired && y.paired })
}
func TestPairRequiresBothConfirmationsAndPersists(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	time.Sleep(150 * time.Millisecond)
	a.a.mu.Lock()
	saved := a.a.config.Peer
	a.a.mu.Unlock()
	if saved != nil {
		t.Fatal("single approval persisted trust")
	}
	approve(t, b)
	paired(t, a, b)
	ca, e := readConfig(a.dir)
	if e != nil || ca.Peer == nil || ca.Peer.ID != b.a.id {
		t.Fatal("trust not persisted", e)
	}
	if ca.Shared.SourceID != a.a.id && ca.Shared.SourceID != b.a.id {
		t.Fatal("invalid source")
	}
	a.a.mu.Lock()
	s := a.a.session
	if s == nil {
		message := a.a.message
		a.a.mu.Unlock()
		t.Fatalf("session ended immediately after pairing: %s", message)
	}
	s.conn.Close()
	a.a.mu.Unlock()
	waitFor(t, 8*time.Second, func() bool {
		x, ok := a.session()
		y, yes := b.session()
		return ok && yes && x.paired && y.paired && x.id != s.id
	})
}
func TestRejectAndStaleApprovalDoNotTrust(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	if a.post(t, "confirm", map[string]any{"session": "old-session", "match": true}) != 409 {
		t.Fatal("stale approval accepted")
	}
	s, _ := a.session()
	if a.post(t, "confirm", map[string]any{"session": s.id, "match": false}) != 200 {
		t.Fatal("reject failed")
	}
	waitFor(t, 3*time.Second, func() bool { _, x := a.session(); _, y := b.session(); return !x && !y })
	a.a.mu.Lock()
	p := a.a.config.Peer
	a.a.mu.Unlock()
	if p != nil {
		t.Fatal("rejected peer persisted")
	}
}
func TestRestartReusesIdentityAndAutoConnects(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	approve(t, b)
	paired(t, a, b)
	id := b.a.id
	port := b.a.peerPort
	b.stop(t)
	waitFor(t, 3*time.Second, func() bool { _, yes := a.session(); return !yes })
	replacement := startApp(t, b.dir, port)
	if replacement.a.id != id {
		t.Fatal("identity changed after restart")
	}
	waitFor(t, 8*time.Second, func() bool {
		x, ok := a.session()
		y, yes := replacement.session()
		return ok && yes && x.paired && y.paired
	})
}
func TestCancelAfterLocalApprovalDoesNotPersistTrust(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	waitFor(t, time.Second, func() bool { s, ok := a.session(); return ok && s.approved })
	s, _ := a.session()
	if a.post(t, "confirm", map[string]any{"session": s.id, "match": false}) != 200 {
		t.Fatal("cancellation rejected")
	}
	waitFor(t, 3*time.Second, func() bool { _, x := a.session(); _, y := b.session(); return !x && !y })
	for _, h := range []*harness{a, b} {
		c, err := readConfig(h.dir)
		if err != nil || c.Peer != nil {
			t.Fatal("cancelled pairing persisted trust", err)
		}
	}
}
func TestExpiredPairingCannotBeApproved(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	a.a.mu.Lock()
	s := a.a.session
	s.expires = time.Now().Add(-time.Second)
	id := s.id
	a.a.mu.Unlock()
	if a.post(t, "confirm", map[string]any{"session": id, "match": true}) != 409 {
		t.Fatal("expired pairing was approved")
	}
	a.a.mu.Lock()
	trusted := a.a.config.Peer != nil
	a.a.mu.Unlock()
	if trusted {
		t.Fatal("expired pairing persisted trust")
	}
}
func TestUnrelatedProfileIsUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.conf")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{DataDir: dir, DiscoveryPort: 59120}); err == nil {
		t.Fatal("unrelated nonempty profile accepted")
	}
	entries, err := os.ReadDir(dir)
	raw, readErr := os.ReadFile(path)
	if err != nil || readErr != nil || len(entries) != 1 || string(raw) != "keep" {
		t.Fatal("unrelated directory was changed")
	}
}
func TestDetectExistingInputWithoutStartingAnything(t *testing.T) {
	if os.Getenv("TWINGLIDE_EXPECT_EXISTING_INPUT") != "1" {
		t.Skip("opt-in read-only check on a machine already running Deskflow")
	}
	busy, err := foreignInputRunning()
	if err != nil || !busy {
		t.Fatal("existing input engine was not detected", err)
	}
}
func TestPinnedIdentityMismatchIsRejected(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	approve(t, b)
	paired(t, a, b)
	old := b.a.id
	b.stop(t)
	waitFor(t, 3*time.Second, func() bool { _, yes := a.session(); return !yes })
	stranger := startApp(t, "", 0)
	a.a.connect(context.Background(), Nearby{ID: old, Address: fmtAddress(stranger.a.peerPort)})
	a.a.mu.Lock()
	p := a.a.config.Peer
	a.a.mu.Unlock()
	if p == nil || p.ID != old {
		t.Fatal("pin replaced")
	}
	stranger.a.mu.Lock()
	p = stranger.a.config.Peer
	stranger.a.mu.Unlock()
	if p != nil {
		t.Fatal("stranger trusted")
	}
}
func TestSettingsSyncAndForget(t *testing.T) {
	a, b := startApp(t, "", 0), startApp(t, "", 0)
	beginPair(t, a, b)
	approve(t, a)
	approve(t, b)
	paired(t, a, b)
	if a.post(t, "settings", map[string]any{"sourceID": b.a.id, "side": "right", "enabled": false}) != 200 {
		t.Fatal("settings rejected")
	}
	waitFor(t, 3*time.Second, func() bool {
		b.a.mu.Lock()
		defer b.a.mu.Unlock()
		return b.a.config.Shared.Side == "right" && !b.a.config.Shared.Enabled && b.a.config.Shared.SourceID == b.a.id
	})
	if a.post(t, "forget", map[string]any{}) != 200 {
		t.Fatal("forget failed")
	}
	waitFor(t, 3*time.Second, func() bool { _, ok := a.session(); return !ok })
	c, e := readConfig(a.dir)
	if e != nil || c.Peer != nil {
		t.Fatal("forget did not persist", e)
	}
}
func TestLocalAPIRejectsMissingTokenOriginAndHost(t *testing.T) {
	a := startApp(t, "", 0)
	for _, tc := range []struct {
		token, host, origin string
		code                int
	}{
		{"", "", "", 401}, {a.ep.Token, "attacker.example", "", 403}, {a.ep.Token, "", "https://attacker.example", 403}, {a.ep.Token, "", "", 200},
	} {
		req, _ := http.NewRequest("GET", a.ep.URL+"/api/state", nil)
		req.Header.Set("X-TwinGlide-Token", tc.token)
		req.Header.Set("Origin", tc.origin)
		if tc.host != "" {
			req.Host = tc.host
		}
		res, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != tc.code {
			t.Errorf("got %d want %d", res.StatusCode, tc.code)
		}
		if bytes.Contains(raw, []byte(a.ep.Token)) {
			t.Fatal("local API disclosed its capability token")
		}
	}
}
func TestDamagedIdentityFailsClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.pem")
	if e := os.WriteFile(path, []byte("damaged"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readIdentity(dir); e == nil {
		t.Fatal("damaged identity accepted")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "damaged" {
		t.Fatal("damaged identity overwritten")
	}
}
func TestEngineConfigurationIsIsolatedAndMutuallyPinned(t *testing.T) {
	dir := t.TempDir()
	if _, e := readIdentity(dir); e != nil {
		t.Fatal(e)
	}
	one := strings.Repeat("a", 64)
	two := strings.Repeat("b", 64)
	core := filepath.Join(dir, "runtime", "engine", "deskflow-core.exe")
	shared := Shared{SourceID: one, Side: "left", Enabled: true, Revision: 1, Actor: one}
	path, mode, e := writeEngineConfig(dir, core, one, Peer{ID: two, Address: "192.0.2.10:59121"}, shared)
	if e != nil {
		t.Fatal(e)
	}
	if mode != "server" {
		t.Fatal("wrong engine role")
	}
	raw, _ := os.ReadFile(path)
	for _, s := range []string{"port=59122", "processMode=1", "tlsEnabled=true", "checkPeerFingerprints=true", "remoteHost=192.0.2.10"} {
		if !strings.Contains(string(raw), s) {
			t.Error("missing", s)
		}
	}
	pin, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "tls", "trusted-clients"))
	if string(pin) != "v2:sha256:"+two+"\n" {
		t.Fatal("incorrect pin")
	}
	if !strings.Contains(string(raw), "enableClipboard=false") {
		t.Fatal("clipboard regression safeguard absent")
	}
}
func TestAtomicWritePreservesLastGoodStateOnInvalidPeer(t *testing.T) {
	dir := t.TempDir()
	if e := writeJSON(filepath.Join(dir, "state.json"), Config{Version: 1, Name: "test", Peer: &Peer{ID: "bad"}}); e != nil {
		t.Fatal(e)
	}
	if _, e := readConfig(dir); e == nil {
		t.Fatal("invalid stored peer accepted")
	}
}
