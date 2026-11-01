package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func TestFilesRequireSessionAndCSRF(t *testing.T) {
	mux := filesMux(t, &filesHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("handler ran without a session check")
			return node.Overview{}, nil
		},
	})
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/files", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d %s", rr.Code, rr.Body.Bytes())
	}

	svcMux := filesMux(t, &filesHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("csrf should stop the handler")
			return node.Overview{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/files/mkdir", strings.NewReader(`{"path":"/a"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	rr = httptest.NewRecorder()
	svcMux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("csrf %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestFilesNodeDown(t *testing.T) {
	mux := filesMux(t, &filesHTTP{})
	rr := do(mux, http.MethodGet, "/api/files?path=/../etc", "", "")
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d %s", rr.Code, rr.Body.Bytes())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "node_not_running" || body["state"] != "down" {
		t.Fatalf("%#v", body)
	}
	if strings.Contains(rr.Body.String(), "5001") || strings.Contains(rr.Body.String(), "multiaddr") {
		t.Fatalf("leaked %s", rr.Body.String())
	}
}

func TestFilesRoutes(t *testing.T) {
	var hits int
	var wrote bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch {
		case strings.HasSuffix(r.URL.Path, "/files/ls"):
			_, _ = io.WriteString(w, `{"Entries":[{"Name":"notes.txt","Type":0,"Size":5,"Hash":"bafyabc","Mtime":1730000000},{"Name":"photos","Type":1,"Size":0,"Hash":"bafydir"}]}`)
		case strings.HasSuffix(r.URL.Path, "/files/stat"):
			arg := r.URL.Query().Get("arg")
			if arg == "/photos" || strings.HasSuffix(arg, "/photos") {
				_, _ = io.WriteString(w, `{"Hash":"bafydir","Size":0,"Type":"directory","Mtime":1730000000}`)
				return
			}
			if !wrote && (arg == "/notes.txt" || strings.HasSuffix(arg, "/notes.txt")) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"Message":"file does not exist"}`)
				return
			}
			_, _ = io.WriteString(w, `{"Hash":"bafyabc","Size":5,"Type":"file","Mtime":1730000000}`)
		case strings.HasSuffix(r.URL.Path, "/files/read"):
			_, _ = io.WriteString(w, "hello")
		case strings.HasSuffix(r.URL.Path, "/files/write"):
			mr, err := r.MultipartReader()
			if err != nil {
				t.Errorf("multipart %v", err)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				t.Errorf("part %v", err)
				return
			}
			buf, _ := io.ReadAll(part)
			if string(buf) != "hello" {
				t.Errorf("body %q", buf)
			}
			if r.ContentLength >= 0 {
				t.Error("upload was buffered with a length")
			}
			wrote = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()
	mux := filesMux(t, upFiles(t, srv.URL, 1024))

	rr := do(mux, http.MethodGet, "/api/files", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list %d %s", rr.Code, rr.Body.Bytes())
	}
	if strings.Contains(rr.Body.String(), srv.URL) || strings.Contains(rr.Body.String(), "5001") {
		t.Fatalf("leaked rpc %s", rr.Body.String())
	}
	var list struct {
		Path         string `json:"path"`
		LocalGateway string `json:"localGateway"`
		MaxBytes     int64  `json:"maxBytes"`
		Entries      []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			CID  string `json:"cid"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Path != "/" || list.LocalGateway != "http://127.0.0.1:8081" || list.MaxBytes != 1024 || len(list.Entries) != 2 {
		t.Fatalf("%#v", list)
	}
	if list.Entries[0].CID != "bafyabc" || list.Entries[1].Type != "directory" {
		t.Fatalf("%#v", list.Entries)
	}

	before := hits
	rr = do(mux, http.MethodGet, "/api/files?path=/../etc/passwd", "", "")
	if rr.Code != http.StatusBadRequest || hits != before {
		t.Fatalf("traversal %d hits %d->%d %s", rr.Code, before, hits, rr.Body.Bytes())
	}

	rr = do(mux, http.MethodPost, "/api/files/upload?path=/notes.txt", "application/octet-stream", "hello")
	if rr.Code != http.StatusCreated || !strings.Contains(rr.Body.String(), "bafyabc") {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.Bytes())
	}

	rr = do(mux, http.MethodGet, "/api/files/download?path=/notes.txt", "", "")
	if rr.Code != http.StatusOK || rr.Body.String() != "hello" {
		t.Fatalf("download %d %s", rr.Code, rr.Body.Bytes())
	}
	if !strings.Contains(rr.Header().Get("Content-Disposition"), "attachment") || rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers %#v", rr.Header())
	}
	if strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
		t.Fatal("download must not be html")
	}

	rr = do(mux, http.MethodPost, "/api/files/mkdir", "application/json", `{"path":"/photos"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("mkdir %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/files/rename", "application/json", `{"path":"/notes.txt","name":"renamed.txt"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "/renamed.txt") {
		t.Fatalf("rename %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/files/move", "application/json", `{"from":"/notes.txt","to":"/photos/notes.txt"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("move %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/files/copy", "application/json", `{"from":"/notes.txt","to":"/photos/copy.txt"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("copy %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/files/delete", "application/json", `{"path":"/notes.txt"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/files/mkdir", "application/json", `{"path":"/photos","extra":true}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown field %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestFilesUploadTooLarge(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	defer srv.Close()
	mux := filesMux(t, upFiles(t, srv.URL, 4))
	rr := do(mux, http.MethodPost, "/api/files/upload?path=/big.txt", "application/octet-stream", "hello")
	if rr.Code != http.StatusRequestEntityTooLarge || hits != 0 {
		t.Fatalf("status %d hits %d %s", rr.Code, hits, rr.Body.Bytes())
	}
}

func filesMux(t *testing.T, h *filesHTTP) http.Handler {
	t.Helper()
	sup, err := node.New(node.Config{ManagerAddr: "127.0.0.1:8080", RepoPath: t.TempDir()})
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
	return newMux(sup, svc, h, nil)
}

func upFiles(t *testing.T, apiBase string, max int64) *filesHTTP {
	t.Helper()
	return &filesHTTP{
		maxBytes:        max,
		op:              time.Second,
		transferTimeout: time.Second,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp, RepoPath: t.TempDir()}, nil
		},
		lookup: func(string) (string, string, error) {
			return apiBase, "http://127.0.0.1:8081", nil
		},
	}
}
