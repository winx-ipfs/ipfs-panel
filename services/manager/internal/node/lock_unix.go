//go:build unix

package node

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// repoLockHeld reports whether some process holds Kubo's repo.lock.
// The check takes the lock only long enough to see that it was free, then releases it.
func repoLockHeld(repo string) (bool, error) {
	if _, err := os.Stat(repo); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	f, err := os.Open(filepath.Join(repo, "repo.lock"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		return false, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return true, nil
	}
	return false, err
}
