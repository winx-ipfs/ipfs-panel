package node_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peering"
)

// A peer id the address rules accept, and not one of Kubo's default bootstrap
// peers, so the swarm does not already have a connection to it.
const realPeer = "12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d"

// TestRealKuboPeering keeps a peer through the live peering service and the
// saved Peering.Peers list. -run TestRealKubo starts it with the other
// released-binary tests.
func TestRealKuboPeering(t *testing.T) {
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
	release := os.Getenv("IPFS_PANEL_KUBO_VERSION")
	if release != "" && release != "0.43.1" {
		t.Fatalf("no settled peering expectations for Kubo %s", release)
	}
	repo := t.TempDir()
	sup, err := node.New(node.Config{
		ManagerAddr: "127.0.0.1:8080",
		RepoPath:    repo,
		KuboBin:     bin,
		ReadyBudget: 90 * time.Second,
		InitTimeout: 90 * time.Second,
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
	up := waitReal(t, sup, 2*time.Minute, node.StateUp)
	if up.State != node.StateUp {
		t.Fatalf("state %s", up.State)
	}
	apiBase, _, err := node.LoopbackEndpoints(repo)
	if err != nil {
		t.Fatal(err)
	}
	secret := realPrivKey(t, repo)
	svc := realPeering(t, repo, apiBase)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	view, err := svc.AddAddress(ctx, "/ip4/203.0.113.10/tcp/4001/p2p/"+realPeer)
	if err != nil {
		t.Fatal(err)
	}
	if view.Sync != peering.SyncOK || len(view.Peers) != 1 || view.Peers[0].PeerID != realPeer {
		t.Fatalf("add %#v", view)
	}
	if view.Peers[0].State != peering.StateReconnecting {
		t.Fatalf("state %s", view.Peers[0].State)
	}
	if view.Peers[0].Addrs[0] != "/ip4/203.0.113.10/tcp/4001" {
		t.Fatalf("addrs %#v", view.Peers[0].Addrs)
	}
	shown, err := configedit.NewClient(apiBase, 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := shown.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "PrivKey") {
		t.Fatal("config show returned the private key")
	}
	if !strings.Contains(string(raw), realPeer) {
		t.Fatal("config show is missing the saved peer")
	}
	if realPrivKey(t, repo) != secret {
		t.Fatal("peering changed the private key on disk")
	}
	if _, err := sup.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	again := waitReal(t, sup, 2*time.Minute, node.StateUp)
	if again.State != node.StateUp {
		t.Fatalf("after restart %s", again.State)
	}
	apiBase, _, err = node.LoopbackEndpoints(repo)
	if err != nil {
		t.Fatal(err)
	}
	svc = realPeering(t, repo, apiBase)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Minute)
	defer cancel2()
	view, err = svc.List(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	if view.Sync != peering.SyncOK || len(view.Peers) != 1 || view.Peers[0].PeerID != realPeer {
		t.Fatalf("after restart %#v", view)
	}
	view, err = svc.Remove(ctx2, realPeer)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Peers) != 0 || view.Sync != peering.SyncOK {
		t.Fatalf("remove %#v", view)
	}
	if _, err := sup.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	down := waitReal(t, sup, 20*time.Second, node.StateDown)
	if down.State != node.StateDown {
		t.Fatalf("stop %s", down.State)
	}
	if _, err := svc.List(context.Background()); peering.CodeOf(err) != "node_not_running" {
		t.Fatalf("stopped list: %v", err)
	}
	t.Log("peering add was live, survived restart, and a stopped node returned node_not_running")
}

func realPeering(t *testing.T, repo, apiBase string) *peering.Service {
	t.Helper()
	edit, err := configedit.NewClient(apiBase, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	live, err := peering.NewClient(apiBase)
	if err != nil {
		t.Fatal(err)
	}
	return peering.New(configedit.New(edit, repo, t.TempDir(), "127.0.0.1:8080"), live)
}

func waitReal(t *testing.T, sup *node.Supervisor, budget time.Duration, want string) node.Overview {
	t.Helper()
	deadline := time.Now().Add(budget)
	var ov node.Overview
	var err error
	for {
		ov, err = sup.Cached(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if ov.State == want || ov.State == node.StateFailed || time.Now().After(deadline) {
			return ov
		}
		time.Sleep(200 * time.Millisecond)
	}
}
