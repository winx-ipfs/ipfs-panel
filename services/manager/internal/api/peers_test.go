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

const peersQM = "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"

func TestPeersRequireSessionAndCSRF(t *testing.T) {
	mux := peersMux(t, &peersHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("handler ran without a session")
			return node.Overview{}, nil
		},
	})
	useSession("", "")
	rr := do(mux, http.MethodGet, "/api/peers", "", "")
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), `"unauthorized"`) {
		t.Fatalf("anon %d %s", rr.Code, rr.Body.Bytes())
	}

	svcMux := peersMux(t, &peersHTTP{
		status: func(context.Context) (node.Overview, error) {
			t.Fatal("csrf should stop the handler")
			return node.Overview{}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/peers/connect", strings.NewReader(`{"address":"/p2p/`+peersQM+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionCookie})
	rr = httptest.NewRecorder()
	svcMux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"csrf"`) {
		t.Fatalf("csrf %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestPeersStoppedNode(t *testing.T) {
	mux := peersMux(t, &peersHTTP{
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateDown}, nil
		},
	})
	rr := do(mux, http.MethodGet, "/api/peers", "", "")
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) || !strings.Contains(rr.Body.String(), `"down"`) {
		t.Fatalf("list %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/peers/connect", "application/json", `{"address":"/p2p/`+peersQM+`"}`)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"node_not_running"`) {
		t.Fatalf("connect %d %s", rr.Code, rr.Body.Bytes())
	}
}

func TestPeersListHidesAddressesAndConnectReturnsPeerID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/v0/swarm/peers"):
			_, _ = io.WriteString(w, `{
				"Peers": [{
					"Addr": "/ip4/203.0.113.10/tcp/4001",
					"Peer": "`+peersQM+`",
					"Latency": "18ms",
					"Direction": 2,
					"Identify": {
						"PublicKey": "secret-key",
						"Addresses": ["/ip4/127.0.0.1/tcp/5001/p2p/`+peersQM+`"],
						"AgentVersion": "kubo/0.43.0"
					}
				}]
			}`)
		case strings.Contains(r.URL.Path, "/api/v0/swarm/connect"):
			if r.URL.Query().Get("arg") != "/ip4/203.0.113.10/tcp/4001/p2p/"+peersQM {
				t.Errorf("arg %q", r.URL.Query().Get("arg"))
			}
			_, _ = io.WriteString(w, `{"Strings":["connect `+peersQM+` success /ip4/203.0.113.10/tcp/4001"]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	mux := peersMux(t, upPeers(t, srv.URL))

	rr := do(mux, http.MethodGet, "/api/peers", "", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list %d %s", rr.Code, rr.Body.Bytes())
	}
	body := rr.Body.String()
	for _, forbidden := range []string{"/ip4/", "5001", "PublicKey", "secret-key", "Addr", "Strings", srv.URL} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("leaked %s in %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"peerId":"`+peersQM+`"`) || !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"agent":"kubo/0.43.0"`) {
		t.Fatalf("body %s", body)
	}

	rr = do(mux, http.MethodPost, "/api/peers/connect", "application/json", `{"address":" /ip4/203.0.113.10/tcp/4001/p2p/`+peersQM+` "}`)
	if rr.Code != http.StatusOK || strings.TrimSpace(rr.Body.String()) != `{"peerId":"`+peersQM+`"}` {
		t.Fatalf("connect %d %s", rr.Code, rr.Body.Bytes())
	}
	for _, forbidden := range []string{"/ip4/", "5001", "Strings", "success"} {
		if strings.Contains(rr.Body.String(), forbidden) {
			t.Fatalf("connect leaked %s in %s", forbidden, rr.Body.String())
		}
	}

	rr = do(mux, http.MethodPost, "/api/peers/connect", "application/json", `{"address":"http://127.0.0.1:5001"}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"invalid_address"`) {
		t.Fatalf("bad address %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/peers/connect", "application/json", `{"address":"/p2p/`+peersQM+`","save":true}`)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"bad_request"`) {
		t.Fatalf("extra %d %s", rr.Code, rr.Body.Bytes())
	}
	rr = do(mux, http.MethodPost, "/api/peers/connect", "text/plain", `{"address":"/p2p/`+peersQM+`"}`)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("type %d %s", rr.Code, rr.Body.Bytes())
	}
}

func peersMux(t *testing.T, h *peersHTTP) http.Handler {
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
	return newMux(sup, svc, nil, nil, nil, h)
}

func upPeers(t *testing.T, apiBase string) *peersHTTP {
	t.Helper()
	return &peersHTTP{
		listTimeout:    time.Second,
		connectTimeout: time.Second,
		status: func(context.Context) (node.Overview, error) {
			return node.Overview{State: node.StateUp, RepoPath: t.TempDir()}, nil
		},
		lookup: func(string) (string, string, error) {
			return apiBase, "", nil
		},
	}
}
