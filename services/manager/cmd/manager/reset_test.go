package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
)

func TestResetAdminRemovesCredentialFile(t *testing.T) {
	t.Setenv("IPFS_PANEL_REPO", "")
	dir := t.TempDir()
	path := auth.AuthPath(dir)
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := resetAdmin([]string{"-data", dir, "-repo", ""}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still there: %v", err)
	}
	if err := resetAdmin([]string{"-data", dir, "-repo", ""}); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	if err := resetAdmin([]string{"-data", repo, "-repo", repo}); err == nil {
		t.Fatal("reset inside the Kubo repo was accepted")
	}
	if _, err := os.Stat(filepath.Join(repo, "auth.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
