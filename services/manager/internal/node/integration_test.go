package node

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestRealKuboOptional runs the lifecycle against a real Kubo binary.
// It is skipped unless IPFS_PANEL_INTEGRATION=1 and ipfs is on PATH.
func TestRealKuboOptional(t *testing.T) {
	if os.Getenv("IPFS_PANEL_INTEGRATION") != "1" {
		t.Skip("set IPFS_PANEL_INTEGRATION=1 to run against a real ipfs binary")
	}
	bin, err := exec.LookPath("ipfs")
	if err != nil {
		t.Skip("ipfs not on PATH")
	}
	if strings.Contains(bin, "ipfs-panel-fakeipfs") {
		t.Skip("PATH is the test double, not Kubo")
	}
	repo := t.TempDir()
	sup, err := New(Config{
		ManagerAddr: "127.0.0.1:8080",
		RepoPath:    repo,
		KuboBin:     bin,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	if _, err := sup.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	up := waitFor(t, sup, 45*time.Second, func(ov Overview) bool {
		return ov.State == StateUp || ov.State == StateFailed
	})
	if up.State != StateUp || up.PeerID == nil || *up.PeerID == "" || up.Version == nil {
		t.Fatalf("%#v", up)
	}
	if _, err := sup.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	down := waitFor(t, sup, 20*time.Second, func(ov Overview) bool { return ov.State == StateDown })
	if down.Error != nil {
		t.Fatalf("%#v", down)
	}
}
