//go:build !windows

package pins

import (
	"os"
	"testing"
)

func requireOwnerOnly(t *testing.T, info os.FileInfo) {
	t.Helper()
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode %o", got)
	}
}
