package api

import (
	"encoding/json"
	"net/http"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type healthBody struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// NewMux is the manager HTTP API.
// GET /health reports this process only. Node routes do not proxy Kubo.
func NewMux(sup *node.Supervisor) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
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
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthBody{
		Status:  "ok",
		Service: "manager",
	})
}
