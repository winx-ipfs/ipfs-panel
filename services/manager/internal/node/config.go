package node

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Config is the manager's node settings. Paths come from flags and the
// environment, never from an HTTP body.
type Config struct {
	ManagerAddr string
	RepoPath    string
	KuboBin     string
	Logger      *slog.Logger

	// Time limits. Zero means the spec default. Tests set shorter values.
	InitTimeout     time.Duration
	ConfigTimeout   time.Duration
	IDTimeout       time.Duration
	ReadyBudget     time.Duration
	ShutdownTimeout time.Duration
	StopKillAfter   time.Duration
	PollInterval    time.Duration
}

func (c *Config) applyDefaults() {
	if c.InitTimeout <= 0 {
		c.InitTimeout = 60 * time.Second
	}
	if c.ConfigTimeout <= 0 {
		c.ConfigTimeout = 10 * time.Second
	}
	if c.IDTimeout <= 0 {
		c.IDTimeout = 2 * time.Second
	}
	if c.ReadyBudget <= 0 {
		c.ReadyBudget = 30 * time.Second
	}
	if c.ShutdownTimeout <= 0 {
		c.ShutdownTimeout = 5 * time.Second
	}
	if c.StopKillAfter <= 0 {
		c.StopKillAfter = 10 * time.Second
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 200 * time.Millisecond
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

// DefaultRepoPath is $XDG_DATA_HOME/ipfs-panel/kubo, or
// ~/.local/share/ipfs-panel/kubo when XDG_DATA_HOME is unset.
func DefaultRepoPath() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "ipfs-panel", "kubo"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "ipfs-panel", "kubo"), nil
}

// LoadConfig reads flags and environment variables.
// Flags win when present. Environment fills the flag default.
//
//	-addr      MANAGER_ADDR           default 127.0.0.1:8080
//	-repo      IPFS_PANEL_REPO        default DefaultRepoPath
//	-kubo-bin  IPFS_PANEL_KUBO_BIN    default empty (ipfs on PATH)
func LoadConfig(args []string) (Config, error) {
	repoDefault := os.Getenv("IPFS_PANEL_REPO")
	if repoDefault == "" {
		var err error
		repoDefault, err = DefaultRepoPath()
		if err != nil {
			return Config{}, err
		}
	}
	addrDefault := os.Getenv("MANAGER_ADDR")
	if addrDefault == "" {
		addrDefault = "127.0.0.1:8080"
	}

	fs := flag.NewFlagSet("manager", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", addrDefault, "listen address (env MANAGER_ADDR)")
	repo := fs.String("repo", repoDefault, "panel Kubo repo (env IPFS_PANEL_REPO)")
	bin := fs.String("kubo-bin", os.Getenv("IPFS_PANEL_KUBO_BIN"), "ipfs binary (env IPFS_PANEL_KUBO_BIN); empty searches PATH")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", fs.Args())
	}
	return Config{
		ManagerAddr: *addr,
		RepoPath:    *repo,
		KuboBin:     *bin,
	}, nil
}
