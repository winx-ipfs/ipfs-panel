package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/logs"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type fakeLevel struct {
	called bool
	err    error
}

func (f *fakeLevel) SetLevel(context.Context, string, string) (string, error) {
	f.called = true
	if f.err != nil {
		return "", f.err
	}
	return "Changed log level of 'core' to 'info'", nil
}

func (f *fakeLevel) Subsystems(context.Context) ([]string, error) {
	return []string{"core"}, nil
}

func TestLogsSessionCSRFAndStoppedNode(t *testing.T) {
	store := openLogStore(t)
	mux := logsMux(t, &logsHTTP{
		store: store,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateDown}, nil
		},
		dial: func(context.Context, string) (levelClient, error) {
			t.Fatal("kubo was called while the node is down")
			return nil, nil
		},
	})
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/logs", "", "")
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), `"unauthorized"`) {
		t.Fatalf("anon %d %s", rr.Code, rr.Body.Bytes())
	}
	mux = logsMux(t, &logsHTTP{
		store: store,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateDown}, nil
		},
	})
	rr = do(mux, http.MethodGet, "/api/logs", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"node":"down"`) || !strings.Contains(rr.Body.String(), `"lines":[]`) {
		t.Fatalf("stopped get %d %s", rr.Code, rr.Body.Bytes())
	}
	req := httptest.NewRequest(http.MethodPost, "/api/logs/level", strings.NewReader(`{"subsystem":"all","level":"info"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"csrf"`) {
		t.Fatalf("csrf %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/logs/level", "application/json", `{"subsystem":"all","level":"info"}`)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) || !strings.Contains(rr.Body.String(), `"down"`) {
		t.Fatalf("level %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestLogsBadLevelDoesNotCallKubo(t *testing.T) {
	fake := &fakeLevel{}
	var buf bytes.Buffer
	mux := logsMux(t, &logsHTTP{
		store: openLogStore(t),
		log:   slog.New(slog.NewTextHandler(&buf, nil)),
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp, RepoPath: t.TempDir()}, nil
		},
		dial: func(context.Context, string) (levelClient, error) {
			return fake, nil
		},
	})
	rr := do(mux, http.MethodPost, "/api/logs/level", "application/json", `{"subsystem":"core","level":"nope"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"bad_level"`) || fake.called {
		t.Fatalf("bad level %d %s called %v", rr.Code, rr.Body.Bytes(), fake.called)
	}
	if strings.Contains(buf.String(), "nope") && strings.Contains(buf.String(), "secret") {
		t.Fatal(buf.String())
	}
	rr = do(mux, http.MethodPost, "/api/logs/level", "application/json", `{"subsystem":"core","level":"info"}`)
	if rr.Code != http.StatusOK || !fake.called || !strings.Contains(buf.String(), "event=log_level") || !strings.Contains(buf.String(), "result=success") {
		t.Fatalf("set %d %s\n%s", rr.Code, rr.Body.Bytes(), buf.String())
	}
	rr = do(mux, http.MethodGet, "/api/logs/levels", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"core"`) {
		t.Fatalf("levels %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestLogsDownloadAndStream(t *testing.T) {
	store := openLogStore(t)
	const secret = "c2VjcmV0a2V5"
	store.AppendManager("info", "manager listening", time.Now())
	store.AppendManager("error", "PrivKey="+secret, time.Now())
	mux := logsMux(t, &logsHTTP{
		store: store,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp}, nil
		},
		capture: func() bool { return true },
	})
	rr := do(mux, http.MethodGet, "/api/logs?level=debug", "", "")
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), secret) || !strings.Contains(rr.Body.String(), logs.SourceManager) {
		t.Fatalf("list %d %s", rr.Code, rr.Body.Bytes())
	}
	if !strings.Contains(rr.Body.String(), `"capture":true`) {
		t.Fatalf("capture %s", rr.Body.Bytes())
	}
	rr = do(mux, http.MethodGet, "/api/logs/download?source=manager", "", "")
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), secret) || !strings.Contains(rr.Body.String(), "[redacted]") {
		t.Fatalf("download %d %s", rr.Code, rr.Body.Bytes())
	}
	if !strings.Contains(rr.Header().Get("Content-Disposition"), "ipfs-panel-manager.txt") {
		t.Fatal(rr.Header().Get("Content-Disposition"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/logs/stream?after=0&level=debug", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	req.Header.Set(auth.CSRFHeader, sessionCSRF)
	body := &safeWriter{hit: make(chan struct{}, 1)}
	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(body, req)
		close(done)
	}()
	select {
	case <-body.hit:
	case <-time.After(2 * time.Second):
		t.Fatalf("stream %s", body.snapshot())
	}
	text := body.snapshot()
	if !strings.Contains(text, "manager listening") || !strings.Contains(text, "event: line") || strings.Contains(text, secret) {
		t.Fatalf("stream %s", text)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not stop")
	}
}

type safeWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	header http.Header
	hit    chan struct{}
	once   sync.Once
}

func (s *safeWriter) Header() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.header == nil {
		s.header = make(http.Header)
	}
	return s.header
}

func (s *safeWriter) WriteHeader(int) {}

func (s *safeWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	n, err := s.buf.Write(p)
	ready := strings.Contains(s.buf.String(), "manager listening")
	s.mu.Unlock()
	if ready {
		s.once.Do(func() { s.hit <- struct{}{} })
	}
	return n, err
}

func (s *safeWriter) Flush() {}

func (s *safeWriter) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func openLogStore(t *testing.T) *logs.Store {
	t.Helper()
	store, err := logs.Open(logs.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func logsMux(t *testing.T, h *logsHTTP) http.Handler {
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
	if h.store == nil {
		h.store = openLogStore(t)
	}
	return newMux(parts{sup: sup, auth: svc, logs: h})
}
