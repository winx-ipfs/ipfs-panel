package peering

import "testing"

const listPeer = "QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN"

func TestUpsertAndRemove(t *testing.T) {
	doc, err := readDoc(`{"Identity":{"PeerID":"self"},"Peering":{"Peers":[]}}`)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := upsert(doc, listPeer, "/ip4/203.0.113.10/tcp/4001")
	if err != nil || !changed {
		t.Fatalf("changed %v err %v", changed, err)
	}
	again, err := upsert(doc, listPeer, "/ip4/203.0.113.10/tcp/4001")
	if err != nil || again {
		t.Fatalf("duplicate changed %v err %v", again, err)
	}
	if _, err := upsert(doc, listPeer, "/ip4/203.0.113.11/tcp/4001"); err != nil {
		t.Fatal(err)
	}
	_, _, entries := peeringEntries(doc)
	idx := findEntry(entries, listPeer)
	if idx < 0 || len(entries[idx].addrs) != 2 {
		t.Fatalf("entries %#v", entries)
	}
	if !removeID(doc, listPeer) {
		t.Fatal("remove")
	}
	_, _, entries = peeringEntries(doc)
	if findEntry(entries, listPeer) >= 0 {
		t.Fatal("peer remained")
	}
}

func TestUpsertAddrCap(t *testing.T) {
	doc, err := readDoc(`{"Peering":{"Peers":[]}}`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= MaxAddrs; i++ {
		if _, err := upsert(doc, listPeer, "/ip4/203.0.113.10/tcp/"+itoa(4000+i)); err != nil {
			t.Fatalf("addr %d: %v", i, err)
		}
	}
	if _, err := upsert(doc, listPeer, "/ip4/203.0.113.10/tcp/4999"); CodeOf(err) != "too_many" {
		t.Fatalf("%v", err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestSplitAddress(t *testing.T) {
	addr, id, err := splitAddress(" /ip4/203.0.113.10/tcp/4001/p2p/" + listPeer + " ")
	if err != nil || id != listPeer || addr != "/ip4/203.0.113.10/tcp/4001" {
		t.Fatalf("%s %s %v", addr, id, err)
	}
	addr, id, err = splitAddress("/p2p/" + listPeer)
	if err != nil || id != listPeer || addr != "" {
		t.Fatalf("bare %q %s %v", addr, id, err)
	}
	if _, _, err := splitAddress("http://127.0.0.1:5001"); CodeOf(err) != "invalid_address" {
		t.Fatalf("%v", err)
	}
}

func TestSyncDiff(t *testing.T) {
	saved := []Saved{{PeerID: listPeer, Addrs: []string{"/ip4/203.0.113.10/tcp/4001"}}}
	live := []Saved{{PeerID: listPeer, Addrs: []string{"/ip4/203.0.113.11/tcp/4001"}}}
	view := project(saved, live, map[string]struct{}{}, "")
	if view.Sync != SyncApply || view.Peers[0].State != StateReconnecting {
		t.Fatalf("%#v", view)
	}
	connected := map[string]struct{}{listPeer: {}}
	view = project(saved, saved, connected, "")
	if view.Sync != SyncOK || view.Peers[0].State != StateConnected {
		t.Fatalf("%#v", view)
	}
	view = project(saved, nil, nil, "")
	if view.Sync != SyncApply || view.Peers[0].State != StatePending || view.Peers[0].Addrs[0] == "" {
		t.Fatalf("%#v", view)
	}
}
