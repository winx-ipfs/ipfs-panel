package api

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/access"
)

type trustedNetsKey struct{}

// WithTrustedProxies records extra proxy networks on the request.
// Loopback peers are trusted whenever -trust-proxy is on; these are the others.
// Invalid entries are ignored here because startup already refused them.
func WithTrustedProxies(next http.Handler, cidrs []string) http.Handler {
	nets, err := access.ParseNets(cidrs)
	if err != nil {
		nets = nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), trustedNetsKey{}, nets)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func netsFrom(r *http.Request) []*net.IPNet {
	nets, _ := r.Context().Value(trustedNetsKey{}).([]*net.IPNet)
	return nets
}

// WithWeb serves the panel on the same origin as the API.
// /api and /health stay on the API handler.
func WithWeb(apiHandler, web http.Handler) http.Handler {
	if web == nil {
		return apiHandler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		web.ServeHTTP(w, r)
	})
}
