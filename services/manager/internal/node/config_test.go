package node

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("MANAGER_ADDR", "")
	t.Setenv("IPFS_PANEL_REPO", "")
	t.Setenv("IPFS_PANEL_DATA", "")
	t.Setenv("IPFS_PANEL_KUBO_BIN", "")
	t.Setenv("IPFS_PANEL_TRUST_PROXY", "")
	t.Setenv("IPFS_PANEL_TLS", "")
	t.Setenv("IPFS_PANEL_TLS_CERT", "")
	t.Setenv("IPFS_PANEL_TLS_KEY", "")
	t.Setenv("IPFS_PANEL_TLS_HOSTS", "")
	t.Setenv("IPFS_PANEL_HOSTS", "")
	t.Setenv("IPFS_PANEL_INSECURE", "")
	t.Setenv("IPFS_PANEL_TRUSTED_PROXIES", "")
	t.Setenv("IPFS_PANEL_WEB", "")
	t.Setenv("XDG_DATA_HOME", "")

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManagerAddr != "127.0.0.1:8080" {
		t.Fatalf("addr %s", cfg.ManagerAddr)
	}
	home, err := DefaultRepoPath()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RepoPath != home {
		t.Fatalf("repo %s want %s", cfg.RepoPath, home)
	}
	if cfg.KuboBin != "" {
		t.Fatalf("bin %s", cfg.KuboBin)
	}
	data, err := DefaultDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != data {
		t.Fatalf("data %s want %s", cfg.DataDir, data)
	}
	if cfg.TrustProxy || cfg.Insecure || cfg.TLS != "off" || cfg.WebDir != "" || cfg.TrustedProxies != "" {
		t.Fatalf("remote defaults %+v", cfg)
	}
}

func TestLoadConfigEnvAndFlags(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "xdg"))
	t.Setenv("MANAGER_ADDR", "127.0.0.1:9091")
	t.Setenv("IPFS_PANEL_KUBO_BIN", "/usr/bin/ipfs")

	envRepo := filepath.Join(t.TempDir(), "from-env")
	t.Setenv("IPFS_PANEL_REPO", envRepo)

	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManagerAddr != "127.0.0.1:9091" || cfg.KuboBin != "/usr/bin/ipfs" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.RepoPath != envRepo {
		t.Fatalf("repo %s", cfg.RepoPath)
	}

	flagRepo := filepath.Join(t.TempDir(), "from-flag")
	cfg, err = LoadConfig([]string{"-addr", "127.0.0.1:9092", "-repo", flagRepo, "-kubo-bin", ""})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManagerAddr != "127.0.0.1:9092" || cfg.RepoPath != flagRepo {
		t.Fatalf("flags lost %+v", cfg)
	}

	xdg := filepath.Join(t.TempDir(), "xdg2")
	t.Setenv("XDG_DATA_HOME", xdg)
	t.Setenv("IPFS_PANEL_REPO", "")
	got, err := DefaultRepoPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "ipfs-panel", "kubo")
	if got != want {
		t.Fatalf("xdg repo %s want %s", got, want)
	}
}

func TestLoadConfigFileLimits(t *testing.T) {
	t.Setenv("IPFS_PANEL_REPO", t.TempDir())
	t.Setenv("IPFS_PANEL_MAX_FILE_BYTES", "1024")
	t.Setenv("IPFS_PANEL_FILES_TIMEOUT", "3s")
	t.Setenv("IPFS_PANEL_FILES_TRANSFER_TIMEOUT", "1m")
	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxFileBytes != 1024 || cfg.FilesTimeout != 3*time.Second || cfg.FilesTransferTimeout != time.Minute {
		t.Fatalf("%+v", cfg)
	}
	cfg, err = LoadConfig([]string{"-max-file-bytes", "2048", "-files-timeout", "4s", "-files-transfer-timeout", "2m"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxFileBytes != 2048 || cfg.FilesTimeout != 4*time.Second || cfg.FilesTransferTimeout != 2*time.Minute {
		t.Fatalf("flags %+v", cfg)
	}
	cfg, err = LoadConfig([]string{"-pins-timeout", "15m"})
	if err != nil || cfg.PinsTimeout != 15*time.Minute {
		t.Fatalf("pins timeout %+v %v", cfg, err)
	}
	t.Setenv("IPFS_PANEL_FILES_TIMEOUT", "nope")
	if _, err := LoadConfig(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadConfigRejectsArgs(t *testing.T) {
	t.Setenv("IPFS_PANEL_REPO", t.TempDir())
	if _, err := LoadConfig([]string{"extra"}); err == nil {
		t.Fatal("expected error")
	}
}
