//go:build unix

package node

import (
	"os"
	"syscall"
)

func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func signalTree(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return os.ErrInvalid
	}
	if err := syscall.Kill(-pid, sig); err == nil {
		return nil
	}
	err := syscall.Kill(pid, sig)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

func terminate(pid int) error { return signalTree(pid, syscall.SIGTERM) }

func kill(pid int) error { return signalTree(pid, syscall.SIGKILL) }
