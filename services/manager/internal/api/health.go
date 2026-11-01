package api

import (
	"encoding/json"
	"net/http"
)

type healthBody struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// NewMux is the manager HTTP API. Kubo is not mounted here.
func NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
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
