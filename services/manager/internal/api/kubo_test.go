package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/fakebin"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuboinstall"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func TestKuboFlagIsNotInstalledAndStartIsHeld(t *testing.T) {
	bin := fakebin.Path(t)
	repo := t.TempDir()
	data := t.TempDir()
	sup, err := node.New(node.Config{
		ManagerAddr:     "127.0.0.1:8080",
		RepoPath:        repo,
		DataDir:         data,
		KuboBin:         bin,
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
	accounts, err := auth.New(t.TempDir(), "", auth.Options{Params: auth.TestingParams()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := accounts.Setup("admin", "correct-horse-battery", "127.0.0.1:9", "")
	if err != nil {
		t.Fatal(err)
	}
	useSession(res.Session, res.CSRF)
	dist := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/versions") {
			_, _ = io.WriteString(w, "v0.43.1\n")
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(dist.Close)
	installer := kuboinstall.New(kuboinstall.Options{
		DataDir:     data,
		RepoDir:     repo,
		ExplicitBin: bin,
		DistBase:    dist.URL,
		GitHubBase:  dist.URL,
		VersionsURL: dist.URL + "/kubo/versions",
		GitHubAPI:   dist.URL + "/releases",
		HTTP:        dist.Client(),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	mux := newMux(parts{
		sup:  sup,
		auth: accounts,
		kubo: &kuboHTTP{sup: sup, svc: installer},
	})

	anon := httptest.NewRequest(http.MethodGet, "/api/kubo", nil)
	anonRR := httptest.NewRecorder()
	mux.ServeHTTP(anonRR, anon)
	if anonRR.Code != http.StatusUnauthorized {
		t.Fatalf("anon %d", anonRR.Code)
	}

	rr := do(mux, http.MethodGet, "/api/kubo", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"source":"flag"`) {
		t.Fatalf("status %d %s", rr.Code, rr.Body.Bytes())
	}
	if strings.Contains(rr.Body.String(), "PrivKey") || strings.Contains(rr.Body.String(), bin) {
		t.Fatalf("body leaked %s", rr.Body.Bytes())
	}

	bad := do(mux, http.MethodPost, "/api/kubo/install", "application/json", `{"url":"http://evil.example"}`)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad %d %s", bad.Code, bad.Body.Bytes())
	}
	install := do(mux, http.MethodPost, "/api/kubo/install", "application/json", `{}`)
	if install.Code != http.StatusConflict || !strings.Contains(install.Body.String(), `"external_binary"`) {
		t.Fatalf("install %d %s", install.Code, install.Body.Bytes())
	}
	missing := do(mux, http.MethodGet, "/api/kubo/jobs/nope", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("job %d %s", missing.Code, missing.Body.Bytes())
	}

	sup.SetStartHold(true)
	held := do(mux, http.MethodPost, "/api/node/start", "application/json", `{}`)
	if held.Code != http.StatusConflict || !strings.Contains(held.Body.String(), `"kubo_busy"`) {
		t.Fatalf("held %d %s", held.Code, held.Body.Bytes())
	}
	if _, err := filepath.Abs(data); err != nil {
		t.Fatal(err)
	}
}

func TestKuboPostNeedsCSRF(t *testing.T) {
	_, mux, _ := fakeMux(t)
	req := httptest.NewRequest(http.MethodPost, "/api/kubo/install", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	sessionMu.Lock()
	cookie := sessionCookie
	sessionMu.Unlock()
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: cookie})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"csrf"`) {
		t.Fatalf("csrf %d %s", rr.Code, rr.Body.Bytes())
	}
}
