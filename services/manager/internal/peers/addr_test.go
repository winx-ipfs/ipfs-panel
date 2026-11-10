package peers

import "testing"

const (
	qmPeer = "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"
	edPeer = "12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d"
	cidKey = "f0172000101"
)

func TestParseAddressAcceptsDialableShapes(t *testing.T) {
	cases := []struct {
		in   string
		addr string
		peer string
	}{
		{
			in:   "  /ip4/203.0.113.10/tcp/4001/p2p/" + qmPeer + " \n",
			addr: "/ip4/203.0.113.10/tcp/4001/p2p/" + qmPeer,
			peer: qmPeer,
		},
		{
			in:   "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/" + edPeer,
			addr: "/ip4/203.0.113.10/udp/4001/quic-v1/p2p/" + edPeer,
			peer: edPeer,
		},
		{
			in:   "/dnsaddr/bootstrap.libp2p.io/p2p/" + qmPeer,
			addr: "/dnsaddr/bootstrap.libp2p.io/p2p/" + qmPeer,
			peer: qmPeer,
		},
		{
			in:   "/ip4/203.0.113.10/tcp/4001/p2p/" + qmPeer + "/p2p-circuit/p2p/" + edPeer,
			addr: "/ip4/203.0.113.10/tcp/4001/p2p/" + qmPeer + "/p2p-circuit/p2p/" + edPeer,
			peer: edPeer,
		},
		{
			in:   "/ip6/2001:db8::1/tcp/4001/ws/p2p/" + qmPeer,
			addr: "/ip6/2001:db8::1/tcp/4001/ws/p2p/" + qmPeer,
			peer: qmPeer,
		},
		{
			in:   "/p2p/" + qmPeer,
			addr: "/p2p/" + qmPeer,
			peer: qmPeer,
		},
		{
			in:   "/ip4/203.0.113.10/tcp/443/wss/p2p/F0172000101",
			addr: "/ip4/203.0.113.10/tcp/443/wss/p2p/" + cidKey,
			peer: cidKey,
		},
	}
	for _, tc := range cases {
		addr, peer, err := ParseAddress(tc.in)
		if err != nil || addr != tc.addr || peer != tc.peer {
			t.Fatalf("in %q -> %q %q %v", tc.in, addr, peer, err)
		}
	}
}

func TestParseAddressRejects(t *testing.T) {
	rejected := []string{
		"",
		"   ",
		"http://127.0.0.1:5001",
		qmPeer,
		"/ip4/203.0.113.10/tcp/4001",
		"/ip4/203.0.113.10/tcp/0/p2p/" + qmPeer,
		"/ip4/203.0.113.10/tcp/080/p2p/" + qmPeer,
		"/ip4/203.0.113.10/tcp/65536/p2p/" + qmPeer,
		"/ip4/999.0.0.1/tcp/4001/p2p/" + qmPeer,
		"/dnsaddr/bootstrap.libp2p.io",
		"/ip4/203.0.113.10/tcp/4001/p2p/not-an-id",
		"/ip4/203.0.113.10/tcp/4001/p2p/" + qmPeer + "?x=1",
		"/unix/tmp/ipfs.sock/p2p/" + qmPeer,
		"/ip4/203.0.113.10/tcp/4001/p2p/" + qmPeer + "/extra",
	}
	for _, in := range rejected {
		if _, _, err := ParseAddress(in); CodeOf(err) != "invalid_address" {
			t.Fatalf("accepted %q (%v)", in, err)
		}
	}
}
