package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func testMux(t *testing.T) (*node.Supervisor, http.Handler) {
	t.Helper()
	sup, err := node.New(node.Config{
		ManagerAddr:     "127.0.0.1:8080",
		RepoPath:        t.TempDir(),
		KuboBin:         t.TempDir() + "/missing-ipfs",
		InitTimeout:     time.Second,
		ConfigTimeout:   time.Second,
		IDTimeout:       time.Second,
		ReadyBudget:     time.Second,
		ShutdownTimeout: time.Second,
		StopKillAfter:   time.Second,
		PollInterval:    10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	return sup, NewMux(sup)
}

func TestHealth(t *testing.T) {
	_, mux := testMux(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	assertNoCORS(t, rr.Header())

	var body healthBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Service != "manager" {
		t.Fatalf("body = %#v", body)
	}
}

func TestHealthPost(t *testing.T) {
	_, mux := testMux(t)
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rr.Code)
	}
	assertNoCORS(t, rr.Header())
}

func TestNotFound(t *testing.T) {
	_, mux := testMux(t)
	for _, path := range []string{"/", "/api/files", "/api/pins", "/api/nodes"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d", path, rr.Code)
		}
		assertNoCORS(t, rr.Header())
	}
}

func assertNoCORS(t *testing.T, h http.Header) {
	t.Helper()
	if h.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("Access-Control-Allow-Origin is set")
	}
}
