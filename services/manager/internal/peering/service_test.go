package peering

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
)

const secretKey = "SECRET-PRIVKEY-VALUE"

type memKubo struct {
	doc      []byte
	replaced []byte
}

func (m *memKubo) Show(context.Context) ([]byte, error) { return append([]byte(nil), m.doc...), nil }

func (m *memKubo) Replace(_ context.Context, doc []byte) error {
	m.replaced = append([]byte(nil), doc...)
	m.doc = append([]byte(nil), doc...)
	return nil
}

type fakeRT struct {
	live      []Saved
	connected map[string]struct{}
	byID      map[string][]string
	failAdd   bool
}

func (f *fakeRT) List(context.Context) ([]Saved, error) {
	return append([]Saved(nil), f.live...), nil
}

func (f *fakeRT) Add(_ context.Context, addrs []string) error {
	if f.failAdd {
		return code(502, "kubo_error")
	}
	if len(addrs) == 0 {
		return code(400, "invalid_address")
	}
	_, id, err := splitAddress(addrs[0])
	if err != nil {
		return err
	}
	stored := make([]string, 0, len(addrs))
	for _, full := range addrs {
		bare, _, err := splitAddress(full)
		if err != nil {
			return err
		}
		stored = append(stored, bare)
	}
	f.live = append(filterSaved(f.live, id), Saved{PeerID: id, Addrs: stored})
	return nil
}

func (f *fakeRT) Remove(_ context.Context, id string) error {
	f.live = filterSaved(f.live, id)
	return nil
}

func (f *fakeRT) Swarm(context.Context) (map[string]struct{}, map[string][]string, error) {
	if f.connected == nil {
		f.connected = map[string]struct{}{}
	}
	if f.byID == nil {
		f.byID = map[string][]string{}
	}
	return f.connected, f.byID, nil
}

func filterSaved(list []Saved, id string) []Saved {
	out := make([]Saved, 0, len(list))
	for _, item := range list {
		if item.PeerID != id {
			out = append(out, item)
		}
	}
	return out
}

func TestServiceAddWritesConfigWithoutThePrivateKey(t *testing.T) {
	data := t.TempDir()
	mem := &memKubo{doc: []byte(`{
	  "Identity": {"PeerID": "12D3KooWexample", "PrivKey": "` + secretKey + `"},
	  "Addresses": {"API": "/ip4/127.0.0.1/tcp/5001", "Gateway": ["/ip4/127.0.0.1/tcp/8081"]}
	}`)}
	rt := &fakeRT{}
	svc := New(configedit.New(mem, t.TempDir(), data, "127.0.0.1:8080"), rt)
	view, err := svc.AddAddress(context.Background(), "/ip4/203.0.113.10/tcp/4001/p2p/"+listPeer)
	if err != nil {
		t.Fatal(err)
	}
	if view.Sync != SyncOK || len(view.Peers) != 1 || view.Peers[0].PeerID != listPeer {
		t.Fatalf("%#v", view)
	}
	if view.Peers[0].State != StateReconnecting {
		t.Fatalf("state %s", view.Peers[0].State)
	}
	if strings.Contains(string(mem.replaced), secretKey) || strings.Contains(string(mem.replaced), "PrivKey") {
		t.Fatalf("replace leaked the key: %s", mem.replaced)
	}
	if !strings.Contains(string(mem.replaced), listPeer) || !strings.Contains(string(mem.replaced), "/ip4/127.0.0.1/tcp/5001") {
		t.Fatalf("replace %s", mem.replaced)
	}
	backup := readBackup(t, data)
	if strings.Contains(backup, secretKey) || strings.Contains(backup, "PrivKey") {
		t.Fatalf("backup leaked the key: %s", backup)
	}
	if len(rt.live) != 1 || rt.live[0].Addrs[0] != "/ip4/203.0.113.10/tcp/4001" {
		t.Fatalf("live %#v", rt.live)
	}
}

func TestServiceLiveFailureAsksForRestart(t *testing.T) {
	mem := &memKubo{doc: []byte(`{
	  "Identity": {"PeerID": "12D3KooWexample"},
	  "Addresses": {"API": "/ip4/127.0.0.1/tcp/5001", "Gateway": ["/ip4/127.0.0.1/tcp/8081"]}
	}`)}
	rt := &fakeRT{failAdd: true}
	svc := New(configedit.New(mem, t.TempDir(), t.TempDir(), "127.0.0.1:8080"), rt)
	view, err := svc.AddAddress(context.Background(), "/ip4/203.0.113.10/tcp/4001/p2p/"+listPeer)
	if err != nil {
		t.Fatal(err)
	}
	if view.Sync != SyncRestart || len(view.Peers) != 1 {
		t.Fatalf("%#v", view)
	}
	if !strings.Contains(string(mem.doc), listPeer) {
		t.Fatal("config was not saved")
	}
}

func TestServiceApplyAndRemove(t *testing.T) {
	mem := &memKubo{doc: []byte(`{
	  "Identity": {"PeerID": "12D3KooWexample"},
	  "Addresses": {"API": "/ip4/127.0.0.1/tcp/5001", "Gateway": ["/ip4/127.0.0.1/tcp/8081"]},
	  "Peering": {"Peers": [{"ID": "` + listPeer + `", "Addrs": ["/ip4/203.0.113.10/tcp/4001"]}]}
	}`)}
	rt := &fakeRT{}
	svc := New(configedit.New(mem, t.TempDir(), t.TempDir(), "127.0.0.1:8080"), rt)
	view, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Sync != SyncApply || view.Peers[0].State != StatePending {
		t.Fatalf("%#v", view)
	}
	view, err = svc.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Sync != SyncOK || view.Peers[0].State != StateReconnecting {
		t.Fatalf("after apply %#v live %#v", view, rt.live)
	}
	view, err = svc.Remove(context.Background(), listPeer)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Peers) != 0 || view.Sync != SyncOK || len(rt.live) != 0 {
		t.Fatalf("%#v live %#v", view, rt.live)
	}
}

func TestServiceKeepFromRow(t *testing.T) {
	mem := &memKubo{doc: []byte(`{
	  "Identity": {"PeerID": "12D3KooWexample"},
	  "Addresses": {"API": "/ip4/127.0.0.1/tcp/5001", "Gateway": ["/ip4/127.0.0.1/tcp/8081"]}
	}`)}
	rt := &fakeRT{
		connected: map[string]struct{}{listPeer: {}},
		byID:      map[string][]string{listPeer: {"/ip4/203.0.113.10/tcp/4001"}},
	}
	svc := New(configedit.New(mem, t.TempDir(), t.TempDir(), "127.0.0.1:8080"), rt)
	view, err := svc.AddPeer(context.Background(), listPeer)
	if err != nil {
		t.Fatal(err)
	}
	if view.Sync != SyncOK || view.Peers[0].State != StateConnected || view.Peers[0].Addrs[0] != "/ip4/203.0.113.10/tcp/4001" {
		t.Fatalf("%#v", view)
	}
	if _, err := svc.AddPeer(context.Background(), "not-a-peer"); CodeOf(err) != "invalid_peer" {
		t.Fatalf("%v", err)
	}
	if _, err := svc.AddPeer(context.Background(), "12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d"); CodeOf(err) != "not_connected" {
		t.Fatalf("%v", err)
	}
}

func readBackup(t *testing.T, data string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(data, "config-backups"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d backups", len(entries))
	}
	body, err := os.ReadFile(filepath.Join(data, "config-backups", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
