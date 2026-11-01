package node

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/fakebin"
)

type invocation struct {
	Args     []string `json:"args"`
	IPFSPath string   `json:"ipfsPath"`
}

func putFakeOnPath(t *testing.T) string {
	t.Helper()
	bin := fakebin.Path(t)
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

func newTestSupervisor(t *testing.T, tweak func(*Config)) (*Supervisor, string, string) {
	t.Helper()
	repo := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "cmds.log")
	t.Setenv("FAKE_IPFS_LOG", logPath)
	putFakeOnPath(t)
	cfg := Config{
		ManagerAddr:     "127.0.0.1:8080",
		RepoPath:        repo,
		InitTimeout:     3 * time.Second,
		ConfigTimeout:   2 * time.Second,
		IDTimeout:       time.Second,
		ReadyBudget:     3 * time.Second,
		ShutdownTimeout: time.Second,
		StopKillAfter:   3 * time.Second,
		PollInterval:    15 * time.Millisecond,
	}
	if tweak != nil {
		tweak(&cfg)
	}
	sup, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sup.Shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
	return sup, repo, logPath
}

func readLog(t *testing.T, path string) []invocation {
	t.Helper()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []invocation
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		var inv invocation
		if err := json.Unmarshal(sc.Bytes(), &inv); err != nil {
			t.Fatalf("log line: %v (%s)", err, sc.Text())
		}
		out = append(out, inv)
	}
	return out
}

func countCmd(log []invocation, name string) int {
	n := 0
	for _, inv := range log {
		if len(inv.Args) > 0 && inv.Args[0] == name {
			n++
		}
	}
	return n
}

func waitFor(t *testing.T, sup *Supervisor, timeout time.Duration, pred func(Overview) bool) Overview {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last Overview
	for time.Now().Before(deadline) {
		var err error
		last, err = sup.Snapshot(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if pred(last) {
			return last
		}
		time.Sleep(15 * time.Millisecond)
	}
	code, detail := "", ""
	if last.Error != nil {
		code = last.Error.Code
		detail = last.Error.Detail
	}
	t.Fatalf("timeout waiting for overview: state=%s code=%s detail=%s", last.State, code, detail)
	return last
}

func waitState(t *testing.T, sup *Supervisor, state string) Overview {
	t.Helper()
	return waitFor(t, sup, 4*time.Second, func(ov Overview) bool { return ov.State == state })
}

func mustStart(t *testing.T, sup *Supervisor) Overview {
	t.Helper()
	ov, err := sup.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateStarting && ov.State != StateUp && ov.State != StateFailed {
		t.Fatalf("start returned %#v", ov)
	}
	return ov
}

func seedConfig(t *testing.T, repo string, api, gateway, swarm any) {
	t.Helper()
	doc := map[string]any{"API": api, "Gateway": gateway, "Swarm": swarm}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func loadSwarm(t *testing.T, repo string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Swarm []string `json:"Swarm"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Swarm
}

func assertNoLeak(t *testing.T, ov Overview) {
	t.Helper()
	b, err := json.Marshal(ov)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"0.0.0.0", "multiaddr", "Protocols", "/tcp/5001", "/tcp/8081", "\"api\"", "\"gateway\""} {
		if bytes.Contains(b, []byte(forbidden)) {
			t.Fatalf("overview contains %s: %s", forbidden, b)
		}
	}
}

func TestFirstStartPinsLoopbackAndOneDaemon(t *testing.T) {
	other := filepath.Join(t.TempDir(), "dot-ipfs")
	t.Setenv("IPFS_PATH", other)
	sup, repo, logPath := newTestSupervisor(t, nil)

	ov := mustStart(t, sup)
	if ov.State != StateStarting {
		t.Fatalf("handler waited: %#v", ov)
	}
	up := waitState(t, sup, StateUp)
	if up.Error != nil || up.PeerID == nil || *up.PeerID != "12D3KooWFakePeer" {
		t.Fatalf("%#v", up)
	}
	if up.Version == nil || *up.Version != "kubo/0.0.0-fake/" {
		t.Fatalf("version %#v", up.Version)
	}
	if up.RepoPath != repo {
		t.Fatalf("repo %s", up.RepoPath)
	}
	assertNoLeak(t, up)

	log := readLog(t, logPath)
	if countCmd(log, "init") != 1 || countCmd(log, "daemon") != 1 {
		t.Fatalf("log %#v", log)
	}
	for _, inv := range log {
		if inv.IPFSPath != repo {
			t.Fatalf("child IPFS_PATH %s want %s", inv.IPFSPath, repo)
		}
		if inv.IPFSPath == other {
			t.Fatal("child inherited the parent IPFS_PATH")
		}
		for _, arg := range inv.Args {
			if strings.Contains(arg, "unrestricted-api") {
				t.Fatalf("forbidden flag in %v", inv.Args)
			}
		}
		if len(inv.Args) > 0 && inv.Args[0] == "daemon" && len(inv.Args) != 1 {
			t.Fatalf("daemon args %v", inv.Args)
		}
	}
	joined := joinArgs(log)
	wantOrder := []string{
		"init",
		"config --json Addresses",
		"config Addresses.API /ip4/127.0.0.1/tcp/5001",
		`config --json Addresses.Gateway ["/ip4/127.0.0.1/tcp/8081"]`,
		"config --json Addresses",
		"daemon",
	}
	if !containsSequence(joined, wantOrder) {
		t.Fatalf("commands %v", joined)
	}

	b, err := os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		API     any      `json:"API"`
		Gateway []string `json:"Gateway"`
		Swarm   []string `json:"Swarm"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.API.(string); !ok {
		t.Fatalf("API type %T (%v)", doc.API, doc.API)
	}
	if len(doc.Gateway) != 1 || doc.Gateway[0] != "/ip4/127.0.0.1/tcp/8081" {
		t.Fatalf("gateway %v", doc.Gateway)
	}
	if strings.Join(doc.Swarm, ",") != "/ip4/0.0.0.0/tcp/4001,/ip6/::/tcp/4001" {
		t.Fatalf("swarm changed: %v", doc.Swarm)
	}
	info, err := os.Stat(pidPath(repo))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("pid mode %o", info.Mode().Perm())
	}

	again, err := sup.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.State != StateUp {
		t.Fatalf("second start %#v", again)
	}
	if countCmd(readLog(t, logPath), "daemon") != 1 {
		t.Fatal("second start spawned another daemon")
	}
}

func TestFirstStartPreservesAPIArrayAndMovesGateway(t *testing.T) {
	t.Setenv("FAKE_API_TYPE", "array")
	sup, repo, _ := newTestSupervisor(t, func(cfg *Config) {
		cfg.ManagerAddr = "127.0.0.1:8081"
	})
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	b, err := os.ReadFile(filepath.Join(repo, "config"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		API     []string `json:"API"`
		Gateway []string `json:"Gateway"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("API was not an array: %v\n%s", err, b)
	}
	if len(doc.API) != 1 || doc.API[0] != "/ip4/127.0.0.1/tcp/5001" {
		t.Fatalf("api %v", doc.API)
	}
	if len(doc.Gateway) != 1 || doc.Gateway[0] != "/ip4/127.0.0.1/tcp/8082" {
		t.Fatalf("gateway %v", doc.Gateway)
	}
}

func TestExplicitBinaryIgnoresPath(t *testing.T) {
	bin := fakebin.Path(t)
	sup, _, logPath := newTestSupervisor(t, func(cfg *Config) {
		cfg.KuboBin = bin
	})
	t.Setenv("PATH", t.TempDir())
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	if countCmd(readLog(t, logPath), "daemon") != 1 {
		t.Fatal("explicit binary did not run")
	}
}

func TestOverlappingStartsOneChild(t *testing.T) {
	t.Setenv("FAKE_INIT_SLEEP", "800ms")
	sup, _, logPath := newTestSupervisor(t, nil)
	start := time.Now()
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	ovCh := make(chan Overview, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ov, err := sup.Start(context.Background())
			errCh <- err
			ovCh <- ov
		}()
	}
	wg.Wait()
	close(errCh)
	close(ovCh)
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("overlapping starts blocked for %s", time.Since(start))
	}
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	for ov := range ovCh {
		if ov.State != StateStarting {
			t.Fatalf("overlap state %#v", ov)
		}
	}
	if countCmd(readLog(t, logPath), "daemon") != 0 {
		t.Fatal("daemon spawned before init finished")
	}
	waitState(t, sup, StateUp)
	if countCmd(readLog(t, logPath), "daemon") != 1 || countCmd(readLog(t, logPath), "init") != 1 {
		t.Fatalf("log %#v", readLog(t, logPath))
	}
}

func TestRefuseUnsafeAddresses(t *testing.T) {
	swarm := []string{"/ip4/0.0.0.0/tcp/4001", "/ip6/::/tcp/4001"}
	cases := []struct {
		name    string
		api     any
		gateway any
		code    string
	}{
		{"public api", "/ip4/0.0.0.0/tcp/5001", []string{"/ip4/127.0.0.1/tcp/8081"}, CodeAPINotLoopback},
		{"not exactly loopback", "/ip4/127.0.0.2/tcp/5001", []string{"/ip4/127.0.0.1/tcp/8081"}, CodeAPINotLoopback},
		{"public gateway", "/ip4/127.0.0.1/tcp/5001", []string{"/ip4/0.0.0.0/tcp/8081"}, CodeGatewayNotLoopback},
		{"manager port", "/ip4/127.0.0.1/tcp/8080", []string{"/ip4/127.0.0.1/tcp/8081"}, CodePanelPortConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sup, repo, logPath := newTestSupervisor(t, nil)
			seedConfig(t, repo, tc.api, tc.gateway, swarm)
			mustStart(t, sup)
			ov := waitState(t, sup, StateFailed)
			if ov.Error == nil || ov.Error.Code != tc.code {
				t.Fatalf("%#v", ov)
			}
			assertNoLeak(t, ov)
			if countCmd(readLog(t, logPath), "daemon") != 0 || countCmd(readLog(t, logPath), "init") != 0 {
				t.Fatalf("daemon started: %#v", readLog(t, logPath))
			}
			if strings.Join(loadSwarm(t, repo), ",") != strings.Join(swarm, ",") {
				t.Fatalf("swarm changed: %v", loadSwarm(t, repo))
			}
		})
	}
}

func TestLoopbackIPv6Starts(t *testing.T) {
	sup, repo, logPath := newTestSupervisor(t, nil)
	seedConfig(t, repo, "/ip6/::1/tcp/5001", []string{"/ip6/::1/tcp/8081"}, []string{"/ip4/0.0.0.0/tcp/4001"})
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	if countCmd(readLog(t, logPath), "daemon") != 1 {
		t.Fatal("expected daemon")
	}
}

func TestBinaryNotFound(t *testing.T) {
	sup, _, _ := newTestSupervisor(t, nil)
	t.Setenv("PATH", t.TempDir())
	mustStart(t, sup)
	ov := waitState(t, sup, StateFailed)
	if ov.Error == nil || ov.Error.Code != CodeBinaryNotFound {
		t.Fatalf("%#v", ov)
	}
	if !strings.Contains(ov.Error.Detail, "PATH") {
		t.Fatalf("detail %q", ov.Error.Detail)
	}
}

func TestMissingExplicitBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-ipfs")
	sup, _, _ := newTestSupervisor(t, func(cfg *Config) {
		cfg.KuboBin = missing
	})
	mustStart(t, sup)
	ov := waitState(t, sup, StateFailed)
	if ov.Error == nil || ov.Error.Code != CodeBinaryNotFound {
		t.Fatalf("%#v", ov)
	}
}

func TestRepoNotWritable(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(parent, "kubo")
	putFakeOnPath(t)
	sup, err := New(Config{ManagerAddr: "127.0.0.1:8080", RepoPath: repo, InitTimeout: time.Second, ConfigTimeout: time.Second, IDTimeout: time.Second, ReadyBudget: time.Second, ShutdownTimeout: time.Second, StopKillAfter: time.Second, PollInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	mustStart(t, sup)
	ov := waitState(t, sup, StateFailed)
	if ov.Error == nil || ov.Error.Code != CodeRepoNotWritable {
		t.Fatalf("%#v", ov)
	}
}

func TestInitFailedLeavesDirectory(t *testing.T) {
	t.Setenv("FAKE_INIT_FAIL", "1")
	sup, repo, _ := newTestSupervisor(t, nil)
	mustStart(t, sup)
	ov := waitState(t, sup, StateFailed)
	if ov.Error == nil || ov.Error.Code != CodeInitFailed {
		t.Fatalf("%#v", ov)
	}
	info, err := os.Stat(repo)
	if err != nil || !info.IsDir() {
		t.Fatalf("repo removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "config")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial config: %v", err)
	}
}

func TestPortInUse(t *testing.T) {
	t.Setenv("FAKE_DAEMON_EXIT", "listen tcp 127.0.0.1:4001: bind: address already in use")
	sup, _, _ := newTestSupervisor(t, nil)
	mustStart(t, sup)
	ov := waitState(t, sup, StateFailed)
	if ov.Error == nil || ov.Error.Code != CodePortInUse {
		t.Fatalf("%#v", ov)
	}
}

func TestHealthTimeoutKillsChild(t *testing.T) {
	t.Setenv("FAKE_ID_FAIL", "1")
	sup, repo, _ := newTestSupervisor(t, func(cfg *Config) {
		cfg.ReadyBudget = 250 * time.Millisecond
		cfg.IDTimeout = 40 * time.Millisecond
		cfg.PollInterval = 10 * time.Millisecond
	})
	mustStart(t, sup)
	ov := waitState(t, sup, StateFailed)
	if ov.Error == nil || ov.Error.Code != CodeHealthTimeout {
		t.Fatalf("%#v", ov)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !daemonPIDAlive(t, repo) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("daemon still alive after health timeout")
}

func TestGracefulStop(t *testing.T) {
	sup, repo, logPath := newTestSupervisor(t, nil)
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	ov, err := sup.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateStopping {
		t.Fatalf("stop returned %#v", ov)
	}
	down := waitState(t, sup, StateDown)
	if down.Error != nil || down.PeerID != nil || down.Version != nil {
		t.Fatalf("%#v", down)
	}
	if _, err := os.Stat(pidPath(repo)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pid file: %v", err)
	}
	if countCmd(readLog(t, logPath), "shutdown") != 1 {
		t.Fatalf("log %#v", readLog(t, logPath))
	}
}

func TestStopTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows termination is immediate; the SIGTERM-then-SIGKILL deadline is a POSIX behavior")
	}
	t.Setenv("FAKE_SHUTDOWN_IGNORE", "1")
	t.Setenv("FAKE_IGNORE_TERM", "1")
	sup, _, _ := newTestSupervisor(t, func(cfg *Config) {
		cfg.ShutdownTimeout = 150 * time.Millisecond
		cfg.StopKillAfter = 400 * time.Millisecond
	})
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	if _, err := sup.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	down := waitState(t, sup, StateDown)
	if down.Error == nil || down.Error.Code != CodeStopTimeout {
		t.Fatalf("%#v", down)
	}
	if down.PeerID != nil {
		t.Fatal("peer id leaked after forced stop")
	}
}

func TestStopWhenDownStaysDown(t *testing.T) {
	sup, _, _ := newTestSupervisor(t, nil)
	ov, err := sup.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateDown || ov.Error != nil {
		t.Fatalf("%#v", ov)
	}
}

func TestStopIgnoredWhileStarting(t *testing.T) {
	t.Setenv("FAKE_INIT_SLEEP", "400ms")
	sup, _, logPath := newTestSupervisor(t, nil)
	mustStart(t, sup)
	ov, err := sup.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateStarting {
		t.Fatalf("stop changed starting: %#v", ov)
	}
	waitState(t, sup, StateUp)
	if countCmd(readLog(t, logPath), "daemon") != 1 {
		t.Fatal(readLog(t, logPath))
	}
}

func TestStartIgnoredWhileStopping(t *testing.T) {
	t.Setenv("FAKE_SHUTDOWN_SLEEP", "400ms")
	sup, _, logPath := newTestSupervisor(t, nil)
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	stop, err := sup.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stop.State != StateStopping {
		t.Fatalf("%#v", stop)
	}
	again, err := sup.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.State != StateStopping {
		t.Fatalf("start queued during stop: %#v", again)
	}
	waitState(t, sup, StateDown)
	if countCmd(readLog(t, logPath), "daemon") != 1 {
		t.Fatalf("log %#v", readLog(t, logPath))
	}
}

func TestRestartFromUp(t *testing.T) {
	t.Setenv("FAKE_ID_SLEEP", "200ms")
	sup, _, logPath := newTestSupervisor(t, nil)
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	ov, err := sup.Restart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateStopping {
		t.Fatalf("restart returned %#v", ov)
	}
	sawStarting := false
	waitFor(t, sup, 4*time.Second, func(cur Overview) bool {
		if cur.State == StateStarting {
			sawStarting = true
		}
		return cur.State == StateUp && sawStarting
	})
	if countCmd(readLog(t, logPath), "daemon") != 2 {
		t.Fatalf("log %#v", readLog(t, logPath))
	}
}

func TestRestartFromDownStarts(t *testing.T) {
	sup, _, _ := newTestSupervisor(t, nil)
	ov, err := sup.Restart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateStarting {
		t.Fatalf("%#v", ov)
	}
	waitState(t, sup, StateUp)
}

func TestFailedStopKeepsSentenceAndRestartClearsIt(t *testing.T) {
	t.Setenv("FAKE_INIT_FAIL", "1")
	sup, _, _ := newTestSupervisor(t, nil)
	mustStart(t, sup)
	failed := waitState(t, sup, StateFailed)
	ov, err := sup.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateFailed || ov.Error == nil || ov.Error.Code != failed.Error.Code {
		t.Fatalf("%#v", ov)
	}
	t.Setenv("FAKE_INIT_FAIL", "")
	again, err := sup.Restart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.State != StateStarting || again.Error != nil {
		t.Fatalf("error not cleared: %#v", again)
	}
	up := waitState(t, sup, StateUp)
	if up.Error != nil {
		t.Fatalf("%#v", up)
	}
}

func TestForeignDaemonIsLeftRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("foreign-daemon lock check in this test uses flock")
	}
	bin := putFakeOnPath(t)
	repo := t.TempDir()
	seedConfig(t, repo, "/ip4/127.0.0.1/tcp/5001", []string{"/ip4/127.0.0.1/tcp/8081"}, []string{"/ip4/0.0.0.0/tcp/4001"})
	cmd := exec.Command(bin, "daemon")
	cmd.Env = withIPFSPath(os.Environ(), repo)
	cmd.SysProcAttr = daemonSysProcAttr()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = kill(cmd.Process.Pid)
		_ = cmd.Wait()
	})
	waitUntil(t, func() bool {
		held, err := repoLockHeld(repo)
		return err == nil && held
	})
	sup, err := New(Config{
		ManagerAddr: "127.0.0.1:8080", RepoPath: repo, KuboBin: bin,
		InitTimeout: time.Second, ConfigTimeout: time.Second, IDTimeout: time.Second,
		ReadyBudget: time.Second, ShutdownTimeout: time.Second, StopKillAfter: time.Second,
		PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	ov, err := sup.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateFailed || ov.Error == nil || ov.Error.Code != CodeRepoBusy {
		t.Fatalf("%#v", ov)
	}
	for _, act := range []func(context.Context) (Overview, error){sup.Start, sup.Stop, sup.Restart} {
		got, err := act(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got.State != StateFailed || got.Error == nil || got.Error.Code != CodeRepoBusy {
			t.Fatalf("%#v", got)
		}
	}
	if !pidAlive(cmd.Process.Pid) {
		t.Fatal("supervisor signaled a daemon it does not own")
	}
}

func TestAdoptionLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("adoption reads /proc")
	}
	bin := putFakeOnPath(t)
	repo := t.TempDir()
	seedConfig(t, repo, "/ip4/127.0.0.1/tcp/5001", []string{"/ip4/127.0.0.1/tcp/8081"}, []string{"/ip4/0.0.0.0/tcp/4001"})
	cmd := exec.Command(bin, "daemon")
	cmd.Env = withIPFSPath(os.Environ(), repo)
	cmd.SysProcAttr = daemonSysProcAttr()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Reap the daemon when it exits. Otherwise it stays a zombie of this
	// test and still looks alive, so the supervisor cannot observe the stop.
	go func() { _ = cmd.Wait() }()
	t.Cleanup(func() {
		if cmd.Process != nil && pidAlive(cmd.Process.Pid) {
			_ = kill(cmd.Process.Pid)
		}
	})
	waitUntil(t, func() bool { return daemonPIDAlive(t, repo) })
	if err := writePid(repo, cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	sup, err := New(Config{
		ManagerAddr: "127.0.0.1:8080", RepoPath: repo, KuboBin: bin,
		InitTimeout: time.Second, ConfigTimeout: time.Second, IDTimeout: time.Second,
		ReadyBudget: 2 * time.Second, ShutdownTimeout: time.Second, StopKillAfter: time.Second,
		PollInterval: 15 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	up := waitState(t, sup, StateUp)
	if up.PeerID == nil || *up.PeerID != "12D3KooWFakePeer" {
		t.Fatalf("%#v", up)
	}
	if _, err := sup.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitState(t, sup, StateDown)
}

func TestCrashWhileUp(t *testing.T) {
	sup, repo, _ := newTestSupervisor(t, nil)
	mustStart(t, sup)
	waitState(t, sup, StateUp)
	pid, ok := readPid(repo)
	if !ok {
		t.Fatal("missing pid")
	}
	if err := kill(pid); err != nil {
		t.Fatal(err)
	}
	ov := waitState(t, sup, StateFailed)
	if ov.Error == nil || ov.Error.Code != CodeDaemonExited {
		t.Fatalf("%#v", ov)
	}
	assertNoLeak(t, ov)
}

func TestDownOverviewNullsIdentity(t *testing.T) {
	sup, repo, _ := newTestSupervisor(t, nil)
	ov, err := sup.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.State != StateDown || ov.PeerID != nil || ov.Version != nil || ov.Error != nil || ov.RepoPath != repo {
		t.Fatalf("%#v", ov)
	}
	b, err := json.Marshal(ov)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"state", "peerId", "version", "repoPath", "error"} {
		if !bytes.Contains(b, []byte(`"`+key+`"`)) {
			t.Fatalf("missing %s in %s", key, b)
		}
	}
}

func TestCapDetail(t *testing.T) {
	long := strings.Repeat("a", 2500)
	got := capDetail(long)
	if len(got) > detailLimit {
		t.Fatalf("len %d", len(got))
	}
	if !strings.HasSuffix(long, got) {
		t.Fatal("not a tail")
	}
}

func daemonPIDAlive(t *testing.T, repo string) bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "fake-daemon.pid"))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	return pidAlive(pid)
}

func waitUntil(t *testing.T, pred func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if pred() {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("condition not met")
}

func joinArgs(log []invocation) []string {
	out := make([]string, 0, len(log))
	for _, inv := range log {
		out = append(out, strings.Join(inv.Args, " "))
	}
	return out
}

func containsSequence(got, want []string) bool {
	i := 0
	for _, line := range got {
		if i < len(want) && line == want[i] {
			i++
		}
	}
	return i == len(want)
}
