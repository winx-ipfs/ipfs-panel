package api

import (
	"encoding/json"
	"net/http"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type healthBody struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// NewMux is the manager HTTP API.
// GET /health reports this process only.
// Node routes do not proxy Kubo. Files and pins routes proxy Kubo
// on loopback and never return that address.
// Every /api route except setup-status, setup, and login requires a session.
func NewMux(sup *node.Supervisor, svc *auth.Service) http.Handler {
	return newMux(sup, svc, nil, nil)
}

func newMux(sup *node.Supervisor, svc *auth.Service, filesHandler *filesHTTP, pinsHandler *pinsHTTP) http.Handler {
	maxBytes, op, transfer := sup.FileLimits()
	if filesHandler == nil {
		filesHandler = &filesHTTP{
			sup:             sup,
			maxBytes:        maxBytes,
			op:              op,
			transferTimeout: transfer,
		}
	} else if filesHandler.sup == nil {
		filesHandler.sup = sup
	}
	if pinsHandler == nil {
		pinsHandler = &pinsHTTP{
			sup:        sup,
			dataDir:    sup.DataDir(),
			maxBytes:   maxBytes,
			op:         op,
			transfer:   transfer,
			pinTimeout: sup.PinTimeout(),
		}
	} else if pinsHandler.sup == nil {
		pinsHandler.sup = sup
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	accounts := &authHTTP{svc: svc}
	mux.HandleFunc("GET /api/auth/setup-status", accounts.setupStatus)
	mux.HandleFunc("POST /api/auth/setup", accounts.setup)
	mux.HandleFunc("POST /api/auth/login", accounts.login)
	mux.HandleFunc("GET /api/auth/session", accounts.session)
	mux.HandleFunc("POST /api/auth/logout", accounts.logout)
	mux.HandleFunc("POST /api/auth/password", accounts.password)
	nodes := &nodeAPI{sup: sup}
	mux.HandleFunc("GET /api/node", nodes.get)
	mux.HandleFunc("POST /api/node/start", nodes.post(func(r *http.Request) (node.Overview, error) {
		return sup.Start(r.Context())
	}))
	mux.HandleFunc("POST /api/node/stop", nodes.post(func(r *http.Request) (node.Overview, error) {
		return sup.Stop(r.Context())
	}))
	mux.HandleFunc("POST /api/node/restart", nodes.post(func(r *http.Request) (node.Overview, error) {
		return sup.Restart(r.Context())
	}))
	mux.HandleFunc("GET /api/scenes", nodes.scenesList)
	mux.HandleFunc("GET /api/scenes/preview", nodes.scenesPreview)
	mux.HandleFunc("POST /api/scenes/apply", nodes.scenesApply)
	mux.HandleFunc("POST /api/scenes/revert", nodes.scenesRevert)
	mux.HandleFunc("GET /api/files", filesHandler.list)
	mux.HandleFunc("GET /api/files/stat", filesHandler.stat)
	mux.HandleFunc("GET /api/files/download", filesHandler.download)
	mux.HandleFunc("POST /api/files/mkdir", filesHandler.mkdir)
	mux.HandleFunc("POST /api/files/upload", filesHandler.upload)
	mux.HandleFunc("POST /api/files/rename", filesHandler.rename)
	mux.HandleFunc("POST /api/files/move", filesHandler.move)
	mux.HandleFunc("POST /api/files/copy", filesHandler.copy)
	mux.HandleFunc("POST /api/files/delete", filesHandler.delete)
	mux.HandleFunc("GET /api/pins", pinsHandler.list)
	mux.HandleFunc("POST /api/pins", pinsHandler.add)
	mux.HandleFunc("POST /api/pins/remove", pinsHandler.remove)
	mux.HandleFunc("POST /api/pins/from-files", pinsHandler.fromFiles)
	mux.HandleFunc("POST /api/pins/upload", pinsHandler.upload)
	mux.HandleFunc("GET /api/pins/jobs", pinsHandler.jobsList)
	mux.HandleFunc("GET /api/pins/jobs/{id}", pinsHandler.jobGet)
	mux.HandleFunc("POST /api/pins/jobs/{id}/cancel", pinsHandler.jobCancel)
	return protect(svc, mux)
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthBody{
		Status:  "ok",
		Service: "manager",
	})
}
