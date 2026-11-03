package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/api"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	if len(os.Args) > 1 && os.Args[1] == "reset-admin" {
		if err := resetAdmin(os.Args[2:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				os.Exit(0)
			}
			logger.Error("reset-admin", "err", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := node.LoadConfig(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logger.Error("config", "err", err)
		os.Exit(2)
	}
	cfg.Logger = logger

	sup, err := node.New(cfg)
	if err != nil {
		logger.Error("supervisor", "err", err)
		os.Exit(1)
	}

	accounts, err := auth.New(cfg.DataDir, cfg.RepoPath, auth.Options{
		Params:     auth.DefaultParams(),
		TrustProxy: cfg.TrustProxy,
		Logger:     logger,
	})
	if err != nil {
		logger.Error("auth", "err", err)
		os.Exit(1)
	}
	if token, ok := accounts.TakeSetupToken(); ok {
		// The token is a secret. It is printed once so an operator who is
		// not on loopback can finish setup. It is not an audit event.
		if _, err := fmt.Fprintf(os.Stdout, "ipfs-panel setup token: %s\n", token); err != nil {
			logger.Error("setup token", "err", err)
			os.Exit(1)
		}
		logger.Warn("one-time setup token; treat this line as a secret", "setup_token", token)
	}

	server := &http.Server{
		Addr:              cfg.ManagerAddr,
		Handler:           api.NewMux(sup, accounts),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("manager listening", "addr", cfg.ManagerAddr, "data", cfg.DataDir)
		errCh <- server.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "err", err)
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			_ = sup.Shutdown(ctx)
			os.Exit(1)
		}
	case <-sig:
		logger.Info("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := sup.Shutdown(ctx); err != nil {
			logger.Error("stop node", "err", err)
		}
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("http shutdown", "err", err)
		}
	}
}
