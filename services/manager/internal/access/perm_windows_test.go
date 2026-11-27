//go:build windows

package access

import (
	"os"
	"testing"
)

func requireOwnerOnly(t *testing.T, info os.FileInfo) {
	t.Helper()
	if info.IsDir() {
		t.Fatalf("expected the key file")
	}
	if info.Mode()&0o200 == 0 {
		t.Fatalf("key file is read-only")
	}
}
