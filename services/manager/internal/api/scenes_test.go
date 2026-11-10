package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/scenes"
)

func TestScenesRequireSessionAndCSRF(t *testing.T) {
	mux, _ := sceneMux(t)
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/scenes", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
	mux, _ = sceneMux(t)
	req := do(mux, http.MethodPost, "/api/scenes/apply", "text/plain", `{"id":"quiet"}`)
	if req.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("type %d %s", req.Code, req.Body.Bytes())
	}
	sessionMu.Lock()
	cookie := sessionCookie
	sessionMu.Unlock()
	bareReq := httptest.NewRequest(http.MethodPost, "/api/scenes/apply", strings.NewReader(`{"id":"quiet"}`))
	bareReq.Header.Set("Content-Type", "application/json")
	bareReq.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookie})
	bare := httptest.NewRecorder()
	mux.ServeHTTP(bare, bareReq)
	if bare.Code != http.StatusForbidden {
		t.Fatalf("csrf %d %s", bare.Code, bare.Body.Bytes())
	}
}

func TestScenesApplyAndRevertWhileStopped(t *testing.T) {
	mux, repo := sceneMux(t)
	rr := do(mux, http.MethodGet, "/api/scenes/preview?id=share", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("preview %d %s", rr.Code, rr.Body.Bytes())
	}
	if strings.Contains(rr.Body.String(), "0.0.0.0") || strings.Contains(rr.Body.String(), "/tcp/5001") {
		t.Fatalf("preview leaked %s", rr.Body.Bytes())
	}
	var preview struct {
		Changes []scenes.Change `json:"changes"`
		Restart bool            `json:"restart"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Restart || !hasChange(preview.Changes, "Provide.Strategy", "mfs") {
		t.Fatalf("%#v", preview)
	}
	rr = do(mux, http.MethodPost, "/api/scenes/apply", "application/json", `{"id":"share","extra":true}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("extra %d", rr.Code)
	}
	rr = do(mux, http.MethodPost, "/api/scenes/apply", "application/json", `{"id":"nope"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/scenes/apply", "application/json", `{"id":"share"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"restart":"later"`) {
		t.Fatalf("apply %d %s", rr.Code, rr.Body.Bytes())
	}
	body, err := os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"mfs"`) || !strings.Contains(string(body), "secret-key") {
		t.Fatalf("config %s", body)
	}
	if strings.Contains(rr.Body.String(), "secret-key") {
		t.Fatal("response included the private key")
	}
	rr = do(mux, http.MethodPost, "/api/scenes/revert", "application/json", `{}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"restart":"later"`) {
		t.Fatalf("revert %d %s", rr.Code, rr.Body.Bytes())
	}
	body, err = os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"mfs"`) {
		t.Fatalf("strategy remained %s", body)
	}
}

func sceneMux(t *testing.T) (http.Handler, string) {
	t.Helper()
	repo := t.TempDir()
	doc := map[string]any{
		"Identity": map[string]any{"PrivKey": "secret-key"},
		"Addresses": map[string]any{
			"API":     "/ip4/127.0.0.1/tcp/5001",
			"Gateway": []any{"/ip4/127.0.0.1/tcp/8081"},
			"Swarm":   []any{"/ip4/0.0.0.0/tcp/4001"},
		},
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	sup, err := node.New(node.Config{
		ManagerAddr:     "127.0.0.1:8080",
		RepoPath:        repo,
		DataDir:         t.TempDir(),
		ShutdownTimeout: time.Second,
		StopKillAfter:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
	return NewMux(sup, svc), repo
}

func hasChange(changes []scenes.Change, key, code string) bool {
	for _, change := range changes {
		if change.Key == key && change.Code == code {
			return true
		}
	}
	return false
}
