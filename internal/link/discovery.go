package link

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type advertisement struct {
	Protocol string `json:"protocol"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	OS       string `json:"os"`
	Port     int    `json:"port"`
}

// Broadcasts are hints, never proof of identity. TLS pins authorize every connection.
func (a *App) discover(ctx context.Context) {
	sock, err := net.ListenUDP("udp4", &net.UDPAddr{Port: a.opt.DiscoveryPort})
	if err != nil {
		a.setMessage("局域网发现不可用，可以手动输入另一台电脑的地址")
		return
	}
	defer sock.Close()
	done := make(chan struct{})
	defer func() { sock.Close(); <-done }()
	go func() {
		defer close(done)
		buf := make([]byte, 2048)
		for {
			sock.SetReadDeadline(time.Now().Add(time.Second))
			n, addr, e := sock.ReadFromUDP(buf)
			if e != nil {
				if ctx.Err() != nil {
					return
				}
				if ne, ok := e.(net.Error); ok && ne.Timeout() {
					continue
				}
				return
			}
			var ad advertisement
			if n > 1400 || json.Unmarshal(buf[:n], &ad) != nil || ad.Protocol != "twinglide/2" || !validID(ad.ID) || ad.ID == a.id || ad.Port < 1 || ad.Port > 65535 || addr.IP.IsLoopback() {
				continue
			}
			a.mu.Lock()
			if len(a.nearby) < 64 || a.nearby[ad.ID].ID != "" {
				a.nearby[ad.ID] = Nearby{ID: ad.ID, Name: cleanName(ad.Name), OS: cleanName(ad.OS), Address: net.JoinHostPort(addr.IP.String(), fmt.Sprint(ad.Port)), Seen: time.Now()}
			}
			a.mu.Unlock()
		}
	}()
	announce := func() {
		a.mu.Lock()
		name := a.config.Name
		a.mu.Unlock()
		b, _ := json.Marshal(advertisement{Protocol: "twinglide/2", ID: a.id, Name: name, OS: localOS(), Port: a.peerPort})
		interfaces, _ := net.Interfaces()
		for _, i := range interfaces {
			if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
				continue
			}
			addresses, _ := i.Addrs()
			for _, addr := range addresses {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				ip := ipNet.IP.To4()
				if ip == nil || len(ipNet.Mask) != 4 {
					continue
				}
				broadcast := make(net.IP, 4)
				for j := 0; j < 4; j++ {
					broadcast[j] = ip[j] | ^ipNet.Mask[j]
				}
				out, e := net.ListenUDP("udp4", &net.UDPAddr{IP: ip})
				if e != nil {
					continue
				}
				out.SetWriteDeadline(time.Now().Add(300 * time.Millisecond))
				_, _ = out.WriteToUDP(b, &net.UDPAddr{IP: broadcast, Port: a.opt.DiscoveryPort})
				out.Close()
			}
		}
	}
	announce()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			announce()
		}
	}
}
