package api

import (
	"encoding/json"
	"net/http"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/explore"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/logs"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type healthBody struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// parts is the set of handlers newMux mounts. A nil feature handler is
// built from sup. Tests pass the one handler they want to replace.
type parts struct {
	sup     *node.Supervisor
	auth    *auth.Service
	files   *filesHTTP
	pins    *pinsHTTP
	explore *exploreHTTP
	peers   *peersHTTP
	peering *peeringHTTP
	config  *configHTTP
	logs    *logsHTTP
	kubo    *kuboHTTP
}

// NewMux is the manager HTTP API.
// GET /health reports this process only.
// Node routes do not proxy Kubo. Feature routes call Kubo on loopback and
// never return that address.
// Every /api route except setup-status, setup, and login requires a session.
func NewMux(sup *node.Supervisor, svc *auth.Service) http.Handler {
	return NewMuxWithLogs(sup, svc, nil)
}

// NewMuxWithLogs is NewMux with the log ring the Logs page reads.
func NewMuxWithLogs(sup *node.Supervisor, svc *auth.Service, store *logs.Store) http.Handler {
	return newMux(parts{
		sup:  sup,
		auth: svc,
		logs: &logsHTTP{sup: sup, store: store, trust: svc.TrustProxy()},
	})
}

func newMux(p parts) http.Handler {
	sup, svc := p.sup, p.auth
	filesHandler, pinsHandler := p.files, p.pins
	exploreHandler, peersHandler := p.explore, p.peers
	peeringHandler, configHandler, logsHandler := p.peering, p.config, p.logs
	kuboHandler := p.kubo
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
	if exploreHandler == nil {
		exploreHandler = &exploreHTTP{sup: sup, timeout: explore.DefaultTimeout}
	} else if exploreHandler.sup == nil {
		exploreHandler.sup = sup
	}
	if peersHandler == nil {
		peersHandler = &peersHTTP{sup: sup}
	} else if peersHandler.sup == nil {
		peersHandler.sup = sup
	}
	if peeringHandler == nil {
		peeringHandler = &peeringHTTP{sup: sup, trust: svc.TrustProxy()}
	} else if peeringHandler.sup == nil {
		peeringHandler.sup = sup
		if !peeringHandler.trust {
			peeringHandler.trust = svc.TrustProxy()
		}
	}
	if configHandler == nil {
		configHandler = &configHTTP{sup: sup, trustProxy: svc.TrustProxy()}
	} else if configHandler.sup == nil {
		configHandler.sup = sup
		if !configHandler.trustProxy {
			configHandler.trustProxy = svc.TrustProxy()
		}
	}
	if logsHandler == nil {
		logsHandler = &logsHTTP{sup: sup, trust: svc.TrustProxy()}
	} else if logsHandler.sup == nil {
		logsHandler.sup = sup
	}
	if kuboHandler == nil {
		kuboHandler = &kuboHTTP{sup: sup, trust: svc.TrustProxy()}
	} else if kuboHandler.sup == nil {
		kuboHandler.sup = sup
		if !kuboHandler.trust {
			kuboHandler.trust = svc.TrustProxy()
		}
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
	mount(mux, filesHandler, pinsHandler, exploreHandler, peersHandler, peeringHandler, configHandler, logsHandler, kuboHandler)
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
