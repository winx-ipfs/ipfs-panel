package activity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchCountsPeersAndHidesAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		switch r.URL.Path {
		case "/api/v0/swarm/peers":
			_, _ = w.Write([]byte(`{"Peers":[` +
				`{"Addr":"/ip4/203.0.113.9/tcp/4001","Peer":"peer-a"},` +
				`{"Addr":"/ip4/203.0.113.9/tcp/4002","Peer":"peer-a"},` +
				`{"Addr":"/ip4/198.51.100.4/tcp/4001","Peer":"peer-b"}` +
				`]}`))
		case "/api/v0/stats/bw":
			_, _ = w.Write([]byte(`{"TotalIn":10,"TotalOut":4,"RateIn":1.5,"RateOut":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	act, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if act.Peers == nil || *act.Peers != 2 {
		t.Fatalf("peers %#v", act.Peers)
	}
	if act.RateIn == nil || *act.RateIn != 1.5 || act.RateOut == nil || *act.RateOut != 0 {
		t.Fatalf("rates %#v", act)
	}
	raw, _ := json.Marshal(act)
	if strings.Contains(string(raw), "203.0.113.9") || strings.Contains(string(raw), "tcp") || strings.Contains(string(raw), "TotalIn") {
		t.Fatalf("leaked %s", raw)
	}
}

func TestBandwidthDisabledKeepsPeers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/stats/bw" {
			http.Error(w, "bandwidth reporter disabled in config", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"Peers":[{"Peer":"only"}]}`))
	}))
	defer srv.Close()
	act, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if act.Peers == nil || *act.Peers != 1 || act.RateIn != nil || act.RateOut != nil {
		t.Fatalf("%#v", act)
	}
}

func TestRefusesNonLoopback(t *testing.T) {
	if _, err := Fetch(context.Background(), "http://203.0.113.9:5001"); err == nil {
		t.Fatal("accepted public api")
	}
}
