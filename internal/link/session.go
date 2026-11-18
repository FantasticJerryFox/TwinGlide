package link

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

type message struct {
	Type     string `json:"type"`
	Version  int    `json:"version,omitempty"`
	Name     string `json:"name,omitempty"`
	OS       string `json:"os,omitempty"`
	Trusted  bool   `json:"trusted,omitempty"`
	Port     int    `json:"port,omitempty"`
	Settings Shared `json:"settings,omitempty"`
}
type session struct {
	conn           *tls.Conn
	sendMu         sync.Mutex
	id             string
	peer           Peer
	code           string
	expires        time.Time
	approved       bool
	remoteApproved bool
	paired         bool
	revoked        bool
	outgoing       bool
	decision       chan bool
	updates        chan Shared
}

func (s *session) send(m message) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.conn.SetWriteDeadline(time.Now().Add(4 * time.Second))
	return json.NewEncoder(s.conn).Encode(m)
}
func verifyCertificate(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) != 1 {
		return errors.New("one device certificate required")
	}
	c := cs.PeerCertificates[0]
	if time.Now().Before(c.NotBefore) || time.Now().After(c.NotAfter) {
		return errors.New("device certificate expired or not valid yet")
	}
	return c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature)
}
func (a *App) tlsConfig() *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{a.cert}, MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAnyClientCert,
		// Local self-signed device certificates are authenticated by pinned SHA-256,
		// or (only during the explicit pairing window) user-verified TLS exporter SAS.
		InsecureSkipVerify: true, VerifyConnection: verifyCertificate, NextProtos: []string{"twinglide-control/2", "twinglide-audio/2"}}
}
func (a *App) accept(ctx context.Context, l net.Listener) {
	slots := make(chan struct{}, 4)
	for {
		c, err := l.Accept()
		if err != nil {
			return
		}
		select {
		case slots <- struct{}{}:
			a.sessions.Add(1)
			go func() {
				defer a.sessions.Done()
				defer func() { <-slots }()
				a.handle(ctx, tls.Server(c, a.tlsConfig()), "", false)
			}()
		default:
			c.Close()
		}
	}
}
func (a *App) connect(ctx context.Context, n Nearby) {
	a.mu.Lock()
	if a.dialing || a.session != nil || a.stopping {
		a.mu.Unlock()
		return
	}
	a.dialing = true
	a.sessions.Add(1)
	if a.ctx != nil {
		ctx = a.ctx
	}
	a.mu.Unlock()
	defer a.sessions.Done()
	defer func() { a.mu.Lock(); a.dialing = false; a.mu.Unlock() }()
	d := net.Dialer{Timeout: 4 * time.Second}
	raw, err := d.DialContext(ctx, "tcp4", n.Address)
	if err != nil {
		if ctx.Err() == nil {
			a.setMessage(connectionFailure(err, runtime.GOOS))
		}
		return
	}
	cfg := a.tlsConfig()
	cfg.NextProtos = []string{"twinglide-control/2"}
	a.handle(ctx, tls.Client(raw, cfg), n.ID, true)
}
func pairingCode(cs tls.ConnectionState) (string, error) {
	material, err := cs.ExportKeyingMaterial("EXPORTER-TwinGlide-Pair-v1", nil, 6)
	if err != nil {
		return "", err
	}
	code := strings.ToUpper(hex.EncodeToString(material))
	return code[:4] + " " + code[4:8] + " " + code[8:], nil
}
func (a *App) handle(ctx context.Context, c *tls.Conn, expected string, outgoing bool) {
	defer c.Close()
	c.SetDeadline(time.Now().Add(6 * time.Second))
	if err := c.HandshakeContext(ctx); err != nil {
		return
	}
	cs := c.ConnectionState()
	peerID := certificateID(cs.PeerCertificates[0].Raw)
	if cs.NegotiatedProtocol == "twinglide-audio/2" {
		a.receiveAudio(ctx, c, peerID)
		return
	}
	if cs.NegotiatedProtocol != "twinglide-control/2" {
		a.setMessage("请将两台电脑上的双游升级到相同版本")
		return
	}
	if peerID == a.id || expected != "" && expected != peerID {
		a.setMessage("设备身份不匹配，连接已拒绝")
		return
	}
	a.mu.Lock()
	trusted := a.config.Peer != nil && a.config.Peer.ID == peerID
	allowed := trusted || (a.config.Peer == nil && time.Now().Before(a.pairingUntil))
	if !allowed || a.session != nil || !trusted && time.Since(a.lastIncoming) < 3*time.Second {
		a.mu.Unlock()
		return
	}
	if !trusted {
		a.lastIncoming = time.Now()
	}
	s := &session{conn: c, outgoing: outgoing, id: peerID + "-" + fmt.Sprint(time.Now().UnixNano()), expires: time.Now().Add(120 * time.Second), decision: make(chan bool, 1), updates: make(chan Shared, 8)}
	a.session = s
	hello := message{Type: "hello", Version: 2, Name: a.config.Name, OS: localOS(), Trusted: trusted, Port: a.peerPort, Settings: a.config.Shared}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		if a.session == s {
			a.session = nil
		}
		a.mu.Unlock()
	}()
	if s.send(hello) != nil {
		return
	}
	scanner := bufio.NewScanner(c)
	scanner.Buffer(make([]byte, 4096), 16384)
	if !scanner.Scan() {
		return
	}
	var remote message
	if json.Unmarshal(scanner.Bytes(), &remote) != nil || remote.Type != "hello" || remote.Version != 2 || remote.Port < 1 || remote.Port > 65535 {
		return
	}
	host, _, err := net.SplitHostPort(c.RemoteAddr().String())
	if err != nil {
		return
	}
	p := Peer{ID: peerID, Name: cleanName(remote.Name), OS: cleanName(remote.OS), Address: net.JoinHostPort(host, fmt.Sprint(remote.Port))}
	code, err := pairingCode(cs)
	if err != nil {
		return
	}
	a.mu.Lock()
	s.peer = p
	s.code = code
	auto := trusted && remote.Trusted
	if !auto && !time.Now().Before(a.pairingUntil) {
		a.mu.Unlock()
		a.setMessage("对方需要重新配对，请点击“允许配对”")
		return
	}
	shared := a.defaultShared(p, s.outgoing)
	if trusted && validShared(a.config.Shared, a.id, p.ID) {
		shared = a.config.Shared
	}
	if auto && validShared(remote.Settings, a.id, p.ID) {
		shared = newer(shared, remote.Settings)
	}
	if auto {
		s.approved = true
		s.remoteApproved = true
	}
	a.mu.Unlock()
	// Each frame has a deadline. An unauthenticated handshake cannot hold an input session.
	incoming := make(chan message, 8)
	readDone := make(chan struct{})
	defer func() { c.Close(); <-readDone }()
	go func() {
		defer close(readDone)
		defer close(incoming)
		for {
			c.SetReadDeadline(time.Now().Add(12 * time.Second))
			if !scanner.Scan() {
				return
			}
			var m message
			if json.Unmarshal(scanner.Bytes(), &m) != nil {
				return
			}
			select {
			case incoming <- m:
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				return
			}
		}
	}()
	finish := func() error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if s.revoked || a.stopping || a.session != s || time.Now().After(s.expires) {
			return errors.New("pairing session no longer valid")
		}
		old := a.config
		a.config.Peer = &p
		a.config.Shared = shared
		if e := a.saveLocked(); e != nil {
			a.config = old
			a.message = "无法保存配对：" + e.Error()
			return e
		}
		s.paired = true
		a.message = ""
		a.pairingUntil = time.Time{}
		return nil
	}
	if auto {
		if finish() != nil {
			return
		}
		if s.send(message{Type: "ready", Settings: shared}) != nil {
			return
		}
	}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ok := <-s.decision:
			if !ok {
				_ = s.send(message{Type: "reject"})
				return
			}
			a.mu.Lock()
			s.approved = true
			both := s.remoteApproved
			a.mu.Unlock()
			if s.send(message{Type: "approve"}) != nil {
				return
			}
			if both {
				if finish() != nil {
					return
				}
				if s.send(message{Type: "ready", Settings: shared}) != nil {
					return
				}
			}
		case update := <-s.updates:
			if s.send(message{Type: "settings", Settings: update}) != nil {
				return
			}
		case m, ok := <-incoming:
			if !ok {
				return
			}
			switch m.Type {
			case "ping":
				a.mu.Lock()
				if s.paired && m.Name != "" && a.config.Peer != nil && a.config.Peer.Name != cleanName(m.Name) {
					old := *a.config.Peer
					next := old
					next.Name = cleanName(m.Name)
					a.config.Peer = &next
					if err := a.saveLocked(); err != nil {
						a.config.Peer = &old
					} else {
						s.peer.Name = next.Name
					}
				}
				a.mu.Unlock()
			case "reject":
				a.setMessage("对方取消了配对")
				return
			case "approve":
				a.mu.Lock()
				s.remoteApproved = true
				both := s.approved
				already := s.paired
				a.mu.Unlock()
				if both && !already {
					if finish() != nil {
						return
					}
					if s.send(message{Type: "ready", Settings: shared}) != nil {
						return
					}
				}
			case "ready", "settings":
				a.mu.Lock()
				if !s.paired || !validShared(m.Settings, a.id, p.ID) {
					a.message = "对方发送了无效的设置，连接已关闭"
					a.mu.Unlock()
					return
				}
				next := newer(a.config.Shared, m.Settings)
				old := a.config.Shared
				if next == old {
					a.mu.Unlock()
					continue
				}
				a.config.Shared = next
				if err := a.saveLocked(); err != nil {
					a.config.Shared = old
					a.message = "无法保存同步设置：" + err.Error()
					a.mu.Unlock()
					return
				}
				a.mu.Unlock()
			default:
				return
			}
		case <-ticker.C:
			a.mu.Lock()
			expired := !s.paired && time.Now().After(s.expires)
			a.mu.Unlock()
			if expired {
				a.setMessage("配对已超时，请重新发起")
				return
			}
			a.mu.Lock()
			name := a.config.Name
			a.mu.Unlock()
			if s.send(message{Type: "ping", Name: name}) != nil {
				return
			}
		}
	}
}
