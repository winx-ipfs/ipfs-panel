package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/explore"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type exploreHTTP struct {
	sup     *node.Supervisor
	timeout time.Duration
	status  func(context.Context) (node.Overview, error)
	lookup  func(repo string) (apiBase, gatewayBase string, err error)
}

type exploreGate struct {
	client  *explore.Client
	gateway string
}

type exploreLinkJSON struct {
	Name string `json:"name"`
	CID  string `json:"cid"`
	Size *int64 `json:"size,omitempty"`
}

type exploreJSON struct {
	Target         string            `json:"target"`
	CID            string            `json:"cid"`
	Version        int               `json:"version"`
	Codec          string            `json:"codec"`
	Hash           string            `json:"hash,omitempty"`
	Kind           string            `json:"kind"`
	Size           *int64            `json:"size,omitempty"`
	Focus          string            `json:"focus,omitempty"`
	Text           string            `json:"text,omitempty"`
	Data           json.RawMessage   `json:"data,omitempty"`
	DataOmitted    bool              `json:"dataOmitted,omitempty"`
	Links          []exploreLinkJSON `json:"links"`
	LinksTruncated bool              `json:"linksTruncated,omitempty"`
	LocalGateway   string            `json:"localGateway,omitempty"`
}

func (h *exploreHTTP) get(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	block, err := g.client.Inspect(r.Context(), r.URL.Query().Get("target"))
	if err != nil {
		writeExploreErr(w, err)
		return
	}
	links := make([]exploreLinkJSON, 0, len(block.Links))
	for _, link := range block.Links {
		links = append(links, exploreLinkJSON{Name: link.Name, CID: link.CID, Size: link.Size})
	}
	body := exploreJSON{
		Target:         block.Target,
		CID:            block.CID,
		Version:        block.Version,
		Codec:          block.Codec,
		Hash:           block.Hash,
		Kind:           block.Kind,
		Size:           block.Size,
		Focus:          block.Focus,
		Text:           block.Text,
		Data:           block.Data,
		DataOmitted:    block.DataOmitted,
		Links:          links,
		LinksTruncated: block.LinksTruncated,
	}
	if g.gateway != "" {
		body.LocalGateway = g.gateway
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *exploreHTTP) open(w http.ResponseWriter, r *http.Request) (exploreGate, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return exploreGate{}, false
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
		return exploreGate{}, false
	}
	apiBase, gateway, err := h.endpoints(ov.RepoPath)
	if err != nil {
		slog.Warn("explore endpoints", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return exploreGate{}, false
	}
	timeout := h.timeout
	if timeout <= 0 {
		timeout = explore.DefaultTimeout
	}
	client, err := explore.New(explore.Options{BaseURL: apiBase, Timeout: timeout})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return exploreGate{}, false
	}
	return exploreGate{client: client, gateway: gateway}, true
}

func (h *exploreHTTP) overview(ctx context.Context) (node.Overview, error) {
	if h.status != nil {
		return h.status(ctx)
	}
	return h.sup.Cached(ctx)
}

func (h *exploreHTTP) endpoints(repo string) (string, string, error) {
	if h.lookup != nil {
		return h.lookup(repo)
	}
	return node.LoopbackEndpoints(repo)
}

func writeExploreErr(w http.ResponseWriter, err error) {
	code := explore.CodeOf(err)
	if code == "" {
		code = "unavailable"
	}
	var status int
	switch code {
	case "invalid_target":
		status = http.StatusBadRequest
	case "not_found":
		status = http.StatusNotFound
	case "too_large":
		status = http.StatusRequestEntityTooLarge
	case "timeout":
		status = http.StatusGatewayTimeout
	case "kubo_error":
		status = http.StatusBadGateway
	case "node_not_running":
		status = http.StatusServiceUnavailable
	default:
		code = "unavailable"
		status = http.StatusServiceUnavailable
	}
	if code == "kubo_error" || code == "unavailable" || code == "timeout" || code == "node_not_running" {
		slog.Warn("explore", "code", code)
	}
	body := map[string]string{"error": code}
	if code == "node_not_running" {
		body["state"] = node.StateDown
	}
	writeJSON(w, status, body)
}
