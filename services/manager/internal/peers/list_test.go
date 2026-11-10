package peers

import (
	"strings"
	"testing"
)

func TestParseListCollapsesAndDropsAddresses(t *testing.T) {
	body := []byte(`{
		"Peers": [
			{
				"Addr": "/ip4/203.0.113.10/tcp/4001",
				"Peer": "bbbb",
				"Latency": "20ms",
				"Direction": 1,
				"Muxer": "yamux",
				"Streams": [{"Protocol": "/ipfs/id/1.0.0"}],
				"Identify": {
					"ID": "bbbb",
					"PublicKey": "c2VjcmV0",
					"Addresses": ["/ip4/203.0.113.10/tcp/4001/p2p/bbbb", "/ip4/127.0.0.1/tcp/5001"],
					"AgentVersion": "kubo/0.43.0\n",
					"Protocols": ["/ipfs/id/1.0.0"]
				}
			},
			{
				"Addr": "/ip4/203.0.113.11/udp/4001/quic-v1",
				"Peer": "bbbb",
				"Latency": "5ms",
				"Direction": 2,
				"Identify": {"AgentVersion": "other"}
			},
			{
				"Addr": "/ip4/198.51.100.8/tcp/4001",
				"Peer": "aaaa",
				"Latency": "n/a",
				"Direction": 2,
				"Identify": {"AgentVersion": "  "}
			},
			{
				"Peer": "cccc",
				"Latency": "400µs",
				"Direction": 9,
				"Identify": {"AgentVersion": "` + strings.Repeat("x", 140) + `"}
			},
			{"Peer": "  ", "Addr": "/ip4/192.0.2.1/tcp/4001"}
		]
	}`)
	list, err := ParseList(body)
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 3 || list.Truncated || len(list.Peers) != 3 {
		t.Fatalf("list %#v", list)
	}
	// Known latency first: 0ms (sub-ms), then 5ms, then unknown.
	if list.Peers[0].PeerID != "cccc" || list.Peers[0].LatencyMs == nil || *list.Peers[0].LatencyMs != 0 || list.Peers[0].Direction != "" {
		t.Fatalf("sub-ms %#v", list.Peers[0])
	}
	if len(list.Peers[0].Agent) != 128 {
		t.Fatalf("agent len %d", len(list.Peers[0].Agent))
	}
	got := list.Peers[1]
	if got.PeerID != "bbbb" || got.Direction != "both" || got.LatencyMs == nil || *got.LatencyMs != 5 || got.Agent != "kubo/0.43.0" {
		t.Fatalf("merged %#v", got)
	}
	if list.Peers[2].PeerID != "aaaa" || list.Peers[2].LatencyMs != nil || list.Peers[2].Agent != "" || list.Peers[2].Direction != "out" {
		t.Fatalf("unknown %#v", list.Peers[2])
	}
}

func TestParseListCapsAtFiveHundred(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"Peers":[`)
	for i := 0; i < MaxRows+1; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"Peer":"peer-`)
		b.WriteString(pad(i))
		b.WriteString(`","Latency":"`)
		b.WriteString(itoa(i + 1))
		b.WriteString(`ms","Direction":1}`)
	}
	b.WriteString(`]}`)
	list, err := ParseList([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if !list.Truncated || list.Total != MaxRows+1 || len(list.Peers) != MaxRows {
		t.Fatalf("cap total=%d rows=%d truncated=%v", list.Total, len(list.Peers), list.Truncated)
	}
	if list.Peers[0].PeerID != "peer-"+pad(0) || list.Peers[MaxRows-1].PeerID != "peer-"+pad(MaxRows-1) {
		t.Fatalf("order %s .. %s", list.Peers[0].PeerID, list.Peers[MaxRows-1].PeerID)
	}
}

func TestParseListEmpty(t *testing.T) {
	list, err := ParseList([]byte(`{}`))
	if err != nil || list.Total != 0 || list.Peers == nil || len(list.Peers) != 0 {
		t.Fatalf("%#v %v", list, err)
	}
	if _, err := ParseList([]byte(`not-json`)); CodeOf(err) != "kubo_error" {
		t.Fatal(err)
	}
}

func pad(n int) string {
	s := itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
