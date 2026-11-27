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

	"path/filepath"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/access"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/api"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuboinstall"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/logs"
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
	logStore, err := logs.Open(logs.Options{Dir: filepath.Join(cfg.DataDir, "logs")})
	if err != nil {
		logger.Error("logs", "err", err)
		os.Exit(1)
	}
	defer func() { _ = logStore.Close() }()
	logger = slog.New(logs.Multi(
		slog.NewTextHandler(os.Stderr, nil),
		logs.NewHandler(logStore),
	))
	slog.SetDefault(logger)
	cfg.Logger = logger
	cfg.KuboLog = logStore.KuboWriter()
	prepared, err := access.Prepare(access.Options{
		Addr:           cfg.ManagerAddr,
		DataDir:        cfg.DataDir,
		TLS:            cfg.TLS,
		CertFile:       cfg.TLSCert,
		KeyFile:        cfg.TLSKey,
		TLSHosts:       access.SplitList(cfg.TLSHosts),
		Hosts:          access.SplitList(cfg.Hosts),
		Insecure:       cfg.Insecure,
		TrustProxy:     cfg.TrustProxy,
		TrustedProxies: access.SplitList(cfg.TrustedProxies),
		WebDir:         cfg.WebDir,
	})
	if err != nil {
		logger.Error("listen", "err", err)
		os.Exit(2)
	}
	// Resolved on every start, so an upgrade is the next daemon without a
	// manager restart. An explicit -kubo-bin still wins inside the supervisor.
	cfg.ManagedBin = func() string {
		return kuboinstall.CurrentPath(cfg.DataDir)
	}

	sup, err := node.New(cfg)
	if err != nil {
		logger.Error("supervisor", "err", err)
		os.Exit(1)
	}

	accounts, err := auth.New(cfg.DataDir, cfg.RepoPath, auth.Options{
		Params:          auth.DefaultParams(),
		TrustProxy:      cfg.TrustProxy,
		Logger:          logger,
		IdleTimeout:     prepared.Idle,
		AbsoluteTimeout: prepared.Absolute,
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

	if err := prepared.Listen(); err != nil {
		logger.Error("listen", "err", err)
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		_ = sup.Shutdown(ctx)
		cancel()
		os.Exit(1)
	}
	handler := api.WithWeb(api.NewMuxConfigured(sup, accounts, logStore, prepared.Snapshot), prepared.Web)
	handler = api.WithTrustedProxies(handler, access.SplitList(cfg.TrustedProxies))
	handler = prepared.Guard(handler)
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("manager listening",
			"addr", prepared.Bound(),
			"data", cfg.DataDir,
			"tls", prepared.TLSMode,
			"remote", prepared.Remote,
			"insecure", prepared.Insecure,
			"trust_proxy", prepared.TrustProxy,
		)
		if prepared.Fingerprint != "" {
			if _, err := fmt.Fprintf(os.Stdout, "ipfs-panel tls fingerprint: %s\n", prepared.Fingerprint); err != nil {
				logger.Error("fingerprint", "err", err)
			}
			logger.Info("tls certificate", "fingerprint", prepared.Fingerprint, "mode", prepared.TLSMode, "renewed", prepared.Renewed)
		}
		if prepared.Expired {
			logger.Warn("tls certificate is expired")
		}
		if prepared.Insecure {
			line := fmt.Sprintf("ipfs-panel warning: listening on %s without TLS. Passwords and sessions travel in cleartext.\n", prepared.Bound())
			_, _ = fmt.Fprint(os.Stdout, line)
			logger.Warn("remote listen without TLS", "addr", prepared.Bound())
		}
		if prepared.InsecureIgnored(cfg.Insecure) {
			logger.Info("insecure ignored because TLS is enabled")
		}
		errCh <- server.Serve(prepared.Listener())
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
