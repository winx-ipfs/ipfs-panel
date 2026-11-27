package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func TestConfigRequiresSessionAndCSRF(t *testing.T) {
	mux := configMux(t, &configHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("handler ran without a session")
			return node.Overview{}, nil
		},
	})
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/config", "", "")
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), `"unauthorized"`) {
		t.Fatalf("anon %d %s", rr.Code, rr.Body.Bytes())
	}
	svcMux := configMux(t, &configHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("csrf should stop the handler")
			return node.Overview{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{"text":"{}"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	rr = httptest.NewRecorder()
	svcMux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"csrf"`) {
		t.Fatalf("csrf %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestConfigStoppedNode(t *testing.T) {
	mux := configMux(t, &configHTTP{
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateDown}, nil
		},
	})
	rr := do(mux, http.MethodGet, "/api/config", "", "")
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) || !strings.Contains(rr.Body.String(), `"down"`) {
		t.Fatalf("get %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/config", "application/json", `{"text":"{}"}`)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) {
		t.Fatalf("post %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestConfigReplaceAuditsWithoutTheDocument(t *testing.T) {
	repo := t.TempDir()
	data := t.TempDir()
	doc := map[string]any{
		"Identity": map[string]any{"PeerID": "12D3KooWexample", "PrivKey": "secret-key"},
		"Addresses": map[string]any{
			"API":     "/ip4/127.0.0.1/tcp/5001",
			"Gateway": []any{"/ip4/127.0.0.1/tcp/8081"},
		},
		"Discovery": map[string]any{"MDNS": map[string]any{"Enabled": true}},
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	kubo := &configMem{doc: doc}
	svc := configedit.New(kubo, repo, data, "127.0.0.1:8080")
	mux := configMux(t, &configHTTP{
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp, RepoPath: repo}, nil
		},
		edit: svc,
		log:  slog.New(slog.NewTextHandler(&buf, nil)),
	})
	rr := do(mux, http.MethodGet, "/api/config", "", "")
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "secret-key") || strings.Contains(rr.Body.String(), "PrivKey") {
		t.Fatalf("get %d %s", rr.Code, rr.Body.Bytes())
	}
	var view struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(view.Text, `"Enabled": true`, `"Enabled": false`, 1)
	rr = do(mux, http.MethodPost, "/api/config", "application/json", `{"text":`+mustJSON(t, edited)+`,"extra":true}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("extra %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/config", "text/plain", `{"text":"{}"}`)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("type %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/config", "application/json", `{"text":`+mustJSON(t, edited)+`}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"restart":"needed"`) {
		t.Fatalf("save %d %s", rr.Code, rr.Body.Bytes())
	}
	if strings.Contains(rr.Body.String(), "secret-key") || strings.Contains(buf.String(), "secret-key") || strings.Contains(buf.String(), edited) {
		t.Fatal("response or audit included the document or the private key")
	}
	if !strings.Contains(buf.String(), "event=config_replace") || !strings.Contains(buf.String(), "result=success") {
		t.Fatalf("audit\n%s", buf.String())
	}
	rr = do(mux, http.MethodPost, "/api/config/restore", "application/json", `{}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("restore %d %s", rr.Code, rr.Body.Bytes())
	}
	var restored struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(restored.Text, `"Enabled": true`) {
		t.Fatalf("restore text %s", restored.Text)
	}
}

func configMux(t *testing.T, h *configHTTP) http.Handler {
	t.Helper()
	sup, err := node.New(node.Config{ManagerAddr: "127.0.0.1:8080", RepoPath: t.TempDir(), DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	svc, err := auth.New(t.TempDir(), "", auth.Options{Params: auth.TestingParams()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Setup("admin", "correct-horse-battery", "127.0.0.1:9", "")
	if err != nil {
		t.Fatal(err)
	}
	useSession(res.Session, res.CSRF)
	return newMux(parts{sup: sup, auth: svc, config: h})
}

func mustJSON(t *testing.T, text string) string {
	t.Helper()
	body, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

type configMem struct {
	doc map[string]any
}

func (m *configMem) Show(context.Context) ([]byte, error) {
	return json.Marshal(m.doc)
}

func (m *configMem) Replace(_ context.Context, document []byte) error {
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
	return nil
}
