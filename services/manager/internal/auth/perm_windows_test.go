//go:build windows

package auth

import (
	"os"
	"testing"
)

// Windows has no owner-only mode bits: os.Chmod only toggles the read-only
// attribute. The credential file is still protected by the user profile's
// ACL, which is where Windows puts access control.
func requireOwnerOnly(t *testing.T, info os.FileInfo) {
	t.Helper()
	if info.IsDir() {
		t.Fatalf("expected the credential file, got a directory")
	}
	if info.Mode()&0o200 == 0 {
		t.Fatalf("credential file is read-only, not writable by the manager")
	}
}

func requireOwnerOnlyDir(t *testing.T, info os.FileInfo) {
	t.Helper()
	if !info.IsDir() {
		t.Fatalf("expected the data directory")
	}
}
