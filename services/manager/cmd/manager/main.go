package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/api"
)

func main() {
	addr := os.Getenv("MANAGER_ADDR")
	if addr == "" {
		// No auth yet. Stay on loopback until a later slice adds login.
		addr = "127.0.0.1:8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           api.NewMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("manager listening on http://%s", addr)
	log.Fatal(server.ListenAndServe())
}
