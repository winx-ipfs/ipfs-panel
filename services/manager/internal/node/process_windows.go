//go:build windows

package node

import (
	"os"
	"syscall"
)

func daemonSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc == nil {
		return false
	}
	_ = proc.Release()
	return true
}

// terminate and kill both end the process. Windows has no POSIX process
// group: ipfs shutdown is the graceful step, and Kill is the follow-up.
func terminate(pid int) error { return killPID(pid) }

func kill(pid int) error { return killPID(pid) }

func killPID(pid int) error {
	if pid <= 0 {
		return os.ErrInvalid
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer func() { _ = proc.Release() }()
	return proc.Kill()
}
