// Command fakeipfs is a stand-in for the Kubo binary. Tests put it on PATH.
// It is not part of the manager that ships.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ipfs <command>")
		os.Exit(2)
	}
	logCmd(os.Args[1:])
	code := 2
	switch os.Args[1] {
	case "init":
		code = cmdInit()
	case "daemon":
		code = cmdDaemon()
	case "id":
		code = cmdID()
	case "shutdown":
		code = cmdShutdown()
	case "config":
		code = cmdConfig(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
	}
	os.Exit(code)
}

type repoConfig struct {
	API     json.RawMessage `json:"API"`
	Gateway json.RawMessage `json:"Gateway"`
	Swarm   json.RawMessage `json:"Swarm"`
}

func repoDir() (string, int) {
	repo := os.Getenv("IPFS_PATH")
	if repo == "" {
		fmt.Fprintln(os.Stderr, "IPFS_PATH is empty")
		return "", 1
	}
	return repo, 0
}

func configPath(repo string) string { return filepath.Join(repo, "config") }

func loadConfig(repo string) (repoConfig, error) {
	b, err := os.ReadFile(configPath(repo))
	if err != nil {
		return repoConfig{}, err
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err != nil {
		return repoConfig{}, err
	}
	// A full Kubo config keeps addresses under Addresses. The lifecycle
	// tests still write the addresses object itself.
	if raw := bytes.TrimSpace(probe["Addresses"]); len(raw) > 0 && raw[0] == '{' {
		var cfg repoConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return repoConfig{}, err
		}
		return cfg, nil
	}
	var cfg repoConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return repoConfig{}, err
	}
	return cfg, nil
}

func saveConfig(repo string, cfg repoConfig) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(configPath(repo), b, 0o600)
}

func cmdInit() int {
	sleepEnv("FAKE_INIT_SLEEP")
	if os.Getenv("FAKE_INIT_FAIL") != "" {
		fmt.Fprintln(os.Stderr, "init failed: disk")
		return 1
	}
	repo, code := repoDir()
	if code != 0 {
		return code
	}
	if err := os.MkdirAll(repo, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := os.Stat(configPath(repo)); err == nil {
		fmt.Fprintln(os.Stderr, "already initialized")
		return 1
	}
	api := json.RawMessage(`"/ip4/127.0.0.1/tcp/5001"`)
	if os.Getenv("FAKE_API_TYPE") == "array" {
		api = json.RawMessage(`["/ip4/127.0.0.1/tcp/5001"]`)
	}
	cfg := repoConfig{
		API:     api,
		Gateway: json.RawMessage(`["/ip4/127.0.0.1/tcp/8080"]`),
		Swarm:   json.RawMessage(`["/ip4/0.0.0.0/tcp/4001","/ip6/::/tcp/4001"]`),
	}
	if err := saveConfig(repo, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func cmdConfig(args []string) int {
	jsonMode := false
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonMode = true
			continue
		}
		rest = append(rest, arg)
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "config key required")
		return 2
	}
	repo, code := repoDir()
	if code != 0 {
		return code
	}
	cfg, err := loadConfig(repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	key := rest[0]
	if len(rest) == 1 {
		if key != "Addresses" {
			fmt.Fprintf(os.Stderr, "unknown key %s\n", key)
			return 2
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(cfg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	value := rest[1]
	switch key {
	case "Addresses.API":
		raw, err := configValue(jsonMode, value)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		cfg.API = raw
	case "Addresses.Gateway":
		raw, err := configValue(jsonMode, value)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		cfg.Gateway = raw
	default:
		fmt.Fprintf(os.Stderr, "refusing to set %s\n", key)
		return 2
	}
	if err := saveConfig(repo, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func configValue(jsonMode bool, value string) (json.RawMessage, error) {
	if jsonMode {
		if !json.Valid([]byte(value)) {
			return nil, fmt.Errorf("invalid JSON")
		}
		return json.RawMessage(value), nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func cmdDaemon() int {
	if msg := os.Getenv("FAKE_DAEMON_EXIT"); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		return 1
	}
	repo, code := repoDir()
	if code != 0 {
		return code
	}
	release, err := holdRepoLock(filepath.Join(repo, "repo.lock"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer release()
	pidFile := filepath.Join(repo, "fake-daemon.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = os.Remove(pidFile) }()

	if os.Getenv("FAKE_IGNORE_TERM") != "" {
		signal.Ignore(syscall.SIGTERM, syscall.SIGINT)
		// select{} makes the runtime report a deadlock and exit.
		for {
			time.Sleep(time.Hour)
		}
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch
	return 0
}

func cmdID() int {
	if os.Getenv("FAKE_ID_FAIL") != "" {
		fmt.Fprintln(os.Stderr, "id failed")
		return 1
	}
	sleepEnv("FAKE_ID_SLEEP")
	repo, code := repoDir()
	if code != 0 {
		return code
	}
	if !daemonAlive(repo) {
		fmt.Fprintln(os.Stderr, "daemon not ready")
		return 1
	}
	fmt.Println(`{"ID":"12D3KooWFakePeer","AgentVersion":"kubo/0.0.0-fake/","Addresses":["/ip4/0.0.0.0/tcp/4001/p2p/12D3KooWFakePeer"],"Protocols":["/ipfs/id/1.0.0"]}`)
	return 0
}

func cmdShutdown() int {
	sleepEnv("FAKE_SHUTDOWN_SLEEP")
	if os.Getenv("FAKE_SHUTDOWN_IGNORE") != "" {
		return 0
	}
	repo, code := repoDir()
	if code != 0 {
		return code
	}
	b, err := os.ReadFile(filepath.Join(repo, "fake-daemon.pid"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		fmt.Fprintln(os.Stderr, "bad daemon pid")
		return 1
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func daemonAlive(repo string) bool {
	b, err := os.ReadFile(filepath.Join(repo, "fake-daemon.pid"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	return processAlive(pid)
}

func sleepEnv(key string) {
	v := os.Getenv(key)
	if v == "" {
		return
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return
	}
	time.Sleep(d)
}

func logCmd(args []string) {
	path := os.Getenv("FAKE_IPFS_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer func() { _ = f.Close() }()
	_ = json.NewEncoder(f).Encode(struct {
		Args     []string `json:"args"`
		IPFSPath string   `json:"ipfsPath"`
	}{Args: args, IPFSPath: os.Getenv("IPFS_PATH")})
}
