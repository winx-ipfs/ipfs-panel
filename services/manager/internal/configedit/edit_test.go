package configedit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/scenes"
)

func TestShowStripsSecrets(t *testing.T) {
	repo, data := t.TempDir(), t.TempDir()
	writeRepo(t, repo, sampleDoc())
	kubo := &memKubo{doc: sampleDoc()}
	svc := New(kubo, repo, data, "127.0.0.1:8080")
	view, err := svc.Show(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(view.Text, "secret-key") || strings.Contains(view.Text, "PrivKey") {
		t.Fatalf("private key leaked: %s", view.Text)
	}
	if strings.Contains(view.Text, "pin-secret") || strings.Contains(view.Text, "auth-secret") {
		t.Fatalf("secret leaked: %s", view.Text)
	}
	if !strings.Contains(view.Text, "12D3KooWexample") {
		t.Fatalf("peer id missing: %s", view.Text)
	}
	if len(view.SceneKeys) == 0 {
		t.Fatal("scene keys missing")
	}
}

func TestReplaceRefusesLockedFieldsWithoutCallingKubo(t *testing.T) {
	repo, data := t.TempDir(), t.TempDir()
	writeRepo(t, repo, sampleDoc())
	kubo := &memKubo{doc: cloneDoc(t, sampleDoc())}
	svc := New(kubo, repo, data, "127.0.0.1:8080")
	base, err := svc.Show(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		text string
		code string
	}{
		{"private key", strings.Replace(base.Text, `"PeerID"`, `"PrivKey": "new-secret", "PeerID"`, 1), "privkey_locked"},
		{"api", strings.Replace(base.Text, "/ip4/127.0.0.1/tcp/5001", "/ip4/127.0.0.1/tcp/5002", 1), "api_locked"},
		{"peer", strings.Replace(base.Text, "12D3KooWexample", "12D3KooWother", 1), "identity_locked"},
		{"auth", strings.TrimRight(base.Text, "\n") + "\n", "secret_locked"},
	}
	auth := strings.Replace(base.Text, `"Addresses"`, `"API": {"Authorizations": {"x": {}}}, "Addresses"`, 1)
	cases[3].text = auth
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := kubo.calls
			if _, err := svc.Replace(context.Background(), tc.text); CodeOf(err) != tc.code {
				t.Fatalf("%v", err)
			}
			if kubo.calls != before {
				t.Fatalf("replace calls %d", kubo.calls)
			}
		})
	}
}

func TestReplaceWritesBackupAndRestorePutsItBack(t *testing.T) {
	repo, data := t.TempDir(), t.TempDir()
	doc := sampleDoc()
	writeRepo(t, repo, doc)
	kubo := &memKubo{doc: cloneDoc(t, doc), repo: repo}
	svc := New(kubo, repo, data, "127.0.0.1:8080")
	base, err := svc.Show(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(base.Text, `"Enabled": true`, `"Enabled": false`, 1)
	res, err := svc.Replace(context.Background(), edited)
	if err != nil {
		t.Fatal(err)
	}
	if res.Restart != "needed" || !res.Backup || kubo.calls != 1 {
		t.Fatalf("result %#v calls %d", res, kubo.calls)
	}
	if strings.Contains(string(kubo.last), "secret-key") || strings.Contains(string(kubo.last), "PrivKey") {
		t.Fatal("replace body contained the private key")
	}
	if strings.Contains(res.Text, `"Enabled": false`) == false {
		t.Fatalf("edited value missing: %s", res.Text)
	}
	info, err := os.Stat(filepath.Join(data, "config-backups"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("backup dir")
	}
	names, err := os.ReadDir(filepath.Join(data, "config-backups"))
	if err != nil || len(names) != 1 {
		t.Fatalf("backups %v %v", names, err)
	}
	binfo, err := names[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerOnly(t, binfo)
	again, err := svc.Replace(context.Background(), res.Text)
	if err != nil {
		t.Fatal(err)
	}
	if again.Restart != "none" || kubo.calls != 1 {
		t.Fatalf("unchanged restart %s calls %d", again.Restart, kubo.calls)
	}
	restored, err := svc.Restore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.Text, `"Enabled": true`) || restored.Backup {
		t.Fatalf("restore %#v", restored.Text)
	}
	if _, err := svc.Restore(context.Background()); CodeOf(err) != "nothing_to_restore" {
		t.Fatal(err)
	}
}

func TestKuboRejectionDropsTheBackup(t *testing.T) {
	repo, data := t.TempDir(), t.TempDir()
	writeRepo(t, repo, sampleDoc())
	kubo := &memKubo{doc: cloneDoc(t, sampleDoc()), err: code(502, "kubo_rejected")}
	svc := New(kubo, repo, data, "127.0.0.1:8080")
	base, err := svc.Show(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(base.Text, `"Enabled": true`, `"Enabled": false`, 1)
	if _, err := svc.Replace(context.Background(), edited); CodeOf(err) != "kubo_rejected" {
		t.Fatal(err)
	}
	ok, err := hasBackup(data)
	if err != nil || ok {
		t.Fatalf("backup left behind: %v %v", ok, err)
	}
}

func TestSceneStaysUntilAManagedKeyChanges(t *testing.T) {
	repo, data := t.TempDir(), t.TempDir()
	writeRepo(t, repo, sampleDoc())
	rt := &sceneRuntime{state: node.StateDown}
	if _, err := scenes.Apply(context.Background(), repo, data, "127.0.0.1:8080", scenes.Quiet, rt); err != nil {
		t.Fatal(err)
	}
	doc := readRepo(t, repo)
	kubo := &memKubo{doc: doc, repo: repo}
	svc := New(kubo, repo, data, "127.0.0.1:8080")
	view, err := svc.Show(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Scene != scenes.Quiet {
		t.Fatalf("scene %q", view.Scene)
	}
	kept, err := svc.Replace(context.Background(), withCustom(view.Text))
	if err != nil {
		t.Fatal(err)
	}
	if kept.Diverged || kept.Scene != scenes.Quiet {
		t.Fatalf("non-scene edit diverged %#v", kept)
	}
	left := strings.Replace(kept.Text, `"autoclient"`, `"auto"`, 1)
	moved, err := svc.Replace(context.Background(), left)
	if err != nil {
		t.Fatal(err)
	}
	if !moved.Diverged || moved.Scene == scenes.Quiet {
		t.Fatalf("scene edit %#v", moved)
	}
	if _, err := scenes.Revert(context.Background(), repo, data, "127.0.0.1:8080", rt); err != nil {
		t.Fatal(err)
	}
	back := readRepo(t, repo)
	if back["Identity"].(map[string]any)["PrivKey"] != "secret-key" {
		t.Fatal("scene undo lost the private key")
	}
	if _, err := scenes.Apply(context.Background(), repo, data, "127.0.0.1:8080", scenes.Quiet, rt); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Show(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.Scene != scenes.Quiet {
		t.Fatalf("scene apply after a raw edit: %q", again.Scene)
	}
	_ = back
}

func TestInvalidJSON(t *testing.T) {
	svc := New(&memKubo{doc: sampleDoc()}, t.TempDir(), t.TempDir(), "127.0.0.1:8080")
	if _, err := svc.Replace(context.Background(), "{"); CodeOf(err) != "invalid_json" {
		t.Fatal(err)
	}
	if _, err := svc.Replace(context.Background(), "[]"); CodeOf(err) != "not_object" {
		t.Fatal(err)
	}
	if _, err := svc.Replace(context.Background(), strings.Repeat("a", MaxDocument+1)); CodeOf(err) != "too_large" {
		t.Fatal(err)
	}
}

func withCustom(text string) string {
	return strings.Replace(text, "{", "{\n  \"Custom\": {\"Keep\": true},", 1)
}

func sampleDoc() map[string]any {
	return map[string]any{
		"Identity": map[string]any{"PeerID": "12D3KooWexample", "PrivKey": "secret-key"},
		"Addresses": map[string]any{
			"API":     "/ip4/127.0.0.1/tcp/5001",
			"Gateway": []any{"/ip4/127.0.0.1/tcp/8081"},
			"Swarm":   []any{"/ip4/0.0.0.0/tcp/4001"},
		},
		"API": map[string]any{"Authorizations": map[string]any{"local": "auth-secret"}},
		"Pinning": map[string]any{
			"RemoteServices": map[string]any{
				"example": map[string]any{"API": map[string]any{"Endpoint": "http://127.0.0.1:9", "Key": "pin-secret"}},
			},
		},
		"Discovery": map[string]any{"MDNS": map[string]any{"Enabled": true}},
	}
}

func writeRepo(t *testing.T, repo string, doc map[string]any) {
	t.Helper()
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readRepo(t *testing.T, repo string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func cloneDoc(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

type memKubo struct {
	doc   map[string]any
	repo  string
	err   error
	calls int
	last  []byte
}

func (m *memKubo) Show(context.Context) ([]byte, error) {
	body, err := json.Marshal(m.doc)
	return body, err
}

func (m *memKubo) Replace(_ context.Context, document []byte) error {
	m.calls++
	m.last = append([]byte(nil), document...)
	if m.err != nil {
		return m.err
	}
	var next map[string]any
	if err := json.Unmarshal(document, &next); err != nil {
		return err
	}
	if ident, ok := m.doc["Identity"].(map[string]any); ok {
		if got, ok := next["Identity"].(map[string]any); ok {
			got["PrivKey"] = ident["PrivKey"]
		}
	}
	m.doc = next
	if m.repo != "" {
		body, err := json.MarshalIndent(next, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(m.repo, "config"), append(body, '\n'), 0o600); err != nil {
			return err
		}
	}
	return nil
}

type sceneRuntime struct {
	state string
}

func (s *sceneRuntime) Cached(context.Context) (node.Overview, error) {
	return node.Overview{State: s.state}, nil
}

func (s *sceneRuntime) Restart(context.Context) (node.Overview, error) {
	s.state = node.StateStopping
	return node.Overview{State: node.StateStopping}, nil
}
