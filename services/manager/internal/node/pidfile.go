package node

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func pidPath(repo string) string {
	return filepath.Join(repo, "ipfs-panel.pid")
}

func writePid(repo string, pid int) error {
	return os.WriteFile(pidPath(repo), []byte(strconv.Itoa(pid)+"\n"), 0o600)
}

func readPid(repo string) (int, bool) {
	b, err := os.ReadFile(pidPath(repo))
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func removePid(repo string) {
	err := os.Remove(pidPath(repo))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
}
