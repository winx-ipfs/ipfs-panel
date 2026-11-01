package node

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/files"
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
	client, err := files.New(files.Options{
		BaseURL:         "http://127.0.0.1:5001",
		OpTimeout:       30 * time.Second,
		TransferTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	roundTripMFS(t, client)
	if _, err := sup.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	down := waitFor(t, sup, 20*time.Second, func(ov Overview) bool { return ov.State == StateDown })
	if down.Error != nil {
		t.Fatalf("%#v", down)
	}
}

func roundTripMFS(t *testing.T, client *files.Client) {
	t.Helper()
	ctx := context.Background()
	const root = "/panel-integration"
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = client.Remove(cctx, root)
	})
	if err := client.Mkdir(ctx, root); err != nil {
		t.Fatal(err)
	}
	entry, err := client.Write(ctx, root+"/hello.txt", strings.NewReader("hello"), false)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Size != 5 || entry.CID == "" || entry.Type != "file" {
		t.Fatalf("write %#v", entry)
	}
	list, err := client.List(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range list {
		if item.Name == "hello.txt" && item.Type == "file" && item.Size == 5 && item.CID != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("list %#v", list)
	}
	body, err := client.Open(ctx, root+"/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("read %q", got)
	}
	if err := client.Copy(ctx, root+"/hello.txt", root+"/copy.txt"); err != nil {
		t.Fatal(err)
	}
	if err := client.Move(ctx, root+"/copy.txt", root+"/moved.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Rename(ctx, root+"/moved.txt", "final.txt"); err != nil {
		t.Fatal(err)
	}
	if err := client.Remove(ctx, root+"/final.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Stat(ctx, root+"/final.txt"); files.CodeOf(err) != "not_found" {
		t.Fatalf("stat after remove: %v", err)
	}
}
