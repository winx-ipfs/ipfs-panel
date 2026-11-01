//go:build linux

package node

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sameDaemon reports whether pid is the configured Kubo binary running
// `daemon` with IPFS_PATH equal to repo. This is how a manager restarted
// after SIGKILL finds the child it already started.
func sameDaemon(pid int, bin, repo string) bool {
	if pid <= 0 || bin == "" || repo == "" {
		return false
	}
	cmdRaw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	envRaw, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return false
	}
	argv := splitNUL(cmdRaw)
	if len(argv) > 0 && argv[len(argv)-1] == "" {
		argv = argv[:len(argv)-1]
	}
	if len(argv) != 2 || argv[1] != "daemon" {
		return false
	}
	if !sameBinary(argv[0], bin) {
		return false
	}
	return envValue(envRaw, "IPFS_PATH") == repo
}

func splitNUL(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	parts := strings.Split(string(b), "\x00")
	return parts
}

func envValue(raw []byte, key string) string {
	prefix := key + "="
	for _, entry := range strings.Split(string(raw), "\x00") {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func sameBinary(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	if filepath.Clean(got) == filepath.Clean(want) {
		return true
	}
	a, errA := filepath.EvalSymlinks(got)
	b, errB := filepath.EvalSymlinks(want)
	if errA != nil || errB != nil {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
