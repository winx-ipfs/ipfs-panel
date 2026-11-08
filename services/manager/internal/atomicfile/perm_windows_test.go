//go:build windows

package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

// Windows has no owner-only mode bits: os.Chmod only toggles the read-only
// file attribute. What this package must guarantee on Windows is that the
// rename replaced the old bytes and that no temporary file was left behind.
func TestWriteReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := Write(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "y" {
		t.Fatalf("body %q", body)
	}
	leftover, err := filepath.Glob(filepath.Join(dir, ".auth.json-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftover) != 0 {
		t.Fatalf("temp files left behind: %v", leftover)
	}
}
