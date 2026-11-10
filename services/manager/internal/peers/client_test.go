package peers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientListAndConnect(t *testing.T) {
	var connectArg string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v0/swarm/peers"):
			q := r.URL.Query()
			if q.Get("latency") != "true" || q.Get("direction") != "true" || q.Get("identify") != "true" {
				t.Errorf("query %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{
				"Peers": [{
					"Addr": "/ip4/203.0.113.10/tcp/4001",
					"Peer": "`+qmPeer+`",
					"Latency": "12ms",
					"Direction": 2,
					"Identify": {
						"PublicKey": "secret",
						"Addresses": ["/ip4/127.0.0.1/tcp/5001"],
						"AgentVersion": "kubo/0.43.0"
					}
				}]
			}`)
		case strings.HasPrefix(r.URL.Path, "/api/v0/swarm/connect"):
			connectArg = r.URL.Query().Get("arg")
			_, _ = io.WriteString(w, `{"Strings":["connect `+qmPeer+` success"]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client, err := New(Options{BaseURL: srv.URL, ListTimeout: time.Second, ConnectTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	list, err := client.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || list.Peers[0].PeerID != qmPeer || list.Peers[0].Agent != "kubo/0.43.0" {
		t.Fatalf("list %#v", list)
	}
	encoded, err := jsonPeers(list)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"/ip4/", "5001", "PublicKey", "secret", "Addr"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("leaked %s in %s", forbidden, encoded)
		}
	}

	id, err := client.Connect(context.Background(), "/ip4/203.0.113.10/tcp/4001/p2p/"+qmPeer)
	if err != nil || id != qmPeer {
		t.Fatalf("connect %q %v", id, err)
	}
	if connectArg != "/ip4/203.0.113.10/tcp/4001/p2p/"+qmPeer {
		t.Fatalf("arg %q", connectArg)
	}

	if _, err := client.Connect(context.Background(), "http://127.0.0.1:5001"); CodeOf(err) != "invalid_address" {
		t.Fatalf("url %v", err)
	}
}

func TestClientRefusesNonLoopbackAndClosedPort(t *testing.T) {
	if _, err := New(Options{BaseURL: "http://example.com:5001"}); err == nil {
		t.Fatal("public base accepted")
	}
	refused, err := New(Options{BaseURL: "http://127.0.0.1:1", ListTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := refused.List(context.Background()); CodeOf(err) != "node_not_running" {
		t.Fatalf("refused %v", err)
	}
}

func TestClientMapsKuboFailureAndSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "connect") {
			http.Error(w, `{"Message":"dial failed /ip4/203.0.113.10/tcp/4001"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytesRepeat(maxListBody + 2))
	}))
	defer srv.Close()
	client, err := New(Options{BaseURL: srv.URL, ListTimeout: time.Second, ConnectTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.List(context.Background()); CodeOf(err) != "too_large" {
		t.Fatalf("size %v", err)
	}
	if _, err := client.Connect(context.Background(), "/ip4/203.0.113.10/tcp/4001/p2p/"+qmPeer); CodeOf(err) != "connect_failed" {
		t.Fatalf("dial %v", err)
	}
}

func jsonPeers(list List) (string, error) {
	// Encode the same shape the HTTP layer will, so a field added by mistake fails here.
	type row struct {
		PeerID    string `json:"peerId"`
		Direction string `json:"direction,omitempty"`
		LatencyMs *int   `json:"latencyMs,omitempty"`
		Agent     string `json:"agent,omitempty"`
	}
	out := struct {
		Total int   `json:"total"`
		Peers []row `json:"peers"`
	}{Total: list.Total, Peers: make([]row, 0, len(list.Peers))}
	for _, peer := range list.Peers {
		out.Peers = append(out.Peers, row(peer))
	}
	buf, err := json.Marshal(out)
	return string(buf), err
}

func bytesRepeat(n int) []byte {
	return []byte(strings.Repeat("x", n))
}
