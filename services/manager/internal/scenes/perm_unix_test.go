//go:build !windows

package scenes

import (
	"os"
	"testing"
)

func requireOwnerOnly(t *testing.T, info os.FileInfo) {
	t.Helper()
	if info.IsDir() {
		t.Fatal("expected a file")
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode %o", got)
	}
}
