package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/access"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
)

type accessHTTP struct {
	svc  *auth.Service
	snap func() access.Snapshot
}

func (h *accessHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/access", h.get},
		{http.MethodGet, "/api/access/sessions", h.sessions},
		{http.MethodPost, "/api/access/sessions/revoke", h.revoke},
		{http.MethodPost, "/api/access/sessions/revoke-others", h.revokeOthers},
	}
}

func (h *accessHTTP) snapshot() access.Snapshot {
	if h.snap != nil {
		snap := h.snap()
		if snap.Listen == nil {
			snap.Listen = []string{}
		}
		if snap.Reachable == nil {
			snap.Reachable = []string{}
		}
		if snap.Hosts == nil {
			snap.Hosts = []string{}
		}
		return snap
	}
	return access.Snapshot{
		Loopback:  true,
		Scheme:    "http",
		TLS:       "off",
		Listen:    []string{"127.0.0.1:8080"},
		Reachable: []string{},
		Hosts:     []string{},
	}
}

func (h *accessHTTP) get(w http.ResponseWriter, r *http.Request) {
	if _, ok := principalFrom(r); !ok {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, h.snapshot())
}

type sessionJSON struct {
	ID      string `json:"id"`
	Current bool   `json:"current"`
	Created string `json:"created"`
	Seen    string `json:"seen"`
	IP      string `json:"ip"`
}

func (h *accessHTTP) sessions(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r)
	if !ok {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	views := h.svc.Sessions(p.raw)
	out := make([]sessionJSON, 0, len(views))
	for _, view := range views {
		out = append(out, sessionJSON{
			ID:      view.ID,
			Current: view.Current,
			Created: view.Created.UTC().Format(time.RFC3339),
			Seen:    view.Seen.UTC().Format(time.RFC3339),
			IP:      view.IP,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (h *accessHTTP) revoke(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r)
	if !ok {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := readAuthJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	if strings.TrimSpace(body.ID) == "" {
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "bad_request"})
		return
	}
	current, err := h.svc.RevokeSession(p.raw, body.ID, clientIP(r, h.svc.TrustProxy()))
	if errors.Is(err, auth.ErrSessionNotFound) {
		writeProblem(w, &statusError{code: http.StatusNotFound, msg: "not_found"})
		return
	}
	if err != nil {
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
		return
	}
	if current {
		clearSessionCookie(w, r, h.svc.TrustProxy())
		clearCSRFCookie(w, r, h.svc.TrustProxy())
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *accessHTTP) revokeOthers(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r)
	if !ok {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	var body struct{}
	if err := readAuthJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	if err := h.svc.RevokeOtherSessions(p.raw, clientIP(r, h.svc.TrustProxy())); err != nil {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}
