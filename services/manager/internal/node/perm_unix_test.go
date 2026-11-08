//go:build !windows

package node

import (
	"os"
	"testing"
)

func requireOwnerOnly(t *testing.T, info os.FileInfo) {
	t.Helper()
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("pid mode %o", got)
	}
}
