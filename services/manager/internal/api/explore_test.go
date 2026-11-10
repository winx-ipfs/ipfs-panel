package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func TestExploreRequiresSessionAndRunningNode(t *testing.T) {
	mux := exploreMux(t, upExplore(t, "http://127.0.0.1:9"))
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/explore?target=bafkqac3imvwgy3zao5xxe3de", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("anon %d %s", rr.Code, rr.Body.Bytes())
	}

	stopped := exploreMux(t, &exploreHTTP{
		timeout: time.Second,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateDown}, nil
		},
	})
	rr = do(stopped, http.MethodGet, "/api/explore?target=bafkqac3imvwgy3zao5xxe3de", "", "")
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) || !strings.Contains(rr.Body.String(), `"down"`) {
		t.Fatalf("down %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestExploreLookup(t *testing.T) {
	const cid = "bafkqac3imvwgy3zao5xxe3de"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "missing") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"Message":"merkledag: not found"}`)
			return
		}
		_, _ = io.WriteString(w, `{"Data":{"/":{"bytes":"CAE"}},"Links":[{"Hash":{"/":"`+cid+`"},"Name":"notes.txt","Tsize":5}]}`)
	}))
	defer srv.Close()
	mux := exploreMux(t, upExplore(t, srv.URL))

	rr := do(mux, http.MethodGet, "/api/explore?target="+cid, "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d %s", rr.Code, rr.Body.Bytes())
	}
	body := rr.Body.String()
	for _, forbidden := range []string{"5001", "/api/v0", "multiaddr", "0.0.0.0"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("leaked %s in %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"kind":"bytes"`) || !strings.Contains(body, `"localGateway":"http://127.0.0.1:8081"`) {
		t.Fatalf("body %s", body)
	}
	// The fixture CID is raw, so a dag-pb body is not treated as a folder.
	// A dag-pb response is only a folder when the CID says dag-pb. This test
	// checks the HTTP shell: session, no RPC address, and the local link field.

	rr = do(mux, http.MethodGet, "/api/explore?target=hello", "", "")
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"invalid_target"`) {
		t.Fatalf("bad %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodGet, "/api/explore?target=missing", "", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing target %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestExploreNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"Message":"merkledag: not found"}`)
	}))
	defer srv.Close()
	mux := exploreMux(t, upExplore(t, srv.URL))
	rr := do(mux, http.MethodGet, "/api/explore?target=bafkqac3imvwgy3zao5xxe3de", "", "")
	if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), `"not_found"`) {
		t.Fatalf("not found %d %s", rr.Code, rr.Body.Bytes())
	}
}

func exploreMux(t *testing.T, h *exploreHTTP) http.Handler {
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
	return newMux(sup, svc, nil, nil, h, nil)
}

func upExplore(t *testing.T, apiBase string) *exploreHTTP {
	t.Helper()
	return &exploreHTTP{
		timeout: time.Second,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp, RepoPath: t.TempDir()}, nil
		},
		lookup: func(string) (string, string, error) {
			return apiBase, "http://127.0.0.1:8081", nil
		},
	}
}
