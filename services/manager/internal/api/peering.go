package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peering"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peers"
)

type peeringHTTP struct {
	sup    *node.Supervisor
	status func(context.Context) (node.Overview, error)
	lookup func(repo string) (apiBase, gatewayBase string, err error)
	svc    *peering.Service
	trust  bool
	log    *slog.Logger
}

func (h *peeringHTTP) list(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.open(w, r, "")
	if !ok {
		return
	}
	view, err := svc.List(r.Context())
	if err != nil {
		h.fail(w, r, "", "", err)
		return
	}
	writeJSON(w, http.StatusOK, peeringJSON(view))
}

func (h *peeringHTTP) add(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.open(w, r, "peering_add")
	if !ok {
		return
	}
	var body struct {
		Address string `json:"address"`
		PeerID  string `json:"peerId"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		h.audit(r, "peering_add", "invalid", "")
		writeProblem(w, err)
		return
	}
	address, peerID := strings.TrimSpace(body.Address), strings.TrimSpace(body.PeerID)
	if (address == "") == (peerID == "") {
		h.audit(r, "peering_add", "invalid", "")
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "bad_request"})
		return
	}
	auditPeer := peerID
	if address != "" {
		if _, id, err := peers.ParseAddress(address); err == nil {
			auditPeer = id
		}
	}
	var (
		view peering.View
		err  error
	)
	if address != "" {
		view, err = svc.AddAddress(r.Context(), address)
	} else {
		view, err = svc.AddPeer(r.Context(), peerID)
	}
	if err != nil {
		h.fail(w, r, "peering_add", auditPeer, err)
		return
	}
	h.audit(r, "peering_add", "success", auditPeer)
	writeJSON(w, http.StatusOK, peeringJSON(view))
}

func (h *peeringHTTP) remove(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.open(w, r, "peering_remove")
	if !ok {
		return
	}
	var body struct {
		PeerID string `json:"peerId"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		h.audit(r, "peering_remove", "invalid", "")
		writeProblem(w, err)
		return
	}
	peerID := strings.TrimSpace(body.PeerID)
	if peerID == "" {
		h.audit(r, "peering_remove", "invalid", "")
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "bad_request"})
		return
	}
	view, err := svc.Remove(r.Context(), peerID)
	if err != nil {
		h.fail(w, r, "peering_remove", peerID, err)
		return
	}
	h.audit(r, "peering_remove", "success", peerID)
	writeJSON(w, http.StatusOK, peeringJSON(view))
}

func (h *peeringHTTP) apply(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.open(w, r, "peering_apply")
	if !ok {
		return
	}
	if err := rejectBody(w, r); err != nil {
		h.audit(r, "peering_apply", "invalid", "")
		writeProblem(w, err)
		return
	}
	view, err := svc.Apply(r.Context())
	if err != nil {
		h.fail(w, r, "peering_apply", "", err)
		return
	}
	result := "success"
	if view.Sync == peering.SyncRestart {
		result = "restart"
	}
	h.audit(r, "peering_apply", result, "")
	writeJSON(w, http.StatusOK, peeringJSON(view))
}

func (h *peeringHTTP) open(w http.ResponseWriter, r *http.Request, event string) (*peering.Service, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		if event != "" {
			h.audit(r, event, "unavailable", "")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	if ov.State != node.StateUp {
		state := ov.State
		if state == "" {
			state = node.StateDown
		}
		if event != "" {
			h.audit(r, event, "unavailable", "")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "node_not_running",
			"state": state,
		})
		return nil, false
	}
	if h.svc != nil {
		return h.svc, true
	}
	repo := ov.RepoPath
	if repo == "" && h.sup != nil {
		repo = h.sup.RepoPath()
	}
	apiBase, _, err := h.endpoints(repo)
	if err != nil {
		h.logger().Warn("peering endpoints", "err", err)
		if event != "" {
			h.audit(r, event, "unavailable", "")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	editClient, err := configedit.NewClient(apiBase, 0)
	if err != nil {
		if event != "" {
			h.audit(r, event, "unavailable", "")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	live, err := peering.NewClient(apiBase)
	if err != nil {
		if event != "" {
			h.audit(r, event, "unavailable", "")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	data, addr := "", "127.0.0.1:8080"
	if h.sup != nil {
		data = h.sup.DataDir()
		if h.sup.ManagerAddr() != "" {
			addr = h.sup.ManagerAddr()
		}
	}
	return peering.New(configedit.New(editClient, repo, data, addr), live), true
}

func (h *peeringHTTP) overview(ctx context.Context) (node.Overview, error) {
	if h.status != nil {
		return h.status(ctx)
	}
	return h.sup.Cached(ctx)
}

func (h *peeringHTTP) endpoints(repo string) (string, string, error) {
	if h.lookup != nil {
		return h.lookup(repo)
	}
	return node.LoopbackEndpoints(repo)
}

func (h *peeringHTTP) fail(w http.ResponseWriter, r *http.Request, event, peerID string, err error) {
	peeringCode := peering.CodeOf(err)
	status := http.StatusServiceUnavailable
	if pe := errorOf(err); pe != nil {
		peeringCode = pe.Code
		status = pe.Status
	}
	if peeringCode == "" {
		peeringCode = "unavailable"
	}
	switch peeringCode {
	case "invalid_address", "invalid_peer", "bad_request":
		status = http.StatusBadRequest
	case "not_connected", "too_many", "api_locked", "identity_locked":
		status = http.StatusConflict
	case "too_large":
		status = http.StatusRequestEntityTooLarge
	case "timeout":
		status = http.StatusGatewayTimeout
	case "kubo_error":
		status = http.StatusBadGateway
	case "node_not_running":
		status = http.StatusServiceUnavailable
	}
	if event != "" {
		result := "unavailable"
		switch status {
		case http.StatusBadRequest:
			result = "invalid"
		case http.StatusConflict:
			result = "forbidden"
		case http.StatusBadGateway:
			result = "rejected"
		}
		h.audit(r, event, result, peerID)
	}
	if peeringCode == "kubo_error" || peeringCode == "unavailable" || peeringCode == "timeout" || peeringCode == "node_not_running" {
		h.logger().Warn("peering", "code", peeringCode)
	}
	body := map[string]string{"error": peeringCode}
	if peeringCode == "node_not_running" {
		body["state"] = node.StateDown
	}
	writeJSON(w, status, body)
}

func errorOf(err error) *peering.Error {
	var pe *peering.Error
	if errors.As(err, &pe) {
		return pe
	}
	return nil
}

func (h *peeringHTTP) audit(r *http.Request, event, result, peerID string) {
	username := ""
	if principal, ok := principalFrom(r); ok {
		username = principal.username
	}
	h.logger().Info("audit",
		"event", event,
		"result", result,
		"username", username,
		"ip", clientIP(r, h.trust),
		"peer", peerID,
	)
}

func (h *peeringHTTP) logger() *slog.Logger {
	if h.log != nil {
		return h.log
	}
	return slog.Default()
}

type peeringPeerJSON struct {
	PeerID string   `json:"peerId"`
	Addrs  []string `json:"addrs"`
	State  string   `json:"state"`
}

type peeringViewJSON struct {
	Sync  string            `json:"sync"`
	Peers []peeringPeerJSON `json:"peers"`
}

func peeringJSON(view peering.View) peeringViewJSON {
	rows := make([]peeringPeerJSON, 0, len(view.Peers))
	for _, peer := range view.Peers {
		addrs := peer.Addrs
		if addrs == nil {
			addrs = []string{}
		}
		rows = append(rows, peeringPeerJSON{PeerID: peer.PeerID, Addrs: addrs, State: peer.State})
	}
	sync := view.Sync
	if sync == "" {
		sync = peering.SyncOK
	}
	return peeringViewJSON{Sync: sync, Peers: rows}
}
