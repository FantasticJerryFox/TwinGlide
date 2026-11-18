//go:build windows

package link

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"syscall"
	"time"
	"unsafe"
)

func hideCommand(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func replaceFile(from, to string) error {
	a, e := syscall.UTF16PtrFromString(from)
	if e != nil {
		return e
	}
	b, e := syscall.UTF16PtrFromString(to)
	if e != nil {
		return e
	}
	move := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		var ok uintptr
		ok, _, err = move.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 0x1|0x8)
		if ok != 0 {
			return nil
		}
		if err != syscall.ERROR_ACCESS_DENIED && err != syscall.Errno(32) {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
	return err
}
func privateDir(path string) error {
	if e := os.MkdirAll(path, 0700); e != nil {
		return e
	}
	u, e := user.Current()
	if e != nil {
		return e
	}
	c := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", "*"+u.Uid+":(OI)(CI)F", "*S-1-5-18:(OI)(CI)F")
	hideCommand(c)
	if out, e := c.CombinedOutput(); e != nil {
		return fmt.Errorf("protecting application data: %w (%s)", e, out)
	}
	return nil
}
func lockData(path string) (func(), error) {
	p, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	h, e := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, fmt.Errorf("TwinGlide is already running for this profile: %w", e)
	}
	return func() { syscall.CloseHandle(h) }, nil
}
