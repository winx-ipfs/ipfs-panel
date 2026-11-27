package peering

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientParsesPeeringAndRefusesAClosedPort(t *testing.T) {
	var added string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/swarm/peering/ls"):
			_, _ = io.WriteString(w, `{"Peers":[{"ID":"`+listPeer+`","Addrs":["/ip4/203.0.113.10/tcp/4001"]}]}`)
		case strings.HasSuffix(r.URL.Path, "/swarm/peering/add"):
			added = r.URL.Query().Get("arg")
			_, _ = io.WriteString(w, `{"ID":"`+listPeer+`","Status":"success"}`)
		case strings.HasSuffix(r.URL.Path, "/swarm/peers"):
			_, _ = io.WriteString(w, `{"Peers":[{"Addr":"/ip4/203.0.113.10/tcp/4001","Peer":"`+listPeer+`","Identify":{"PublicKey":"secret"}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	live, err := client.List(context.Background())
	if err != nil || len(live) != 1 || live[0].PeerID != listPeer {
		t.Fatalf("%#v %v", live, err)
	}
	if err := client.Add(context.Background(), []string{"/ip4/203.0.113.10/tcp/4001/p2p/" + listPeer}); err != nil {
		t.Fatal(err)
	}
	if added != "/ip4/203.0.113.10/tcp/4001/p2p/"+listPeer {
		t.Fatalf("arg %s", added)
	}
	connected, addrs, err := client.Swarm(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := connected[listPeer]; !ok || addrs[listPeer][0] != "/ip4/203.0.113.10/tcp/4001" {
		t.Fatalf("%v %v", connected, addrs)
	}
	if _, err := NewClient("http://203.0.113.10:5001"); err == nil {
		t.Fatal("non-loopback was accepted")
	}
	refused, err := NewClient("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := refused.List(ctx); CodeOf(err) != "node_not_running" && CodeOf(err) != "timeout" && CodeOf(err) != "unavailable" {
		t.Fatalf("%v", err)
	}
}
