package peers

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	// MaxRows is the most rows one response returns. Total still counts every
	// distinct peer id.
	MaxRows = 500

	maxAgentRunes = 128
)

// Peer is one connected peer. Fields that are not known stay at their zero
// value and are omitted from JSON by the HTTP layer.
type Peer struct {
	PeerID    string
	Direction string
	LatencyMs *int
	Agent     string
}

// List is the distinct set of connected peers.
type List struct {
	Total     int
	Truncated bool
	Peers     []Peer
}

type rawConn struct {
	Peer      string `json:"Peer"`
	Latency   string `json:"Latency"`
	Direction *int   `json:"Direction"`
	Identify  struct {
		AgentVersion string `json:"AgentVersion"`
	} `json:"Identify"`
}

// ParseList collapses Kubo's swarm/peers JSON to one row per peer id.
// Addresses, keys, and stream names in the body are ignored.
func ParseList(body []byte) (List, error) {
	var parsed struct {
		Peers []rawConn `json:"Peers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return List{}, &OpError{Code: "kubo_error"}
	}
	byID := map[string]*bucket{}
	order := make([]string, 0)
	for _, conn := range parsed.Peers {
		id := strings.TrimSpace(conn.Peer)
		if id == "" {
			continue
		}
		row := byID[id]
		if row == nil {
			row = &bucket{}
			byID[id] = row
			order = append(order, id)
		}
		if ms := latencyMs(conn.Latency); ms != nil && (row.latency == nil || *ms < *row.latency) {
			row.latency = ms
		}
		if conn.Direction != nil {
			switch *conn.Direction {
			case 1:
				row.in = true
			case 2:
				row.out = true
			}
		}
		if row.agent == "" {
			row.agent = cleanAgent(conn.Identify.AgentVersion)
		}
	}
	rows := make([]Peer, 0, len(order))
	for _, id := range order {
		row := byID[id]
		item := Peer{PeerID: id, LatencyMs: row.latency, Agent: row.agent}
		switch {
		case row.in && row.out:
			item.Direction = "both"
		case row.in:
			item.Direction = "in"
		case row.out:
			item.Direction = "out"
		}
		rows = append(rows, item)
	}
	sort.Slice(rows, func(i, j int) bool {
		li, lj := rows[i].LatencyMs, rows[j].LatencyMs
		if (li == nil) != (lj == nil) {
			return li != nil
		}
		if li != nil && lj != nil && *li != *lj {
			return *li < *lj
		}
		return rows[i].PeerID < rows[j].PeerID
	})
	list := List{Total: len(rows), Peers: rows}
	if len(rows) > MaxRows {
		list.Truncated = true
		list.Peers = append([]Peer(nil), rows[:MaxRows]...)
	}
	if list.Peers == nil {
		list.Peers = []Peer{}
	}
	return list, nil
}

type bucket struct {
	latency *int
	in      bool
	out     bool
	agent   string
}

func latencyMs(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "n/a" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return nil
	}
	ms := int((d + time.Millisecond/2) / time.Millisecond)
	return &ms
}

func cleanAgent(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cs, r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	runes := []rune(out)
	if len(runes) > maxAgentRunes {
		out = string(runes[:maxAgentRunes])
	}
	return strings.TrimSpace(out)
}
