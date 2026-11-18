package link

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Endpoint struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

func (a *App) handler(host string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		if r.Host != host {
			http.Error(w, "invalid host", 403)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-TwinGlide-Token")), []byte(a.token)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+host {
			http.Error(w, "invalid origin", 403)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method == "GET" && r.URL.Path == "/api/state" {
			a.state(w)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var body struct {
			ID            string  `json:"id"`
			Address       string  `json:"address"`
			Session       string  `json:"session"`
			Match         bool    `json:"match"`
			SourceID      string  `json:"sourceID"`
			Side          string  `json:"side"`
			Enabled       bool    `json:"enabled"`
			EdgeDelayMS   *int    `json:"edgeDelayMS"`
			ReturnKey     *string `json:"returnKey"`
			AudioTarget   *string `json:"audioTarget"`
			AudioVolume   *int    `json:"audioVolume"`
			AudioBufferMS *int    `json:"audioBufferMS"`
			Name          string  `json:"name"`
			Theme         string  `json:"theme"`
			StartAtLogin  bool    `json:"startAtLogin"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if e := d.Decode(&body); e != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		var extra any
		if e := d.Decode(&extra); e != io.EOF {
			http.Error(w, "trailing request data", 400)
			return
		}
		fail := func(s string) { w.WriteHeader(409); _ = json.NewEncoder(w).Encode(map[string]string{"error": s}) }
		switch r.URL.Path {
		case "/api/nickname":
			name, e := randomDeviceName()
			if e != nil {
				fail(e.Error())
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"name": name})
			return
		case "/api/preferences":
			if e := a.updatePreferences(body.Name, Preferences{Theme: body.Theme, StartAtLogin: body.StartAtLogin}); e != nil {
				fail(e.Error())
				return
			}
		case "/api/pairing":
			a.mu.Lock()
			a.pairingUntil = time.Now().Add(5 * time.Minute)
			a.message = ""
			a.mu.Unlock()
		case "/api/connect":
			a.mu.Lock()
			n, ok := a.nearby[body.ID]
			busy := a.session != nil || a.dialing
			paired := a.config.Peer != nil && a.config.Peer.ID != body.ID
			if ok && !busy && !paired {
				a.pairingUntil = time.Now().Add(5 * time.Minute)
			}
			a.mu.Unlock()
			if !ok || busy || paired {
				fail("设备不在线、正在连接或已配对其他设备")
				return
			}
			go a.connect(context.Background(), n)
		case "/api/manual":
			address := strings.TrimSpace(body.Address)
			if !strings.Contains(address, ":") {
				address = net.JoinHostPort(address, strconv.Itoa(a.peerPort))
			}
			h, p, e := net.SplitHostPort(address)
			ip := net.ParseIP(h)
			port, pe := strconv.Atoi(p)
			if e != nil || pe != nil || port < 1 || port > 65535 || ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsMulticast() {
				fail("请输入有效的 IPv4 地址，可附带端口")
				return
			}
			a.mu.Lock()
			busy := a.session != nil || a.dialing
			expected := ""
			if a.config.Peer != nil {
				expected = a.config.Peer.ID
			}
			a.pairingUntil = time.Now().Add(5 * time.Minute)
			a.mu.Unlock()
			if busy {
				fail("已有连接或配对正在进行")
				return
			}
			go a.connect(context.Background(), Nearby{Address: address, ID: expected})
		case "/api/confirm":
			a.mu.Lock()
			s := a.session
			if s == nil || s.id != body.Session || s.paired || time.Now().After(s.expires) {
				a.mu.Unlock()
				fail("配对已过期，请重新连接")
				return
			}
			if !body.Match {
				s.revoked = true
				a.message = "已取消配对"
				a.mu.Unlock()
				s.conn.Close()
				_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
				return
			}
			select {
			case s.decision <- body.Match:
			default:
			}
			a.mu.Unlock()
		case "/api/settings":
			a.mu.Lock()
			if a.config.Peer == nil {
				a.mu.Unlock()
				fail("请先完成配对")
				return
			}
			next := normalizedShared(a.config.Shared)
			next.SourceID = body.SourceID
			next.Side = body.Side
			next.Enabled = body.Enabled
			next.Revision = time.Now().UnixMilli()
			next.Actor = a.id
			if body.EdgeDelayMS != nil {
				next.EdgeDelayMS = *body.EdgeDelayMS
			}
			if body.ReturnKey != nil {
				next.ReturnKey = *body.ReturnKey
			}
			if body.AudioTarget != nil {
				next.AudioTarget = *body.AudioTarget
			}
			if body.AudioVolume != nil {
				next.AudioVolume = *body.AudioVolume
			}
			if body.AudioBufferMS != nil {
				next.AudioBufferMS = *body.AudioBufferMS
			}
			if next.Revision <= a.config.Shared.Revision {
				next.Revision = a.config.Shared.Revision + 1
			}
			if !validShared(next, a.id, a.config.Peer.ID) {
				a.mu.Unlock()
				fail("屏幕或音频设置无效，请检查选择和数值范围")
				return
			}
			old := a.config.Shared
			a.config.Shared = next
			if e := a.saveLocked(); e != nil {
				a.config.Shared = old
				a.mu.Unlock()
				fail("无法保存设置")
				return
			}
			if a.session != nil && a.session.paired {
				select {
				case a.session.updates <- next:
				default:
					a.session.conn.Close()
				}
			}
			a.mu.Unlock()
		case "/api/forget":
			a.mu.Lock()
			old := a.config
			a.config.Peer = nil
			a.config.Shared = defaultSettings()
			if e := a.saveLocked(); e != nil {
				a.config = old
				a.mu.Unlock()
				fail("无法移除配对")
				return
			}
			if a.session != nil {
				a.session.revoked = true
				a.session.conn.Close()
			}
			a.pairingUntil = time.Now().Add(5 * time.Minute)
			a.message = ""
			a.mu.Unlock()
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
}
func (a *App) state(w http.ResponseWriter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	nearby := make([]Nearby, 0, len(a.nearby))
	for _, n := range a.nearby {
		nearby = append(nearby, n)
	}
	sort.Slice(nearby, func(i, j int) bool { return nearby[i].Name < nearby[j].Name })
	var pending any
	connected := false
	if s := a.session; s != nil {
		connected = s.paired
		if !s.paired && s.code != "" {
			pending = map[string]any{"session": s.id, "name": s.peer.Name, "code": s.code, "approved": s.approved, "expires": s.expires.Unix()}
		}
	}
	state := map[string]any{"version": Version, "id": a.id, "name": a.config.Name, "os": localOS(), "peer": a.config.Peer, "nearby": nearby, "pending": pending, "connected": connected, "dialing": a.dialing, "settings": a.config.Shared, "message": a.message, "engine": a.engineStatus, "pairingOpen": time.Now().Before(a.pairingUntil), "noInput": a.opt.NoInput}
	state["preferences"] = a.config.Preferences
	state["audio"] = a.audioStatus
	state["audioTest"] = a.opt.AudioTest
	state["audioFrames"] = a.audioFrames
	state["audioRole"] = a.audioRole
	_ = json.NewEncoder(w).Encode(state)
}
