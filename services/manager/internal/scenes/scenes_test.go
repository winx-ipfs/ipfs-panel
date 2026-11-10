package scenes

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func TestQuietPreservesIdentityAndAPI(t *testing.T) {
	repo := t.TempDir()
	data := t.TempDir()
	writeFresh(t, repo)
	rt := &fakeRuntime{state: node.StateDown}
	res, err := Apply(context.Background(), repo, data, "127.0.0.1:8080", Quiet, rt)
	if err != nil {
		t.Fatal(err)
	}
	if res.Restart != RestartLater || rt.restarts != 0 {
		t.Fatalf("restart %s calls %d", res.Restart, rt.restarts)
	}
	doc := readDoc(t, repo)
	ident := doc["Identity"].(map[string]any)
	if ident["PrivKey"] != "secret-key" {
		t.Fatalf("identity %v", ident)
	}
	addrs := doc["Addresses"].(map[string]any)
	if addrs["API"] != "/ip4/127.0.0.1/tcp/5001" {
		t.Fatalf("api %v", addrs["API"])
	}
	swarm, _ := json.Marshal(addrs["Swarm"])
	if string(swarm) != `["/ip4/0.0.0.0/tcp/4001"]` {
		t.Fatalf("swarm %s", swarm)
	}
	if doc["Custom"].(map[string]any)["Keep"] != true {
		t.Fatal("custom key dropped")
	}
	if doc["Routing"].(map[string]any)["Type"] != "autoclient" {
		t.Fatalf("routing %v", doc["Routing"])
	}
	if doc["AutoNAT"].(map[string]any)["ServiceMode"] != "disabled" {
		t.Fatal("autonat")
	}
	relay := doc["Swarm"].(map[string]any)["RelayService"].(map[string]any)
	if relay["Enabled"] != false {
		t.Fatal("relay")
	}
	conn := doc["Swarm"].(map[string]any)["ConnMgr"].(map[string]any)
	if conn["LowWater"] != json.Number("20") || conn["HighWater"] != json.Number("40") || conn["GracePeriod"] != "1m0s" {
		t.Fatalf("conn %#v", conn)
	}
	info, err := os.Stat(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerOnly(t, info)
	backups, err := os.ReadDir(filepath.Join(data, "scene-backups"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups %v %v", backups, err)
	}
	binfo, err := backups[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerOnly(t, binfo)
}

func TestScenesWriteTheirKeys(t *testing.T) {
	cases := []struct {
		id       string
		strategy string
		noFetch  bool
		public   bool
	}{
		{Share, "mfs", false, false},
		{Pinset, "pinned", true, false},
		{Gateway, "pinned+mfs", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			repo := t.TempDir()
			writeFresh(t, repo)
			if _, err := Apply(context.Background(), repo, t.TempDir(), "127.0.0.1:8080", tc.id, &fakeRuntime{state: node.StateDown}); err != nil {
				t.Fatal(err)
			}
			doc := readDoc(t, repo)
			if doc["Provide"].(map[string]any)["Strategy"] != tc.strategy {
				t.Fatalf("strategy %v", doc["Provide"])
			}
			gw := doc["Gateway"].(map[string]any)
			if gw["NoFetch"] != tc.noFetch {
				t.Fatalf("nofetch %v", gw["NoFetch"])
			}
			addrs := gatewayAddrs(t, doc)
			if tc.public {
				if !contains(addrs, "/ip4/0.0.0.0/tcp/8081") || !contains(addrs, "/ip6/::/tcp/8081") {
					t.Fatalf("public %v", addrs)
				}
				if gw["NoDNSLink"] != true || gw["ExposeRoutingAPI"] != false {
					t.Fatalf("flags %v", gw)
				}
			} else if contains(addrs, "/ip4/0.0.0.0/tcp/8081") {
				t.Fatalf("gateway became public %v", addrs)
			}
			body := readBytes(t, repo)
			if err := node.AssessConfig(body, "127.0.0.1:8080"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApplyUpRestartsAndRevertRestores(t *testing.T) {
	repo := t.TempDir()
	data := t.TempDir()
	writeFresh(t, repo)
	rt := &fakeRuntime{state: node.StateUp}
	res, err := Apply(context.Background(), repo, data, "127.0.0.1:8080", Quiet, rt)
	if err != nil {
		t.Fatal(err)
	}
	if res.Restart != RestartStarted || rt.restarts != 1 || res.Node.State != node.StateStopping {
		t.Fatalf("%#v restarts %d", res, rt.restarts)
	}
	rt.state = node.StateUp
	again, err := Apply(context.Background(), repo, data, "127.0.0.1:8080", Quiet, rt)
	if err != nil || again.Restart != RestartNone || rt.restarts != 1 {
		t.Fatalf("second %#v %v restarts %d", again, err, rt.restarts)
	}
	rt.state = node.StateDown
	rev, err := Revert(context.Background(), repo, data, "127.0.0.1:8080", rt)
	if err != nil {
		t.Fatal(err)
	}
	if rev.Restart != RestartLater || rt.restarts != 1 {
		t.Fatalf("revert %#v restarts %d", rev, rt.restarts)
	}
	doc := readDoc(t, repo)
	if routing, ok := doc["Routing"].(map[string]any); ok && routing["Type"] != nil {
		t.Fatalf("routing restored as %#v", routing)
	}
	if doc["Identity"].(map[string]any)["PrivKey"] != "secret-key" {
		t.Fatal("identity")
	}
	ok, err := hasBackup(data)
	if err != nil || ok {
		t.Fatalf("backup remains %v %v", ok, err)
	}
}

func TestPreviewHidesAddresses(t *testing.T) {
	repo := t.TempDir()
	writeFresh(t, repo)
	view, err := Preview(repo, t.TempDir(), "127.0.0.1:8080", node.StateUp, Gateway)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Restart || view.Active {
		t.Fatalf("%#v", view)
	}
	raw, _ := json.Marshal(view)
	for _, forbidden := range []string{"0.0.0.0", "/tcp/5001", "secret-key", "multiaddr"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("preview contains %s: %s", forbidden, raw)
		}
	}
	found := false
	for _, change := range view.Changes {
		if change.Key == "Addresses.Gateway" && change.Code == "public" {
			found = true
		}
	}
	if !found {
		t.Fatalf("changes %#v", view.Changes)
	}
}

func TestRefusesPublicAPIAndBusy(t *testing.T) {
	repo := t.TempDir()
	doc := fresh()
	doc["Addresses"].(map[string]any)["API"] = "/ip4/0.0.0.0/tcp/5001"
	writeDoc(t, repo, doc)
	before := readBytes(t, repo)
	_, err := Apply(context.Background(), repo, t.TempDir(), "127.0.0.1:8080", Quiet, &fakeRuntime{state: node.StateDown})
	if err == nil || err.Error() != node.CodeAPINotLoopback {
		t.Fatal(err)
	}
	if string(readBytes(t, repo)) != string(before) {
		t.Fatal("config changed")
	}
	writeFresh(t, repo)
	_, err = Apply(context.Background(), repo, t.TempDir(), "127.0.0.1:8080", Quiet, &fakeRuntime{state: node.StateStarting})
	if err == nil || err.Error() != "node_busy" {
		t.Fatal(err)
	}
	_, err = Revert(context.Background(), repo, t.TempDir(), "127.0.0.1:8080", &fakeRuntime{state: node.StateDown})
	if err == nil || err.Error() != "nothing_to_revert" {
		t.Fatal(err)
	}
}

func TestNotReady(t *testing.T) {
	_, err := Apply(context.Background(), t.TempDir(), t.TempDir(), "127.0.0.1:8080", Share, &fakeRuntime{state: node.StateDown})
	if err == nil || err.Error() != "not_ready" {
		t.Fatal(err)
	}
}

type fakeRuntime struct {
	state    string
	restarts int
}

func (f *fakeRuntime) Cached(context.Context) (node.Overview, error) {
	return node.Overview{State: f.state}, nil
}

func (f *fakeRuntime) Restart(context.Context) (node.Overview, error) {
	f.restarts++
	f.state = node.StateStopping
	return node.Overview{State: node.StateStopping}, nil
}

func fresh() map[string]any {
	return map[string]any{
		"Identity": map[string]any{"PeerID": "abc", "PrivKey": "secret-key"},
		"Addresses": map[string]any{
			"API":     "/ip4/127.0.0.1/tcp/5001",
			"Gateway": []any{"/ip4/127.0.0.1/tcp/8081"},
			"Swarm":   []any{"/ip4/0.0.0.0/tcp/4001"},
		},
		"Custom": map[string]any{"Keep": true},
	}
}

func writeFresh(t *testing.T, repo string) {
	t.Helper()
	writeDoc(t, repo, fresh())
}

func writeDoc(t *testing.T, repo string, doc map[string]any) {
	t.Helper()
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readDoc(t *testing.T, repo string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytesReader(t, repo))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func readBytes(t *testing.T, repo string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func bytesReader(t *testing.T, repo string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func gatewayAddrs(t *testing.T, doc map[string]any) []string {
	t.Helper()
	raw := doc["Addresses"].(map[string]any)["Gateway"]
	list, ok := raw.([]any)
	if !ok {
		t.Fatalf("gateway %T", raw)
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, item.(string))
	}
	return out
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
