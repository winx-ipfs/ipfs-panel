package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peers"
)

type peersHTTP struct {
	sup            *node.Supervisor
	listTimeout    time.Duration
	connectTimeout time.Duration
	status         func(context.Context) (node.Overview, error)
	lookup         func(repo string) (apiBase, gatewayBase string, err error)
}

type peerJSON struct {
	PeerID    string `json:"peerId"`
	Direction string `json:"direction,omitempty"`
	LatencyMs *int   `json:"latencyMs,omitempty"`
	Agent     string `json:"agent,omitempty"`
}

type peersListJSON struct {
	Total     int        `json:"total"`
	Truncated bool       `json:"truncated,omitempty"`
	Peers     []peerJSON `json:"peers"`
}

func (h *peersHTTP) list(w http.ResponseWriter, r *http.Request) {
	client, ok := h.open(w, r)
	if !ok {
		return
	}
	found, err := client.List(r.Context())
	if err != nil {
		writePeersErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPeersJSON(found))
}

func (h *peersHTTP) connect(w http.ResponseWriter, r *http.Request) {
	client, ok := h.open(w, r)
	if !ok {
		return
	}
	var body struct {
		Address string `json:"address"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	peerID, err := client.Connect(r.Context(), body.Address)
	if err != nil {
		writePeersErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"peerId": peerID})
}

func (h *peersHTTP) open(w http.ResponseWriter, r *http.Request) (*peers.Client, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	if ov.State != node.StateUp {
		state := ov.State
		if state == "" {
			state = node.StateDown
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "node_not_running",
			"state": state,
		})
		return nil, false
	}
	apiBase, _, err := h.endpoints(ov.RepoPath)
	if err != nil {
		slog.Warn("peers endpoints", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	client, err := peers.New(peers.Options{
		BaseURL:        apiBase,
		ListTimeout:    h.listTimeout,
		ConnectTimeout: h.connectTimeout,
	})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	return client, true
}

func (h *peersHTTP) overview(ctx context.Context) (node.Overview, error) {
	if h.status != nil {
		return h.status(ctx)
	}
	return h.sup.Cached(ctx)
}

func (h *peersHTTP) endpoints(repo string) (string, string, error) {
	if h.lookup != nil {
		return h.lookup(repo)
	}
	return node.LoopbackEndpoints(repo)
}

func toPeersJSON(list peers.List) peersListJSON {
	rows := make([]peerJSON, 0, len(list.Peers))
	for _, peer := range list.Peers {
		rows = append(rows, peerJSON{
			PeerID:    peer.PeerID,
			Direction: peer.Direction,
			LatencyMs: peer.LatencyMs,
			Agent:     peer.Agent,
		})
	}
	return peersListJSON{Total: list.Total, Truncated: list.Truncated, Peers: rows}
}

func writePeersErr(w http.ResponseWriter, err error) {
	code := peers.CodeOf(err)
	if code == "" {
		code = "unavailable"
	}
	var status int
	switch code {
	case "invalid_address":
		status = http.StatusBadRequest
	case "too_large":
		status = http.StatusRequestEntityTooLarge
	case "timeout":
		status = http.StatusGatewayTimeout
	case "connect_failed", "kubo_error":
		status = http.StatusBadGateway
	case "node_not_running":
		status = http.StatusServiceUnavailable
	default:
		code = "unavailable"
		status = http.StatusServiceUnavailable
	}
	slog.Warn("peers", "code", code)
	body := map[string]string{"error": code}
	if code == "node_not_running" {
		body["state"] = node.StateDown
	}
	writeJSON(w, status, body)
}
