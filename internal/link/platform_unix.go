//go:build !windows

package link

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func hideCommand(c *exec.Cmd)           {}
func replaceFile(from, to string) error { return os.Rename(from, to) }
func privateDir(path string) error {
	if e := os.MkdirAll(path, 0700); e != nil {
		return e
	}
	return os.Chmod(path, 0700)
}
func lockData(path string) (func(), error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("TwinGlide is already running for this profile: %w", e)
	}
	return func() { f.Close() }, nil
}
