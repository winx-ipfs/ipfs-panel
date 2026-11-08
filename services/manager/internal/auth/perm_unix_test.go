//go:build !windows

package auth

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

func requireOwnerOnlyDir(t *testing.T, info os.FileInfo) {
	t.Helper()
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir mode %o", got)
	}
}
