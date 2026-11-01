package node

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the manager's settings. Paths come from flags and the
// environment, never from an HTTP body.
type Config struct {
	ManagerAddr string
	RepoPath    string
	DataDir     string
	KuboBin     string
	TrustProxy  bool
	Logger      *slog.Logger

	// Time limits. Zero means the spec default. Tests set shorter values.
	InitTimeout     time.Duration
	ConfigTimeout   time.Duration
	IDTimeout       time.Duration
	ReadyBudget     time.Duration
	ShutdownTimeout time.Duration
	StopKillAfter   time.Duration
	PollInterval    time.Duration

	// Files limits. Zero means the spec default.
	MaxFileBytes         int64
	FilesTimeout         time.Duration
	FilesTransferTimeout time.Duration

	// PinsTimeout is the deadline for one background pin/add. Zero means 30m.
	PinsTimeout time.Duration
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
	if c.MaxFileBytes <= 0 {
		c.MaxFileBytes = 512 << 20
	}
	if c.FilesTimeout <= 0 {
		c.FilesTimeout = 20 * time.Second
	}
	if c.FilesTransferTimeout <= 0 {
		c.FilesTransferTimeout = 10 * time.Minute
	}
	if c.PinsTimeout <= 0 {
		c.PinsTimeout = 30 * time.Minute
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

// DefaultDataDir is $XDG_DATA_HOME/ipfs-panel, or
// ~/.local/share/ipfs-panel when XDG_DATA_HOME is unset.
// The Kubo repo is a child of this directory. Credentials are not.
func DefaultDataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "ipfs-panel"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "ipfs-panel"), nil
}

// DefaultRepoPath is the panel Kubo repo inside DefaultDataDir.
func DefaultRepoPath() (string, error) {
	data, err := DefaultDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "kubo"), nil
}

// LoadConfig reads flags and environment variables.
// Flags win when present. Environment fills the flag default.
//
//	-addr         MANAGER_ADDR              default 127.0.0.1:8080
//	-repo         IPFS_PANEL_REPO           default DefaultRepoPath
//	-data         IPFS_PANEL_DATA           default DefaultDataDir
//	-kubo-bin     IPFS_PANEL_KUBO_BIN       default empty (ipfs on PATH)
//	-trust-proxy              IPFS_PANEL_TRUST_PROXY              default false
//	-max-file-bytes           IPFS_PANEL_MAX_FILE_BYTES           default 512 MiB
//	-files-timeout            IPFS_PANEL_FILES_TIMEOUT            default 20s
//	-files-transfer-timeout   IPFS_PANEL_FILES_TRANSFER_TIMEOUT   default 10m
//	-pins-timeout             IPFS_PANEL_PINS_TIMEOUT             default 30m
func LoadConfig(args []string) (Config, error) {
	repoDefault := os.Getenv("IPFS_PANEL_REPO")
	if repoDefault == "" {
		var err error
		repoDefault, err = DefaultRepoPath()
		if err != nil {
			return Config{}, err
		}
	}
	dataDefault := os.Getenv("IPFS_PANEL_DATA")
	if dataDefault == "" {
		var err error
		dataDefault, err = DefaultDataDir()
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
	data := fs.String("data", dataDefault, "manager data directory (env IPFS_PANEL_DATA)")
	bin := fs.String("kubo-bin", os.Getenv("IPFS_PANEL_KUBO_BIN"), "ipfs binary (env IPFS_PANEL_KUBO_BIN); empty searches PATH")
	trust := fs.Bool("trust-proxy", envBool(os.Getenv("IPFS_PANEL_TRUST_PROXY")), "trust X-Forwarded-Proto and X-Forwarded-For from a reverse proxy (env IPFS_PANEL_TRUST_PROXY)")
	maxDefault, err := envPositiveInt("IPFS_PANEL_MAX_FILE_BYTES")
	if err != nil {
		return Config{}, err
	}
	opDefault, err := envPositiveDuration("IPFS_PANEL_FILES_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	transferDefault, err := envPositiveDuration("IPFS_PANEL_FILES_TRANSFER_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	maxBytes := fs.Int64("max-file-bytes", maxDefault, "largest upload in bytes (env IPFS_PANEL_MAX_FILE_BYTES)")
	opTimeout := fs.Duration("files-timeout", opDefault, "metadata deadline (env IPFS_PANEL_FILES_TIMEOUT)")
	transferTimeout := fs.Duration("files-transfer-timeout", transferDefault, "upload and download deadline (env IPFS_PANEL_FILES_TRANSFER_TIMEOUT)")
	pinDefault, err := envPositiveDuration("IPFS_PANEL_PINS_TIMEOUT")
	if err != nil {
		return Config{}, err
	}
	pinTimeout := fs.Duration("pins-timeout", pinDefault, "pin fetch deadline (env IPFS_PANEL_PINS_TIMEOUT)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", fs.Args())
	}
	return Config{
		ManagerAddr:          *addr,
		RepoPath:             *repo,
		DataDir:              *data,
		KuboBin:              *bin,
		TrustProxy:           *trust,
		MaxFileBytes:         *maxBytes,
		FilesTimeout:         *opTimeout,
		FilesTransferTimeout: *transferTimeout,
		PinsTimeout:          *pinTimeout,
	}, nil
}

func envPositiveInt(key string) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s: %q", key, raw)
	}
	return n, nil
}

func envPositiveDuration(key string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: %q", key, raw)
	}
	return d, nil
}

func envBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
