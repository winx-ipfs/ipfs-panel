package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/pins"
)

const (
	sampleV0 = "QmPVGjYFugq4XUyBfoTHG6c3qxfBS26jEdaFM1gdAVuMZ2"
	sampleV1 = "bafkreibceirceirceirceirceirceirceirceirceirceirceirceircei"
)

func TestPinsRequireSessionAndCSRF(t *testing.T) {
	mux := pinsMux(t, &pinsHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("handler ran without a session")
			return node.Overview{}, nil
		},
	})
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/pins", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d %s", rr.Code, rr.Body.Bytes())
	}

	svcMux := pinsMux(t, &pinsHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("csrf should stop the handler")
			return node.Overview{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/pins", strings.NewReader(`{"target":"`+sampleV1+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	rr = httptest.NewRecorder()
	svcMux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("csrf %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestPinsNodeDown(t *testing.T) {
	mux := pinsMux(t, &pinsHTTP{})
	rr := do(mux, http.MethodGet, "/api/pins", "", "")
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
	rr = do(mux, http.MethodGet, "/api/pins/jobs", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"jobs":[]`) {
		t.Fatalf("jobs %d %s", rr.Code, rr.Body.Bytes())
	}
	if strings.Contains(rr.Body.String(), "5001") {
		t.Fatalf("leaked %s", rr.Body.String())
	}
}

func TestPinsRejectsBadTarget(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	defer srv.Close()
	mux := pinsMux(t, upPins(t, srv.URL))
	rr := do(mux, http.MethodPost, "/api/pins", "application/json", `{"target":"hello"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_cid") || hits != 0 {
		t.Fatalf("status %d hits %d %s", rr.Code, hits, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/pins", "application/json", `{"target":"`+sampleV1+`","name":"a/b"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_name") || hits != 0 {
		t.Fatalf("name %d hits %d %s", rr.Code, hits, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/pins", "application/json", `{"target":"`+sampleV1+`","extra":true}`)
	if rr.Code != http.StatusBadRequest || hits != 0 {
		t.Fatalf("extra %d hits %d %s", rr.Code, hits, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/pins", "text/plain", `{"target":"`+sampleV1+`"}`)
	if rr.Code != http.StatusUnsupportedMediaType || hits != 0 {
		t.Fatalf("type %d %s", rr.Code, rr.Body.Bytes())
	}
	if _, err := pins.ParseTarget(sampleV1); err != nil {
		t.Fatal(err)
	}
}

func TestPinsAddListAndCancel(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	var sawRecursive string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pin/ls"):
			if r.URL.Query().Get("names") == "true" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"Message":"unrecognized argument: \"names\""}`)
				return
			}
			if r.URL.Query().Get("type") == "direct" {
				_, _ = io.WriteString(w, `{"Keys":{}}`)
				return
			}
			_, _ = io.WriteString(w, `{"Keys":{"`+sampleV1+`":{"Type":"recursive"}}}`)
		case strings.HasSuffix(r.URL.Path, "/object/stat"):
			_, _ = io.WriteString(w, `{"CumulativeSize":9}`)
		case strings.HasSuffix(r.URL.Path, "/pin/add"):
			if r.URL.Query().Get("name") != "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"Message":"unused argument: \"name\""}`)
				return
			}
			mu.Lock()
			sawRecursive = r.URL.Query().Get("recursive")
			mu.Unlock()
			once.Do(func() { close(entered) })
			select {
			case <-release:
				_, _ = io.WriteString(w, `{"Progress":4}`+"\n"+`{"Pins":["`+sampleV1+`"]}`+"\n")
			case <-r.Context().Done():
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	mux := pinsMux(t, upPins(t, srv.URL))

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- do(mux, http.MethodPost, "/api/pins", "application/json", `{"target":"  `+sampleV1+`\n","name":"notes"}`)
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("pin add was not started")
	}
	var rr *httptest.ResponseRecorder
	select {
	case rr = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("add held the request open")
	}
	if rr.Code != http.StatusAccepted {
		t.Fatalf("add %d %s", rr.Code, rr.Body.Bytes())
	}
	if strings.Contains(rr.Body.String(), srv.URL) || strings.Contains(rr.Body.String(), "5001") {
		t.Fatalf("leaked %s", rr.Body.String())
	}
	var started struct {
		Job jobJSON `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.Job.State != "running" || started.Job.Name != "notes" || started.Job.CID != sampleV1 || started.Job.Type != "recursive" {
		t.Fatalf("%#v", started.Job)
	}
	close(release)
	waitPinJob(t, mux, started.Job.ID, "succeeded")

	rr = do(mux, http.MethodGet, "/api/pins", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list %d %s", rr.Code, rr.Body.Bytes())
	}
	if !strings.Contains(rr.Body.String(), `"namesFrom":"panel"`) || !strings.Contains(rr.Body.String(), `"name":"notes"`) || !strings.Contains(rr.Body.String(), `"size":9`) || !strings.Contains(rr.Body.String(), `"maxBytes":1024`) {
		t.Fatalf("list %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"localGateway":"http://127.0.0.1:8081"`) {
		t.Fatalf("gateway %s", rr.Body.String())
	}
	mu.Lock()
	if sawRecursive != "true" {
		t.Fatalf("recursive %q", sawRecursive)
	}
	mu.Unlock()

	blocked := make(chan struct{})
	blockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/pin/add") {
			close(blocked)
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer blockSrv.Close()
	blockMux := pinsMux(t, upPins(t, blockSrv.URL))
	rr = do(blockMux, http.MethodPost, "/api/pins", "application/json", `{"target":"`+sampleV0+`","recursive":false}`)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("direct %d %s", rr.Code, rr.Body.Bytes())
	}
	var direct struct {
		Job jobJSON `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &direct); err != nil {
		t.Fatal(err)
	}
	if direct.Job.Type != "direct" {
		t.Fatalf("%#v", direct.Job)
	}
	select {
	case <-blocked:
	case <-time.After(2 * time.Second):
		t.Fatal("direct pin was not started")
	}
	rr = do(blockMux, http.MethodPost, "/api/pins/jobs/"+direct.Job.ID+"/cancel", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"cancelled"`) {
		t.Fatalf("cancel %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(blockMux, http.MethodGet, "/api/pins/jobs/not-an-id", "", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("bad id %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestPinsUploadAndFromFiles(t *testing.T) {
	var uploaded string
	var chunked bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pin/add"):
			_, _ = io.WriteString(w, `{"Pins":["`+sampleV1+`"]}`)
		case strings.HasSuffix(r.URL.Path, "/add"):
			chunked = r.ContentLength < 0
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
			uploaded = string(buf)
			_, _ = io.WriteString(w, `{"Name":"notes.txt","Hash":"`+sampleV1+`","Size":"5"}`)
		case strings.HasSuffix(r.URL.Path, "/files/stat"):
			if r.URL.Query().Get("arg") != "/notes.txt" {
				t.Errorf("stat %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"Hash":"`+sampleV0+`","Size":5,"Type":"file"}`)
		case strings.HasSuffix(r.URL.Path, "/pin/ls"):
			_, _ = io.WriteString(w, `{"Keys":{}}`)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()
	mux := pinsMux(t, upPins(t, srv.URL))
	rr := do(mux, http.MethodPost, "/api/pins/upload?filename=notes.txt", "application/octet-stream", "hello")
	if rr.Code != http.StatusCreated || uploaded != "hello" || !chunked {
		t.Fatalf("upload %d chunked %v body %q resp %s", rr.Code, chunked, uploaded, rr.Body.Bytes())
	}
	if !strings.Contains(rr.Body.String(), sampleV1) || !strings.Contains(rr.Body.String(), `"name":"notes.txt"`) || !strings.Contains(rr.Body.String(), `"size":5`) {
		t.Fatalf("upload body %s", rr.Body.Bytes())
	}

	rr = do(mux, http.MethodPost, "/api/pins/from-files", "application/json", `{"path":"/notes.txt"}`)
	if rr.Code != http.StatusAccepted || !strings.Contains(rr.Body.String(), sampleV0) || !strings.Contains(rr.Body.String(), `"name":"notes.txt"`) {
		t.Fatalf("from files %d %s", rr.Code, rr.Body.Bytes())
	}
	var started struct {
		Job jobJSON `json:"job"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	waitPinJob(t, mux, started.Job.ID, "succeeded")

	rr = do(mux, http.MethodPost, "/api/pins/upload?filename=../x", "application/octet-stream", "hello")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("filename %d %s", rr.Code, rr.Body.Bytes())
	}

	limited := upPins(t, srv.URL)
	limited.maxBytes = 4
	rr = do(pinsMux(t, limited), http.MethodPost, "/api/pins/upload?filename=big.txt", "application/octet-stream", "hello")
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestPinsRemove(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/pin/rm") {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("arg") == sampleV0 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"Message":"not pinned or pinned indirectly"}`)
			return
		}
		if r.URL.Query().Get("recursive") != "true" {
			t.Errorf("recursive %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"Pins":["`+sampleV1+`"]}`)
	}))
	defer srv.Close()
	mux := pinsMux(t, upPins(t, srv.URL))
	rr := do(mux, http.MethodPost, "/api/pins/remove", "application/json", `{"cid":"`+sampleV1+`","type":"recursive"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), sampleV1) {
		t.Fatalf("rm %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/pins/remove", "application/json", `{"cid":"`+sampleV0+`","type":"recursive"}`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "indirect") {
		t.Fatalf("indirect %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/pins/remove", "application/json", `{"cid":"`+sampleV1+`"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("type %d %s", rr.Code, rr.Body.Bytes())
	}
}

func pinsMux(t *testing.T, h *pinsHTTP) http.Handler {
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
	return newMux(sup, svc, nil, h)
}

func upPins(t *testing.T, apiBase string) *pinsHTTP {
	t.Helper()
	return &pinsHTTP{
		dataDir:    t.TempDir(),
		maxBytes:   1024,
		op:         time.Second,
		transfer:   time.Second,
		pinTimeout: 2 * time.Second,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp, RepoPath: t.TempDir()}, nil
		},
		lookup: func(string) (string, string, error) {
			return apiBase, "http://127.0.0.1:8081", nil
		},
	}
}

func waitPinJob(t *testing.T, mux http.Handler, id, state string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rr := do(mux, http.MethodGet, "/api/pins/jobs/"+id, "", "")
		if rr.Code == http.StatusOK && strings.Contains(rr.Body.String(), `"state":"`+state+`"`) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	rr := do(mux, http.MethodGet, "/api/pins/jobs/"+id, "", "")
	t.Fatalf("job %s did not reach %s: %s", id, state, rr.Body.Bytes())
}
