package peering

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peers"
)

const (
	// MaxPeers is the panel cap on saved peers. Kubo has no such cap.
	MaxPeers = 64
	// MaxAddrs is the panel cap on saved addresses for one peer.
	MaxAddrs = 8

	// SyncOK means the running peering service matches the saved list.
	SyncOK = "ok"
	// SyncApply means the saved list can be pushed to the running service.
	SyncApply = "apply"
	// SyncRestart means the save landed and the live call did not.
	SyncRestart = "restart"

	// StateConnected means the peer id has an open swarm connection.
	StateConnected = "connected"
	// StateReconnecting means the peering service has the id and the swarm does not.
	StateReconnecting = "reconnecting"
	// StatePending means the id is saved and the peering service does not have it.
	StatePending = "pending"
)

// Peer is one saved peer, as the page shows it.
type Peer struct {
	PeerID string
	Addrs  []string
	State  string
}

// View is the saved list plus whether the running service matches it.
type View struct {
	Peers []Peer
	Sync  string
}

// Saved is a peer id and the addresses stored for it.
type Saved struct {
	PeerID string
	Addrs  []string
}

type entry struct {
	raw    map[string]any
	peerID string
	addrs  []string
	ok     bool
}

func readDoc(text string) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, code(http.StatusBadGateway, "kubo_error")
	}
	doc, ok := value.(map[string]any)
	if !ok || doc == nil {
		return nil, code(http.StatusBadGateway, "kubo_error")
	}
	return doc, nil
}

func writeDoc(doc map[string]any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return "", code(http.StatusServiceUnavailable, "unavailable")
	}
	return buf.String(), nil
}

func peeringEntries(doc map[string]any) (map[string]any, []any, []entry) {
	peering, _ := doc["Peering"].(map[string]any)
	if peering == nil {
		peering = map[string]any{}
		doc["Peering"] = peering
	}
	raw, _ := peering["Peers"].([]any)
	if raw == nil {
		raw = []any{}
	}
	out := make([]entry, 0, len(raw))
	for _, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok || obj == nil {
			out = append(out, entry{})
			continue
		}
		id, _ := obj["ID"].(string)
		canon, ok := peers.CanonicalPeerID(id)
		if !ok {
			out = append(out, entry{raw: obj})
			continue
		}
		out = append(out, entry{raw: obj, peerID: canon, addrs: addrList(obj["Addrs"]), ok: true})
	}
	return peering, raw, out
}

func addrList(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if _, dup := seen[text]; dup {
			continue
		}
		seen[text] = struct{}{}
		out = append(out, text)
	}
	return out
}

func validEntries(entries []entry) []Saved {
	out := make([]Saved, 0, len(entries))
	for _, item := range entries {
		if !item.ok {
			continue
		}
		out = append(out, Saved{PeerID: item.peerID, Addrs: append([]string(nil), item.addrs...)})
	}
	return out
}

func findEntry(entries []entry, id string) int {
	for i, item := range entries {
		if item.ok && item.peerID == id {
			return i
		}
	}
	return -1
}

// splitAddress returns the stored address (without the final peer id) and the
// canonical peer id. A bare /p2p/<id> stores an empty address.
func splitAddress(raw string) (string, string, error) {
	full, id, err := peers.ParseAddress(raw)
	if err != nil {
		return "", "", code(http.StatusBadRequest, "invalid_address")
	}
	for _, suffix := range []string{"/p2p/" + id, "/ipfs/" + id} {
		if strings.HasSuffix(full, suffix) {
			return strings.TrimSuffix(full, suffix), id, nil
		}
	}
	return "", "", code(http.StatusBadRequest, "invalid_address")
}

func upsert(doc map[string]any, id, addr string) (bool, error) {
	peering, raw, entries := peeringEntries(doc)
	idx := findEntry(entries, id)
	if idx < 0 && countValid(entries) >= MaxPeers {
		return false, code(http.StatusConflict, "too_many")
	}
	if idx >= 0 && addr != "" && !containsAddr(entries[idx].addrs, addr) && len(entries[idx].addrs) >= MaxAddrs {
		return false, code(http.StatusConflict, "too_many")
	}
	changed := false
	if idx < 0 {
		obj := map[string]any{"ID": id, "Addrs": addrJSON(oneAddr(addr))}
		raw = append(raw, obj)
		changed = true
	} else if addr != "" && !containsAddr(entries[idx].addrs, addr) {
		next := append(append([]string(nil), entries[idx].addrs...), addr)
		entries[idx].raw["ID"] = id
		entries[idx].raw["Addrs"] = addrJSON(next)
		raw[idx] = entries[idx].raw
		changed = true
	} else if entries[idx].raw["ID"] != id {
		entries[idx].raw["ID"] = id
		raw[idx] = entries[idx].raw
		changed = true
	}
	if changed {
		peering["Peers"] = raw
	}
	return changed, nil
}

func removeID(doc map[string]any, id string) bool {
	peering, raw, entries := peeringEntries(doc)
	next := make([]any, 0, len(raw))
	changed := false
	for i, item := range entries {
		if item.ok && item.peerID == id {
			changed = true
			continue
		}
		if i < len(raw) {
			next = append(next, raw[i])
		}
	}
	if !changed {
		return false
	}
	peering["Peers"] = next
	return true
}

func countValid(entries []entry) int {
	n := 0
	for _, item := range entries {
		if item.ok {
			n++
		}
	}
	return n
}

func containsAddr(addrs []string, addr string) bool {
	for _, item := range addrs {
		if item == addr {
			return true
		}
	}
	return false
}

func oneAddr(addr string) []string {
	if addr == "" {
		return nil
	}
	return []string{addr}
}

func addrJSON(addrs []string) []any {
	out := make([]any, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr)
	}
	return out
}

func sameSet(saved, live []Saved) bool {
	if len(saved) != len(live) {
		return false
	}
	left := indexSaved(saved)
	right := indexSaved(live)
	if len(left) != len(right) {
		return false
	}
	for id, addrs := range left {
		if !sameAddrs(addrs, right[id]) {
			return false
		}
	}
	return true
}

func indexSaved(list []Saved) map[string][]string {
	out := make(map[string][]string, len(list))
	for _, item := range list {
		id, ok := peers.CanonicalPeerID(item.PeerID)
		if !ok {
			id = item.PeerID
		}
		out[id] = append(out[id], item.Addrs...)
	}
	return out
}

func sameAddrs(left, right []string) bool {
	a := uniqueSorted(left)
	b := uniqueSorted(right)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func uniqueSorted(addrs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
	}
	sort.Strings(out)
	return out
}

func dialAddrs(id string, addrs []string) []string {
	if len(addrs) == 0 {
		return []string{"/p2p/" + id}
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		full := strings.TrimRight(addr, "/") + "/p2p/" + id
		if checked, _, err := peers.ParseAddress(full); err == nil {
			out = append(out, checked)
			continue
		}
		if checked, _, err := peers.ParseAddress(addr); err == nil {
			out = append(out, checked)
		}
	}
	if len(out) == 0 {
		return []string{"/p2p/" + id}
	}
	return out
}

func project(saved []Saved, live []Saved, connected map[string]struct{}, sync string) View {
	liveSet := indexSaved(live)
	rows := make([]Peer, 0, len(saved))
	for _, item := range saved {
		state := StatePending
		if _, ok := connected[item.PeerID]; ok {
			state = StateConnected
		} else if _, ok := liveSet[item.PeerID]; ok {
			state = StateReconnecting
		}
		addrs := append([]string(nil), item.Addrs...)
		if addrs == nil {
			addrs = []string{}
		}
		rows = append(rows, Peer{PeerID: item.PeerID, Addrs: addrs, State: state})
	}
	if sync == "" {
		if sameSet(saved, live) {
			sync = SyncOK
		} else {
			sync = SyncApply
		}
	}
	return View{Peers: rows, Sync: sync}
}
