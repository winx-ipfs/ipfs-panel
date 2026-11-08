//go:build !windows

package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAppliesOwnerOnlyModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := Write(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode %o", got)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir mode %o", got)
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
