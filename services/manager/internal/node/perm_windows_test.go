//go:build windows

package node

import (
	"os"
	"testing"
)

// Windows has no owner-only mode bits, so the pid file cannot be asserted to
// 0600. Assert the file is there and writable, which is what the manager
// needs to rewrite it on the next start.
func requireOwnerOnly(t *testing.T, info os.FileInfo) {
	t.Helper()
	if info.IsDir() {
		t.Fatalf("expected the pid file, got a directory")
	}
	if info.Mode()&0o200 == 0 {
		t.Fatalf("pid file is read-only")
	}
}
