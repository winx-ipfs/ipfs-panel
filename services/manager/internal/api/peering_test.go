package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peering"
)

func TestPeeringRequiresSessionAndCSRF(t *testing.T) {
	mux := peeringMux(t, &peeringHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("handler ran without a session")
			return node.Overview{}, nil
		},
	})
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/peers/permanent", "", "")
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), `"unauthorized"`) {
		t.Fatalf("anon %d %s", rr.Code, rr.Body.Bytes())
	}

	svcMux := peeringMux(t, &peeringHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("csrf should stop the handler")
			return node.Overview{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/peers/permanent", strings.NewReader(`{"peerId":"`+peersQM+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	rr = httptest.NewRecorder()
	svcMux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"csrf"`) {
		t.Fatalf("csrf %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestPeeringStoppedNode(t *testing.T) {
	mux := peeringMux(t, &peeringHTTP{
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateDown}, nil
		},
	})
	rr := do(mux, http.MethodGet, "/api/peers/permanent", "", "")
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) || !strings.Contains(rr.Body.String(), `"down"`) {
		t.Fatalf("list %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/peers/permanent", "application/json", `{"peerId":"`+peersQM+`"}`)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) {
		t.Fatalf("add %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestPeeringAddAuditsThePeerIDOnly(t *testing.T) {
	mem := &peeringMem{doc: []byte(`{
	  "Identity": {"PeerID": "12D3KooWexample", "PrivKey": "SECRET-PRIVKEY-VALUE"},
	  "Addresses": {"API": "/ip4/127.0.0.1/tcp/5001", "Gateway": ["/ip4/127.0.0.1/tcp/8081"]}
	}`)}
	rt := &peeringRT{}
	svc := peering.New(configedit.New(mem, t.TempDir(), t.TempDir(), "127.0.0.1:8080"), rt)
	var buf bytes.Buffer
	mux := peeringMux(t, &peeringHTTP{
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp}, nil
		},
		svc: svc,
		log: slog.New(slog.NewTextHandler(&buf, nil)),
	})
	body := `{"address":" /ip4/203.0.113.10/tcp/4001/p2p/` + peersQM + ` "}`
	rr := do(mux, http.MethodPost, "/api/peers/permanent", "application/json", body)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"sync":"ok"`) || !strings.Contains(rr.Body.String(), peersQM) {
		t.Fatalf("add %d %s", rr.Code, rr.Body.Bytes())
	}
	if strings.Contains(rr.Body.String(), "SECRET-PRIVKEY-VALUE") || strings.Contains(rr.Body.String(), "5001") {
		t.Fatalf("response leaked %s", rr.Body.Bytes())
	}
	if !strings.Contains(rr.Body.String(), "/ip4/203.0.113.10/tcp/4001") {
		t.Fatalf("saved address missing %s", rr.Body.Bytes())
	}
	audit := buf.String()
	if !strings.Contains(audit, "peering_add") || !strings.Contains(audit, peersQM) || strings.Contains(audit, "203.0.113.10") || strings.Contains(audit, "SECRET") {
		t.Fatalf("audit %s", audit)
	}
	rr = do(mux, http.MethodPost, "/api/peers/permanent", "application/json", `{"address":"http://127.0.0.1:5001"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"invalid_address"`) {
		t.Fatalf("bad %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/peers/permanent", "application/json", `{"address":"/p2p/`+peersQM+`","save":true}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"bad_request"`) {
		t.Fatalf("extra %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodGet, "/api/peers/permanent", "", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"reconnecting"`) {
		t.Fatalf("list %d %s", rr.Code, rr.Body.Bytes())
	}
}

type peeringMem struct {
	doc []byte
}

func (m *peeringMem) Show(context.Context) ([]byte, error) {
	return append([]byte(nil), m.doc...), nil
}

func (m *peeringMem) Replace(_ context.Context, doc []byte) error {
	m.doc = append([]byte(nil), doc...)
	return nil
}

type peeringRT struct {
	live []peering.Saved
}

func (f *peeringRT) List(context.Context) ([]peering.Saved, error) {
	return append([]peering.Saved(nil), f.live...), nil
}

func (f *peeringRT) Add(_ context.Context, addrs []string) error {
	if len(addrs) == 0 {
		return nil
	}
	id := addrs[0][strings.LastIndex(addrs[0], "/")+1:]
	bare := strings.TrimSuffix(addrs[0], "/p2p/"+id)
	f.live = []peering.Saved{{PeerID: id, Addrs: []string{bare}}}
	return nil
}

func (f *peeringRT) Remove(context.Context, string) error { return nil }

func (f *peeringRT) Swarm(context.Context) (map[string]struct{}, map[string][]string, error) {
	return map[string]struct{}{}, map[string][]string{}, nil
}

func peeringMux(t *testing.T, h *peeringHTTP) http.Handler {
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
	return newMux(parts{sup: sup, auth: svc, peering: h})
}
