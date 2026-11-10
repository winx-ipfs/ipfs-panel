//go:build windows

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
	if info.Mode()&0o200 == 0 {
		t.Fatal("file is read-only")
	}
}
