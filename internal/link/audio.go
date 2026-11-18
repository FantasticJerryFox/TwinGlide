package link

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Audio uses a separate mutually pinned TLS connection on the existing control
// port. ALPN separates protocols; pairing permission alone never permits audio.
func (a *App) audioRouteLocked() (string, Peer, Shared) {
	s := normalizedShared(a.config.Shared)
	if a.stopping || (a.opt.NoInput && !a.opt.AudioTest) || a.session == nil || !a.session.paired || a.session.revoked ||
		a.config.Peer == nil || s.AudioTarget == "" || !validShared(s, a.id, a.config.Peer.ID) {
		return "", Peer{}, s
	}
	p := a.session.peer
	key := fmt.Sprintf("%s|%s|%d|%d", a.session.id, s.AudioTarget, s.AudioVolume, s.AudioBufferMS)
	return key, p, s
}
func (a *App) setAudioStatus(s string) { a.mu.Lock(); a.audioStatus = s; a.mu.Unlock() }

func (a *App) runAudio(ctx context.Context) {
	var cancel context.CancelFunc
	var done chan struct{}
	var current string
	retry := time.Time{}
	stop := func() {
		if cancel != nil {
			cancel()
			<-done
			cancel = nil
			done = nil
			current = ""
		}
	}
	defer stop()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		a.mu.Lock()
		key, peer, s := a.audioRouteLocked()
		dialer := a.session != nil && a.session.outgoing
		if key == "" {
			switch {
			case a.opt.NoInput && !a.opt.AudioTest:
				a.audioStatus = "隔离测试：不采集或播放系统声音"
			case s.AudioTarget == "":
				a.audioStatus = "各自播放，音频转发已关闭"
			default:
				a.audioStatus = "等待已配对电脑上线"
			}
		}
		a.mu.Unlock()
		if key != current {
			stop()
		}
		if cancel != nil {
			select {
			case <-done:
				cancel()
				cancel = nil
				done = nil
				current = ""
				retry = time.Now().Add(4 * time.Second)
			default:
			}
		}
		if key == "" || !dialer || cancel != nil || time.Now().Before(retry) {
			continue
		}
		job, jobCancel := context.WithCancel(ctx)
		cancel = jobCancel
		done = make(chan struct{})
		current = key
		finished := done
		go func() {
			defer jobCancel()
			defer close(finished)
			if err := a.dialAudio(job, key, peer); err != nil && job.Err() == nil {
				a.setAudioStatus("音频连接恢复中：" + err.Error())
			}
		}()
	}
}

func (a *App) dialAudio(ctx context.Context, key string, peer Peer) error {
	cfg := a.tlsConfig()
	cfg.NextProtos = []string{"twinglide-audio/2"}
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if err := verifyCertificate(cs); err != nil {
			return err
		}
		if certificateID(cs.PeerCertificates[0].Raw) != peer.ID {
			return errors.New("audio peer identity mismatch")
		}
		return nil
	}
	d := tls.Dialer{NetDialer: &net.Dialer{Timeout: 4 * time.Second}, Config: cfg}
	raw, err := d.DialContext(ctx, "tcp4", peer.Address)
	if err != nil {
		return err
	}
	c := raw.(*tls.Conn)
	defer c.Close()
	if c.ConnectionState().NegotiatedProtocol != "twinglide-audio/2" {
		return errors.New("audio protocol mismatch")
	}
	_ = c.SetReadDeadline(time.Now().Add(6 * time.Second))
	var ack [1]byte
	if _, err = io.ReadFull(c, ack[:]); err != nil {
		return err
	}
	if ack[0] != 0xA2 {
		return errors.New("audio route was not accepted")
	}
	a.mu.Lock()
	_, _, settings := a.audioRouteLocked()
	a.mu.Unlock()
	return a.audioProcess(ctx, c, key, settings.AudioTarget != a.id, false)
}

func (a *App) receiveAudio(ctx context.Context, c *tls.Conn, peerID string) {
	a.mu.Lock()
	key, p, s := a.audioRouteLocked()
	allowed := key != "" && p.ID == peerID && a.audioConn == nil && !a.session.outgoing
	a.mu.Unlock()
	if !allowed {
		return
	}
	if err := a.audioProcess(ctx, c, key, s.AudioTarget != a.id, true); err != nil && ctx.Err() == nil {
		a.setAudioStatus("音频接收恢复中：" + err.Error())
	}
}

// A child owns only stdio, never sockets or credentials. No test mode invokes
// microphone/system capture, output devices, or login registration.
func (a *App) audioProcess(parent context.Context, c *tls.Conn, key string, capture, incoming bool) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	a.mu.Lock()
	live, _, settings := a.audioRouteLocked()
	if live != key || a.audioConn != nil {
		a.mu.Unlock()
		return errors.New("audio route changed")
	}
	a.audioConn = c
	a.audioKey = key
	a.audioFrames = 0
	a.audioRole = "receive"
	if capture {
		a.audioRole = "send"
	}
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		if a.audioConn == c {
			a.audioConn = nil
			a.audioKey = ""
			a.audioRole = ""
		}
		a.mu.Unlock()
	}()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	name := "TwinGlideAudio"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	mode := "--play"
	if capture {
		mode = "--capture"
	}
	if a.opt.AudioTest {
		mode = "--verify"
		if capture {
			mode = "--fixture"
		}
	}
	cmd := exec.CommandContext(ctx, filepath.Join(filepath.Dir(exe), name), mode, strconv.Itoa(settings.AudioVolume), strconv.Itoa(settings.AudioBufferMS), strconv.Itoa(os.Getpid()))
	hideCommand(cmd)
	var diagnostic limitedDiagnostic
	cmd.Stderr = &diagnostic
	var reader io.ReadCloser
	var writer io.WriteCloser
	if capture {
		reader, err = cmd.StdoutPipe()
	} else {
		writer, err = cmd.StdinPipe()
	}
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("音频组件不可用：%w", err)
	}
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		t := time.NewTicker(150 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				c.Close()
				return
			case <-t.C:
			}
			a.mu.Lock()
			now, _, _ := a.audioRouteLocked()
			a.mu.Unlock()
			if now != key {
				cancel()
				c.Close()
				return
			}
		}
	}()
	defer func() {
		cancel()
		if reader != nil {
			reader.Close()
		}
		if writer != nil {
			writer.Close()
		}
		_ = cmd.Wait()
		<-watchDone
	}()
	if incoming {
		c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if _, err = c.Write([]byte{0xA2}); err != nil {
			return err
		}
	}
	if a.opt.AudioTest {
		a.setAudioStatus("合成音频测试：不采集、不播放")
	}
	var src io.Reader = c
	var dst io.Writer = writer
	if capture {
		src = reader
		dst = c
	}
	seenFormat := false
	for {
		if !capture {
			timeout := 4 * time.Second
			if !seenFormat {
				timeout = 90 * time.Second
			}
			c.SetReadDeadline(time.Now().Add(timeout))
		}
		frame, err := readAudioFrame(src, &seenFormat)
		if err != nil {
			detail := diagnostic.String()
			if detail != "" {
				return fmt.Errorf("%s", detail)
			}
			return err
		}
		if capture {
			c.SetWriteDeadline(time.Now().Add(3 * time.Second))
		}
		if err = writeAudioFrame(dst, frame); err != nil {
			return err
		}
		if frame[0] == 2 {
			a.mu.Lock()
			a.audioFrames += uint64((len(frame) - 1) / 8)
			if !a.opt.AudioTest {
				if capture {
					a.audioStatus = "正在发送系统声音"
				} else {
					a.audioStatus = "正在本机播放另一台电脑的声音"
				}
			}
			a.mu.Unlock()
		}
	}
}

func readAudioFrame(r io.Reader, seenFormat *bool) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint32(header[:])
	if n < 1 || n > 65537 {
		return nil, errors.New("audio frame exceeds size limit")
	}
	b := make([]byte, int(n))
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	if !*seenFormat {
		if len(b) != 7 || b[0] != 1 || b[5] != 2 || b[6] != 0 {
			return nil, errors.New("invalid stereo float audio format")
		}
		rate := binary.LittleEndian.Uint32(b[1:5])
		if rate < 8000 || rate > 192000 {
			return nil, errors.New("unsupported audio sample rate")
		}
		*seenFormat = true
		return b, nil
	}
	if b[0] == 3 && len(b) == 1 {
		return b, nil
	}
	if b[0] != 2 || len(b) < 9 || (len(b)-1)%8 != 0 {
		return nil, errors.New("invalid audio sample frame")
	}
	return b, nil
}
func writeAudioFrame(w io.Writer, b []byte) error {
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(b)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

type limitedDiagnostic struct {
	mu   sync.Mutex
	text string
}

func (d *limitedDiagnostic) Write(b []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.text += string(b)
	if len(d.text) > 1024 {
		d.text = d.text[len(d.text)-1024:]
	}
	return len(b), nil
}
func (d *limitedDiagnostic) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.TrimSpace(d.text)
}
