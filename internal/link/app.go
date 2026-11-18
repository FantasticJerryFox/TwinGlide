package link

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const Version = "0.1.0-alpha.5"

type Options struct {
	DataDir                         string
	PeerPort, DiscoveryPort, UIPort int
	NoInput, NoDiscovery            bool
	AudioTest                       bool
	EnginePath                      string
}
type Nearby struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	OS      string    `json:"os"`
	Address string    `json:"address"`
	Seen    time.Time `json:"-"`
}
type App struct {
	mu           sync.Mutex
	opt          Options
	config       Config
	cert         tls.Certificate
	id           string
	token        string
	nearby       map[string]Nearby
	session      *session
	dialing      bool
	pairingUntil time.Time
	lastIncoming time.Time
	message      string
	engineStatus string
	audioStatus  string
	audioConn    *tls.Conn
	audioKey     string
	audioFrames  uint64
	audioRole    string
	peerPort     int
	input        *engine
	cancel       context.CancelFunc
	ready        chan struct{}
	ctx          context.Context
	sessions     sync.WaitGroup
	stopping     bool
}

func New(opt Options) (*App, error) {
	if opt.AudioTest {
		opt.NoInput = true
	}
	abs, err := filepath.Abs(opt.DataDir)
	if err != nil {
		return nil, err
	}
	opt.DataDir = abs
	entries, readErr := os.ReadDir(abs)
	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, readErr
	}
	if len(entries) > 0 {
		if _, err := os.Stat(filepath.Join(abs, ".twinglide-profile")); err != nil {
			return nil, errors.New("refusing to use a nonempty directory that is not a TwinGlide profile")
		}
	}
	if opt.PeerPort < 0 || opt.PeerPort > 65535 || opt.DiscoveryPort < 1 || opt.DiscoveryPort > 65535 {
		return nil, errors.New("invalid network port")
	}
	if err = privateDir(abs); err != nil {
		return nil, err
	}
	if err = atomicWrite(filepath.Join(abs, ".twinglide-profile"), []byte("TwinGlide profile v1\n")); err != nil {
		return nil, err
	}
	return &App{opt: opt, nearby: map[string]Nearby{}, ready: make(chan struct{})}, nil
}
func (a *App) Run(parent context.Context) error {
	unlock, err := lockData(filepath.Join(a.opt.DataDir, "instance.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	if a.cert, err = readIdentity(a.opt.DataDir); err != nil {
		return err
	}
	a.id = certificateID(a.cert.Certificate[0])
	if a.config, err = readConfig(a.opt.DataDir); err != nil {
		return err
	}
	if a.config.Peer == nil {
		a.pairingUntil = time.Now().Add(5 * time.Minute)
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return err
	}
	a.token = hex.EncodeToString(token)
	ctx, cancel := context.WithCancel(parent)
	a.cancel = cancel
	a.ctx = ctx
	defer cancel()
	listener, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", a.opt.PeerPort))
	if err != nil {
		return fmt.Errorf("control port: %w", err)
	}
	defer listener.Close()
	a.peerPort = listener.Addr().(*net.TCPAddr).Port
	local, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", a.opt.UIPort))
	if err != nil {
		return err
	}
	server := &http.Server{Handler: a.handler(local.Addr().String()), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	defer server.Close()
	endpoint := Endpoint{URL: "http://" + local.Addr().String(), Token: a.token, PID: os.Getpid()}
	if err = writeJSON(filepath.Join(a.opt.DataDir, "endpoint.json"), endpoint); err != nil {
		return err
	}
	defer os.Remove(filepath.Join(a.opt.DataDir, "endpoint.json"))
	a.input = &engine{app: a}
	close(a.ready)
	var wg sync.WaitGroup
	launch := func(fn func()) { wg.Add(1); go func() { defer wg.Done(); fn() }() }
	launch(func() { _ = server.Serve(local) })
	launch(func() { a.accept(ctx, listener) })
	launch(func() { a.maintain(ctx) })
	launch(func() { a.input.run(ctx) })
	launch(func() { a.runAudio(ctx) })
	if !a.opt.NoDiscovery {
		launch(func() { a.discover(ctx) })
	}
	<-ctx.Done()
	listener.Close()
	server.Close()
	a.mu.Lock()
	a.stopping = true
	if a.session != nil {
		a.session.conn.Close()
	}
	a.mu.Unlock()
	wg.Wait()
	a.sessions.Wait()
	return nil
}
func (a *App) saveLocked() error {
	return writeJSON(filepath.Join(a.opt.DataDir, "state.json"), a.config)
}
func (a *App) setMessage(s string) { a.mu.Lock(); a.message = s; a.mu.Unlock() }
func (a *App) maintain(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.Lock()
			for id, p := range a.nearby {
				if time.Since(p.Seen) > 15*time.Second {
					delete(a.nearby, id)
				}
			}
			var target *Nearby
			if a.session == nil && !a.dialing && a.config.Peer != nil {
				p := a.config.Peer
				// Exactly one side dials on a known pair; incoming is accepted on either side.
				if a.id < p.ID {
					n := Nearby{ID: p.ID, Name: p.Name, OS: p.OS, Address: p.Address}
					if live, ok := a.nearby[p.ID]; ok {
						n = live
					}
					target = &n
				}
			}
			a.mu.Unlock()
			if target != nil {
				go a.connect(ctx, *target)
			}
		}
	}
}
func localOS() string {
	if runtime.GOOS == "darwin" {
		return "macOS"
	}
	if runtime.GOOS == "windows" {
		return "Windows"
	}
	return runtime.GOOS
}
func (a *App) defaultShared(peer Peer, outgoing bool) Shared {
	source := a.id
	if peer.OS == "macOS" && localOS() != "macOS" || peer.OS == localOS() && !outgoing {
		source = peer.ID
	}
	actor := a.id
	if peer.ID < actor {
		actor = peer.ID
	}
	s := defaultSettings()
	s.SourceID = source
	s.Revision = 1
	s.Actor = actor
	return s
}
func validShared(s Shared, local, peer string) bool {
	return (s.SourceID == local || s.SourceID == peer) && (s.Side == "left" || s.Side == "right" || s.Side == "up" || s.Side == "down") && s.Revision > 0 && (s.Actor == local || s.Actor == peer) &&
		s.EdgeDelayMS >= 0 && s.EdgeDelayMS <= 1000 && (s.ReturnKey == "" || s.ReturnKey == "F10" || s.ReturnKey == "F11" || s.ReturnKey == "F12") &&
		(s.AudioTarget == "" || s.AudioTarget == local || s.AudioTarget == peer) && s.AudioVolume >= 0 && s.AudioVolume <= 100 &&
		(s.AudioBufferMS == 0 || s.AudioBufferMS == 40 || s.AudioBufferMS == 80 || s.AudioBufferMS == 160)
}
