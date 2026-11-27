package api

import "net/http"

// route is one manager endpoint. A feature adds itself by implementing
// routes. newMux is the only place that mounts them, and the only place
// that knows the session wrapper. Auth and the node lifecycle stay inline
// in newMux because they close over the process.
type route struct {
	method  string
	pattern string
	handler http.HandlerFunc
}

type routes interface {
	routes() []route
}

func mount(mux *http.ServeMux, sets ...routes) {
	for _, set := range sets {
		for _, rt := range set.routes() {
			mux.HandleFunc(rt.method+" "+rt.pattern, rt.handler)
		}
	}
}

func (h *filesHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/files", h.list},
		{http.MethodGet, "/api/files/stat", h.stat},
		{http.MethodGet, "/api/files/download", h.download},
		{http.MethodPost, "/api/files/mkdir", h.mkdir},
		{http.MethodPost, "/api/files/upload", h.upload},
		{http.MethodPost, "/api/files/rename", h.rename},
		{http.MethodPost, "/api/files/move", h.move},
		{http.MethodPost, "/api/files/copy", h.copy},
		{http.MethodPost, "/api/files/delete", h.delete},
	}
}

func (h *pinsHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/pins", h.list},
		{http.MethodPost, "/api/pins", h.add},
		{http.MethodPost, "/api/pins/remove", h.remove},
		{http.MethodPost, "/api/pins/from-files", h.fromFiles},
		{http.MethodPost, "/api/pins/upload", h.upload},
		{http.MethodGet, "/api/pins/jobs", h.jobsList},
		{http.MethodGet, "/api/pins/jobs/{id}", h.jobGet},
		{http.MethodPost, "/api/pins/jobs/{id}/cancel", h.jobCancel},
	}
}

func (h *exploreHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/explore", h.get},
	}
}

func (h *peersHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/peers", h.list},
		{http.MethodPost, "/api/peers/connect", h.connect},
	}
}

func (h *peeringHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/peers/permanent", h.list},
		{http.MethodPost, "/api/peers/permanent", h.add},
		{http.MethodPost, "/api/peers/permanent/remove", h.remove},
		{http.MethodPost, "/api/peers/permanent/apply", h.apply},
	}
}

func (h *configHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/config", h.get},
		{http.MethodPost, "/api/config", h.replace},
		{http.MethodPost, "/api/config/restore", h.restore},
	}
}

func (h *logsHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/logs", h.list},
		{http.MethodGet, "/api/logs/stream", h.stream},
		{http.MethodGet, "/api/logs/download", h.download},
		{http.MethodGet, "/api/logs/levels", h.levels},
		{http.MethodPost, "/api/logs/level", h.setLevel},
	}
}
