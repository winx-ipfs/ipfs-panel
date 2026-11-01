package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/fakebin"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

var (
	sessionMu     sync.Mutex
	sessionCookie string
	sessionCSRF   string
)

func fakeMux(t *testing.T) (*node.Supervisor, http.Handler, string) {
	t.Helper()
	bin := fakebin.Path(t)
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := t.TempDir()
	sup, err := node.New(node.Config{
		ManagerAddr:     "127.0.0.1:8080",
		RepoPath:        repo,
		InitTimeout:     3 * time.Second,
		ConfigTimeout:   2 * time.Second,
		IDTimeout:       time.Second,
		ReadyBudget:     3 * time.Second,
		ShutdownTimeout: time.Second,
		StopKillAfter:   3 * time.Second,
		PollInterval:    15 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	return sup, NewMux(sup, svc), repo
}

func useSession(cookie, csrf string) {
	sessionMu.Lock()
	sessionCookie, sessionCSRF = cookie, csrf
	sessionMu.Unlock()
}

func do(mux http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" && method == http.MethodPost {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	sessionMu.Lock()
	cookie, csrf := sessionCookie, sessionCSRF
	sessionMu.Unlock()
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookie})
		req.Header.Set(auth.CSRFHeader, csrf)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodeOverview(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v (%s)", err, rr.Body.Bytes())
	}
	for key := range body {
		switch key {
		case "state", "peerId", "version", "repoPath", "error":
		default:
			t.Fatalf("unexpected key %s in %s", key, rr.Body.Bytes())
		}
	}
	raw := rr.Body.String()
	for _, forbidden := range []string{"0.0.0.0", "multiaddr", "/tcp/5001", "Protocols"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("body contains %s: %s", forbidden, raw)
		}
	}
	assertNoCORS(t, rr.Header())
	return body
}

func TestNodeDown(t *testing.T) {
	_, mux, repo := fakeMux(t)
	rr := do(mux, http.MethodGet, "/api/node", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type %s", ct)
	}
	body := decodeOverview(t, rr)
	if body["state"] != "down" || body["peerId"] != nil || body["version"] != nil || body["error"] != nil {
		t.Fatalf("%v", body)
	}
	if body["repoPath"] != repo {
		t.Fatalf("repo %v", body["repoPath"])
	}
}

func TestPostContentTypeAndBody(t *testing.T) {
	_, mux, _ := fakeMux(t)

	rr := do(mux, http.MethodPost, "/api/node/start", "", "{}")
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing type %d", rr.Code)
	}
	assertNoCORS(t, rr.Header())

	rr = do(mux, http.MethodPost, "/api/node/stop", "text/plain", "{}")
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text %d", rr.Code)
	}

	rr = do(mux, http.MethodPost, "/api/node/restart", "application/json", `{"repo":"/tmp"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("keys %d %s", rr.Code, rr.Body.Bytes())
	}

	rr = do(mux, http.MethodPost, "/api/node/start", "application/json", "[]")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("array %d", rr.Code)
	}

	rr = do(mux, http.MethodPost, "/api/node/stop", "application/json", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("empty body %d %s", rr.Code, rr.Body.Bytes())
	}
	body := decodeOverview(t, rr)
	if body["state"] != "down" {
		t.Fatalf("%v", body)
	}
}

func TestPostStartAndMethod(t *testing.T) {
	_, mux, _ := fakeMux(t)
	rr := do(mux, http.MethodPost, "/api/node/start", "application/json; charset=utf-8", "{}")
	if rr.Code != http.StatusOK {
		t.Fatalf("start %d %s", rr.Code, rr.Body.Bytes())
	}
	body := decodeOverview(t, rr)
	if body["state"] != "starting" {
		t.Fatalf("start returned %v", body["state"])
	}

	deadline := time.Now().Add(4 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		got := do(mux, http.MethodGet, "/api/node", "", "")
		overview := decodeOverview(t, got)
		state, _ = overview["state"].(string)
		if state == "up" || state == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state != "up" {
		t.Fatalf("ended in %s", state)
	}

	options := httptest.NewRequest(http.MethodOptions, "/api/node/start", nil)
	options.Header.Set("Origin", "https://evil.example")
	options.Header.Set("Access-Control-Request-Method", "POST")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, options)
	if rr.Code == http.StatusOK {
		t.Fatalf("options %d", rr.Code)
	}
	assertNoCORS(t, rr.Header())

	getStart := do(mux, http.MethodGet, "/api/node/start", "", "")
	if getStart.Code != http.StatusMethodNotAllowed {
		t.Fatalf("get start %d", getStart.Code)
	}
}

func TestOverlappingHTTPStarts(t *testing.T) {
	t.Setenv("FAKE_INIT_SLEEP", "600ms")
	_, mux, _ := fakeMux(t)
	type result struct {
		code  int
		state string
	}
	ch := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			rr := do(mux, http.MethodPost, "/api/node/start", "application/json", "{}")
			var body map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &body)
			state, _ := body["state"].(string)
			ch <- result{code: rr.Code, state: state}
		}()
	}
	for i := 0; i < 2; i++ {
		got := <-ch
		if got.code != http.StatusOK || got.state != "starting" {
			t.Fatalf("%+v", got)
		}
	}
}
