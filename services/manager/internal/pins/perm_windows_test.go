//go:build windows

package pins

import (
	"os"
	"testing"
)

// Windows has no owner-only mode bits, so the label file cannot be asserted
// to 0600. Assert the file exists and is not read-only, which is the only
// permission state os.Chmod controls there.
func requireOwnerOnly(t *testing.T, info os.FileInfo) {
	t.Helper()
	if info.IsDir() {
		t.Fatalf("expected a file, got a directory")
	}
	if info.Mode()&0o200 == 0 {
		t.Fatalf("label file is read-only")
	}
}
