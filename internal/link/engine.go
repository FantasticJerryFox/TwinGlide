package link

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type engine struct {
	app       *App
	cmd       *exec.Cmd
	guard     *exec.Cmd
	done      chan error
	guardDone chan error
	key       string
	retry     time.Time
	log       *os.File
}

func (e *engine) status(s string) { e.app.mu.Lock(); e.app.engineStatus = s; e.app.mu.Unlock() }
func (e *engine) run(ctx context.Context) {
	defer e.stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick()
		}
	}
}
func (e *engine) tick() {
	a := e.app
	a.mu.Lock()
	s := a.session
	shared := a.config.Shared
	active := s != nil && s.paired && a.config.Peer != nil && shared.Enabled
	var peer Peer
	if active {
		peer = s.peer
	}
	a.mu.Unlock()
	if a.opt.NoInput {
		e.status("隔离测试模式：不会启动键鼠引擎")
		return
	}
	if !active {
		e.stop()
		if !shared.Enabled {
			e.status("键鼠共享已暂停")
		} else {
			e.status("等待配对设备上线")
		}
		return
	}
	key := fmt.Sprintf("%s|%s|%s|%d|%s", shared.SourceID, shared.Side, peer.Address, shared.EdgeDelayMS, shared.ReturnKey)
	if e.cmd != nil {
		select {
		case <-e.done:
			lastStatus := e.readStatus()
			if e.guard != nil {
				stopOwned(e.guard, e.guardDone)
			}
			e.cleanup()
			e.retry = time.Now().Add(4 * time.Second)
			if strings.Contains(lastStatus, "辅助功能") {
				e.status(lastStatus)
			} else {
				e.status("键鼠引擎已退出，请检查辅助功能授权和日志")
			}
			return
		default:
		}
		if key == e.key {
			e.status(e.readStatus())
			return
		}
		e.stop()
	}
	if time.Now().Before(e.retry) {
		return
	}
	busy, err := foreignInputRunning()
	if err != nil {
		e.status("无法核实其他键鼠程序的状态，已暂停接管")
		return
	}
	if busy {
		e.status("检测到其他键鼠共享程序正在运行。请自行退出原程序后再启用双游。")
		return
	}
	core, err := e.prepare()
	if err != nil {
		e.status("键鼠组件不可用：" + err.Error())
		e.retry = time.Now().Add(10 * time.Second)
		return
	}
	settings, mode, err := writeEngineConfig(a.opt.DataDir, core, a.id, peer, shared)
	if err != nil {
		e.status("无法写入键鼠设置：" + err.Error())
		return
	}
	logPath := filepath.Join(a.opt.DataDir, "logs", "input-launch.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0700)
	e.log, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		e.status(err.Error())
		return
	}
	e.cmd = exec.Command(core, mode, "--settings", settings)
	e.cmd.Dir = filepath.Dir(core)
	e.cmd.Env = engineEnv(a.opt.DataDir)
	hideCommand(e.cmd)
	e.cmd.Stdout = e.log
	e.cmd.Stderr = e.log
	if err = e.cmd.Start(); err != nil {
		e.cleanup()
		e.status("无法启动键鼠：" + err.Error())
		return
	}
	e.key = key
	e.done = make(chan error, 1)
	cmd := e.cmd
	done := e.done
	go func() { done <- cmd.Wait() }()
	if runtime.GOOS == "darwin" && mode == "server" {
		exe, _ := os.Executable()
		guardPath := filepath.Join(filepath.Dir(exe), "InputSourceGuard")
		if _, err := os.Stat(guardPath); err == nil {
			guard := exec.Command(guardPath)
			guard.Env = append(os.Environ(), "TWINGLIDE_DATA_DIR="+a.opt.DataDir, fmt.Sprintf("TWINGLIDE_INPUT_PID=%d", cmd.Process.Pid), "TWINGLIDE_INPUT_PATH="+core, fmt.Sprintf("TWINGLIDE_AGENT_PID=%d", os.Getpid()))
			guard.Stdout = e.log
			guard.Stderr = e.log
			if guard.Start() == nil {
				e.guard = guard
				e.guardDone = make(chan error, 1)
				gd := e.guardDone
				go func() { gd <- guard.Wait() }()
			}
		}
	}
	e.status("键鼠引擎已启动，正在连接")
}
func (e *engine) readStatus() string {
	p := filepath.Join(e.app.opt.DataDir, "logs", "keyboard.log")
	f, err := os.Open(p)
	if err != nil {
		return "键鼠引擎已运行，等待连接"
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "键鼠引擎已运行"
	}
	if info.Size() > 12000 {
		_, _ = f.Seek(-12000, io.SeekEnd)
	}
	raw, _ := io.ReadAll(io.LimitReader(f, 12000))
	lines := strings.Split(string(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		lower := strings.ToLower(line)
		if strings.Contains(lower, "not trusted") || strings.Contains(lower, "failed to create quartz event tap") {
			return "Mac 键鼠控制需要辅助功能授权。请从双游菜单打开“辅助功能授权”，允许 TwinGlide；若列出 TwinGlide Input，也请允许。随后会自动重试。"
		}
		if strings.Contains(lower, "no route to host") || strings.Contains(lower, "network is unreachable") || strings.Contains(lower, "permission denied") || strings.Contains(lower, "operation not permitted") {
			return connectionFailure(errors.New(line), runtime.GOOS)
		}
		if strings.Contains(line, "disconnected") || strings.Contains(line, "failed") || strings.Contains(line, "server is dead") {
			return "键鼠连接恢复中；可查看本机 logs/keyboard.log"
		}
		if strings.Contains(line, "connected to server") || strings.Contains(line, "has connected") || strings.Contains(line, "accepted client") || strings.Contains(line, "switch from") || strings.Contains(line, "clipboard sharing disabled by server") || strings.Contains(line, "entering computer") {
			return "键鼠已连接"
		}
	}
	return "键鼠引擎已运行，等待另一台电脑"
}
func (e *engine) stop() {
	// Terminate only process handles created by this instance. Never process names.
	if e.cmd != nil {
		stopOwned(e.cmd, e.done)
	}
	if e.guard != nil {
		stopOwned(e.guard, e.guardDone)
	}
	e.cleanup()
}
func (e *engine) cleanup() {
	if e.log != nil {
		e.log.Close()
	}
	e.log = nil
	e.cmd = nil
	e.done = nil
	e.key = ""
	e.guard = nil
	e.guardDone = nil
}
func stopOwned(cmd *exec.Cmd, done chan error) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
	} else {
		_ = cmd.Process.Signal(os.Interrupt)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
}
func foreignInputRunning() (bool, error) {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("tasklist.exe", "/FI", "IMAGENAME eq deskflow-core.exe", "/FO", "CSV", "/NH")
		hideCommand(c)
	} else {
		c = exec.Command("pgrep", "-x", "deskflow-core")
	}
	raw, err := c.Output()
	if runtime.GOOS != "windows" {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return false, nil
		}
		return len(strings.TrimSpace(string(raw))) > 0, err
	}
	return strings.Contains(strings.ToLower(string(raw)), "deskflow-core.exe"), err
}
func (e *engine) prepare() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	src := e.app.opt.EnginePath
	if src == "" {
		if runtime.GOOS == "windows" {
			src = filepath.Join(filepath.Dir(exe), "engine", "deskflow-core.exe")
		} else {
			src = filepath.Join(filepath.Dir(exe), "..", "Helpers", "InputEngine.app", "Contents", "MacOS", "deskflow-core")
		}
	}
	src, err = filepath.Abs(src)
	if err != nil {
		return "", err
	}
	if _, err = os.Stat(src); err != nil {
		return "", errors.New("请使用包含键鼠引擎的完整安装包")
	}
	if runtime.GOOS != "windows" {
		return src, nil
	}
	// Deskflow's Windows TLS database is isolated only in portable mode.
	// Its portable settings must live beside the executable, so stage our own bundle
	// into this user's private data directory, never into another installation.
	target := filepath.Join(e.app.opt.DataDir, "runtime", "engine")
	manifestPath := filepath.Join(filepath.Dir(src), "twinglide-engine.json")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", errors.New("引擎缺少发行校验清单")
	}
	hash := sha256.Sum256(manifest)
	stamp := hex.EncodeToString(hash[:])
	current, _ := os.ReadFile(filepath.Join(target, ".bundle"))
	if string(current) != stamp {
		if err = verifyBundle(filepath.Dir(src), manifest); err != nil {
			return "", err
		}
		if err = copyTree(filepath.Dir(src), target); err != nil {
			return "", err
		}
		if err = atomicWrite(filepath.Join(target, ".bundle"), []byte(stamp)); err != nil {
			return "", err
		}
	}
	return filepath.Join(target, "deskflow-core.exe"), nil
}
func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("unexpected symlink in Windows engine")
		}
		dest := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return atomicWrite(dest, b)
	})
}
func engineEnv(root string) []string {
	env := os.Environ()
	values := map[string]string{"XDG_CONFIG_HOME": filepath.Join(root, "input", "profile"), "XDG_STATE_HOME": filepath.Join(root, "input", "state")}
	if runtime.GOOS == "windows" {
		values["LOCALAPPDATA"] = filepath.Join(root, "input", "profile")
		values["APPDATA"] = values["LOCALAPPDATA"]
		values["PROGRAMDATA"] = values["LOCALAPPDATA"]
	}
	for k, v := range values {
		for i := len(env) - 1; i >= 0; i-- {
			if strings.EqualFold(strings.SplitN(env[i], "=", 2)[0], k) {
				env = append(env[:i], env[i+1:]...)
			}
		}
		env = append(env, k+"="+v)
	}
	return env
}
func writeEngineConfig(root, core, localID string, peer Peer, s Shared) (string, string, error) {
	if !validID(localID) || !validID(peer.ID) || !validShared(s, localID, peer.ID) {
		return "", "", errors.New("invalid engine identity or layout")
	}
	host, _, err := net.SplitHostPort(peer.Address)
	if err != nil || net.ParseIP(host) == nil {
		return "", "", errors.New("invalid peer address")
	}
	s = normalizedShared(s)
	input := filepath.Join(root, "input")
	settingsDir := input
	if runtime.GOOS == "windows" {
		settingsDir = filepath.Join(filepath.Dir(core), "settings")
	}
	tlsDir := filepath.Join(settingsDir, "tls")
	if err = os.MkdirAll(tlsDir, 0700); err != nil {
		return "", "", err
	}
	identity, err := os.ReadFile(filepath.Join(root, "identity.pem"))
	if err != nil {
		return "", "", err
	}
	if err = atomicWrite(filepath.Join(tlsDir, "deskflow.pem"), identity); err != nil {
		return "", "", err
	}
	fingerprint := []byte("v2:sha256:" + peer.ID + "\n")
	for _, name := range []string{"trusted-servers", "trusted-clients"} {
		if err = atomicWrite(filepath.Join(tlsDir, name), fingerprint); err != nil {
			return "", "", err
		}
	}
	localName := "tg-" + localID[:16]
	remoteName := "tg-" + peer.ID[:16]
	mode := "client"
	modeValue := 1
	if s.SourceID == localID {
		mode = "server"
		modeValue = 2
	}
	server, client := localName, remoteName
	if mode == "client" {
		server, client = remoteName, localName
	}
	opposite := map[string]string{"left": "right", "right": "left", "up": "down", "down": "up"}[s.Side]
	// Deskflow 1.27 registers computers in the main settings, not section: screens.
	layout := fmt.Sprintf("section: links\n    %s:\n        %s = %s\n    %s:\n        %s = %s\nend\nsection: options\n    keystroke(Control+Alt+Shift+%s) = switchToScreen(%s)\nend\n", server, s.Side, client, client, opposite, server, s.ReturnKey, server)
	if err = atomicWrite(filepath.Join(input, "layout.conf"), []byte(layout)); err != nil {
		return "", "", err
	}
	slash := func(s string) string { return filepath.ToSlash(s) }
	conf := fmt.Sprintf("[core]\ncomputerName=%s\ncoreMode=%d\nprocessMode=1\nport=59122\ninterface=0.0.0.0\nlanguage=en\n[client]\nremoteHost=%s\nlanguageSync=false\n[server]\nenableClipboard=false\nenableSwitchDelay=%t\nswitchDelay=%d\nexternalConfig=true\nexternalConfigFile=%s\n[security]\ntlsEnabled=true\ncheckPeerFingerprints=true\ncertificate=%s\n[log]\ntoFile=true\nfile=%s\nlevel=4\n", localName, modeValue, host, s.EdgeDelayMS > 0, s.EdgeDelayMS, slash(filepath.Join(input, "layout.conf")), slash(filepath.Join(tlsDir, "deskflow.pem")), slash(filepath.Join(root, "logs", "keyboard.log")))
	conf += fmt.Sprintf("[computer_%s]\nname=%s\n[computer_%s]\nname=%s\n", server, server, client, client)
	path := filepath.Join(settingsDir, "Deskflow.conf")
	if err = atomicWrite(path, []byte(conf)); err != nil {
		return "", "", err
	}
	// The macOS constructor loads XDG settings before --settings is processed.
	initial := filepath.Join(input, "profile", "Deskflow", "Deskflow.conf")
	if err = atomicWrite(initial, []byte(conf)); err != nil {
		return "", "", err
	}
	_ = os.MkdirAll(filepath.Join(root, "logs"), 0700)
	// Remove stale connection status, never another profile's log.
	if err = atomicWrite(filepath.Join(root, "logs", "keyboard.log"), nil); err != nil {
		return "", "", err
	}
	return path, mode, nil
}
