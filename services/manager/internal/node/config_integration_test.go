package node_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

// TestRealKuboConfig exercises config/show and config/replace against the
// released binary. It lives outside package node because configedit already
// imports node, and a node test cannot import it back.
func TestRealKuboConfig(t *testing.T) {
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
	deadline := time.Now().Add(2 * time.Minute)
	var up node.Overview
	for {
		up, err = sup.Cached(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if up.State == node.StateUp || up.State == node.StateFailed || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if up.State != node.StateUp {
		t.Fatalf("state %s", up.State)
	}
	apiBase, _, err := node.LoopbackEndpoints(repo)
	if err != nil {
		t.Fatal(err)
	}
	roundTripRealConfig(t, repo, apiBase)
	if _, err := sup.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	downBy := time.Now().Add(20 * time.Second)
	for {
		ov, err := sup.Cached(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if ov.State == node.StateDown || time.Now().After(downBy) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	client, err := configedit.NewClient(apiBase, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Show(context.Background()); configedit.CodeOf(err) != "node_not_running" {
		t.Fatalf("config show while stopped: %v", err)
	}
	t.Log("config show while stopped returned node_not_running")
}

func roundTripRealConfig(t *testing.T, repo, apiBase string) {
	t.Helper()
	client, err := configedit.NewClient(apiBase, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	svc := configedit.New(client, repo, t.TempDir(), "127.0.0.1:8080")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	view, err := svc.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secret := realPrivKey(t, repo)
	if secret == "" {
		t.Fatal("repo config has no private key")
	}
	if strings.Contains(view.Text, secret) || strings.Contains(view.Text, "PrivKey") {
		t.Fatal("config show returned the private key")
	}
	var shown map[string]any
	if err := json.Unmarshal([]byte(view.Text), &shown); err != nil {
		t.Fatal(err)
	}
	ident, _ := shown["Identity"].(map[string]any)
	if ident["PeerID"] == "" {
		t.Fatal("config show has no peer id")
	}
	enabled := realMDNS(shown)
	setRealMDNS(shown, !enabled)
	edited, err := json.Marshal(shown)
	if err != nil {
		t.Fatal(err)
	}
	replaced, err := svc.Replace(ctx, string(edited))
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if replaced.Restart != "needed" {
		t.Fatalf("restart %s", replaced.Restart)
	}
	var after map[string]any
	if err := json.Unmarshal([]byte(replaced.Text), &after); err != nil {
		t.Fatal(err)
	}
	if realMDNS(after) == enabled {
		t.Fatalf("Discovery.MDNS.Enabled stayed %v", enabled)
	}
	if strings.Contains(replaced.Text, secret) {
		t.Fatal("replaced config returned the private key")
	}
	if realPrivKey(t, repo) != secret {
		t.Fatal("config replace changed the private key on disk")
	}
	api := realAPI(after)
	if api != realAPI(shown) {
		t.Fatalf("API changed to %s", api)
	}
	locked := strings.Replace(string(edited), api, `"/ip4/127.0.0.1/tcp/5002"`, 1)
	if _, err := svc.Replace(ctx, locked); configedit.CodeOf(err) != "api_locked" {
		t.Fatalf("changed API: %v", err)
	}
	withKey := strings.Replace(strings.TrimRight(replaced.Text, "\n"), `"PeerID"`, `"PrivKey": "submitted-secret", "PeerID"`, 1)
	if _, err := svc.Replace(ctx, withKey); configedit.CodeOf(err) != "privkey_locked" {
		t.Fatalf("submitted private key: %v", err)
	}
	if realPrivKey(t, repo) != secret {
		t.Fatal("refused replace changed the private key")
	}
	restored, err := svc.Restore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal([]byte(restored.Text), &back); err != nil {
		t.Fatal(err)
	}
	if realMDNS(back) != enabled {
		t.Fatal("restore did not put Discovery.MDNS.Enabled back")
	}
	if restored.Backup {
		t.Fatal("restore left the backup in place")
	}
	t.Log("config show redacted the private key; replace and restore round-tripped Discovery.MDNS.Enabled")
}

func realPrivKey(t *testing.T, repo string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Identity struct {
			PrivKey string
		}
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Identity.PrivKey
}

func realMDNS(doc map[string]any) bool {
	discovery, _ := doc["Discovery"].(map[string]any)
	mdns, _ := discovery["MDNS"].(map[string]any)
	enabled, _ := mdns["Enabled"].(bool)
	return enabled
}

func setRealMDNS(doc map[string]any, enabled bool) {
	discovery, _ := doc["Discovery"].(map[string]any)
	if discovery == nil {
		discovery = map[string]any{}
		doc["Discovery"] = discovery
	}
	mdns, _ := discovery["MDNS"].(map[string]any)
	if mdns == nil {
		mdns = map[string]any{}
		discovery["MDNS"] = mdns
	}
	mdns["Enabled"] = enabled
}

func realAPI(doc map[string]any) string {
	addresses, _ := doc["Addresses"].(map[string]any)
	raw, err := json.Marshal(addresses["API"])
	if err != nil {
		return ""
	}
	return string(raw)
}
