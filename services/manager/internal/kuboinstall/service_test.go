package kuboinstall

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func sumOf(b []byte) string {
	h := sha512.Sum512(b)
	return hex.EncodeToString(h[:])
}

func script(version string, migrateExit int, writeVersion string) string {
	bump := ""
	if writeVersion != "" {
		bump = fmt.Sprintf("printf '%%s\\n' %s > \"$IPFS_PATH/version\"\n", writeVersion)
	}
	return fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\nversion)\n  printf '%%s\\n' %s\n  exit 0\n  ;;\nrepo)\n  %s  exit %d\n  ;;\nesac\nexit 3\n", version, bump, migrateExit)
}

func packTar(files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}
		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write([]byte(body))
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func packZip(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o755)
		w, _ := zw.CreateHeader(h)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	return buf.Bytes()
}

type memNode struct {
	mu     sync.Mutex
	state  string
	starts int
	stops  int
	held   bool
	failOn int
}

func (m *memNode) Snapshot(context.Context) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == "" {
		return "down", "", nil
	}
	return m.state, "", nil
}

func (m *memNode) Stop(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stops++
	m.state = "down"
	return nil
}

func (m *memNode) Start(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.starts++
	if m.failOn > 0 && m.starts == m.failOn {
		m.state = "failed"
		return nil
	}
	m.state = "up"
	return nil
}

func (m *memNode) Hold(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.held = on
}

func (m *memNode) holds() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.held
}

func (m *memNode) counts() (starts, stops int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.starts, m.stops
}

type dist struct {
	srv   *httptest.Server
	files map[string][]byte
	hits  map[string]int
	mu    sync.Mutex
	delay chan struct{}
}

func newDist(t *testing.T) *dist {
	t.Helper()
	d := &dist{files: map[string][]byte{}, hits: map[string]int{}}
	d.srv = httptest.NewServer(http.HandlerFunc(d.serve))
	t.Cleanup(d.srv.Close)
	return d
}

func (d *dist) serve(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	d.hits[r.URL.Path]++
	body, ok := d.files[r.URL.Path]
	delay := d.delay
	d.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if delay != nil && strings.HasSuffix(r.URL.Path, ".tar.gz") {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.WriteHeader(http.StatusOK)
		mid := len(body) / 2
		if mid == 0 {
			mid = len(body)
		}
		_, _ = w.Write(body[:mid])
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-delay
		_, _ = w.Write(body[mid:])
		return
	}
	_, _ = w.Write(body)
}

func (d *dist) add(version string, body []byte, goos, goarch, ext string) {
	name := fmt.Sprintf("kubo_v%s_%s-%s%s", version, goos, goarch, ext)
	sum := sumOf(body)
	sidecar := []byte(sum + "  " + name + "\n")
	d.files["/kubo/v"+version+"/"+name] = body
	d.files["/kubo/v"+version+"/"+name+".sha512"] = sidecar
	d.files["/v"+version+"/"+name] = body
	d.files["/v"+version+"/"+name+".sha512"] = sidecar
}

func (d *dist) setSidecar(origin, version, name, sum string) {
	var path string
	if origin == "dist" {
		path = "/kubo/v" + version + "/" + name + ".sha512"
	} else {
		path = "/v" + version + "/" + name + ".sha512"
	}
	d.files[path] = []byte(sum + "  " + name + "\n")
}

func testService(t *testing.T, d *dist, node Controller, pins map[string]string) (*Service, string, *bytes.Buffer) {
	t.Helper()
	data := t.TempDir()
	var buf bytes.Buffer
	s := New(Options{
		DataDir:     data,
		RepoDir:     t.TempDir(),
		DistBase:    d.srv.URL,
		GitHubBase:  d.srv.URL,
		VersionsURL: d.srv.URL + "/kubo/versions",
		GitHubAPI:   d.srv.URL + "/releases",
		HTTP:        d.srv.Client(),
		GOOS:        "linux",
		GOARCH:      "amd64",
		Pins:        pins,
		Node:        node,
		Logger:      slog.New(slog.NewTextHandler(&buf, nil)),
		LookPath:    func(string) (string, error) { return "", exec.ErrNotFound },
		StartWait:   time.Second,
		StopWait:    time.Second,
	})
	return s, data, &buf
}

func waitJob(t *testing.T, s *Service, id string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := s.Job(id)
		if err != nil {
			t.Fatal(err)
		}
		if job.State != "running" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job still running")
	return Job{}
}

func linuxArchive(version, body string) []byte {
	return packTar(map[string]string{"kubo/ipfs": body, "kubo/install.sh": "#!/bin/sh\necho no\n"})
}

func TestInstallVerifiesThenStarts(t *testing.T) {
	version := Pinned
	body := linuxArchive(version, script(version, 0, ""))
	sum := sumOf(body)
	d := newDist(t)
	d.add(version, body, "linux", "amd64", ".tar.gz")
	d.files["/kubo/versions"] = []byte("v" + version + "\n")
	node := &memNode{state: "down"}
	s, data, log := testService(t, d, node, map[string]string{"linux-amd64": sum})
	job, err := s.Install(Actor{Username: "admin", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, s, job.ID)
	if done.State != "succeeded" || done.Error != "" {
		t.Fatalf("job %+v", done)
	}
	if done.Received == 0 || done.Received != int64(len(body)) {
		t.Fatalf("progress %+v", done)
	}
	if CurrentPath(data) == "" {
		t.Fatal("no current binary")
	}
	starts, _ := node.counts()
	if starts != 1 {
		t.Fatalf("starts %d", starts)
	}
	if node.holds() {
		t.Fatal("hold leaked")
	}
	text := log.String()
	if !strings.Contains(text, "event=kubo_install") || !strings.Contains(text, "result=success") {
		t.Fatalf("audit %s", text)
	}
	if strings.Contains(text, "PrivKey") || strings.Contains(text, "BEGIN") {
		t.Fatalf("audit leaked %s", text)
	}
	st := s.Status(context.Background())
	if st.Source != "managed" || st.Version != version || st.Upgrade {
		t.Fatalf("status %+v", st)
	}
}

func TestSidecarDisagreesWithPin(t *testing.T) {
	body := linuxArchive(Pinned, script(Pinned, 0, ""))
	d := newDist(t)
	d.add(Pinned, body, "linux", "amd64", ".tar.gz")
	node := &memNode{}
	s, data, _ := testService(t, d, node, map[string]string{"linux-amd64": strings.Repeat("ab", 64)})
	job, err := s.Install(Actor{Username: "admin", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, s, job.ID)
	if done.State != "failed" || done.Error != CodeVerify {
		t.Fatalf("job %+v", done)
	}
	if _, err := os.Stat(filepath.Join(data, "kubo-bin", "versions")); !os.IsNotExist(err) {
		t.Fatal("binary extracted after a bad sidecar")
	}
	starts, _ := node.counts()
	if starts != 0 {
		t.Fatal("binary ran")
	}
}

func TestOriginsDisagreeOnUnpinnedVersion(t *testing.T) {
	old := linuxArchive(Pinned, script(Pinned, 0, ""))
	nextBody := linuxArchive("0.44.0", script("0.44.0", 0, ""))
	d := newDist(t)
	d.add(Pinned, old, "linux", "amd64", ".tar.gz")
	d.add("0.44.0", nextBody, "linux", "amd64", ".tar.gz")
	name := "kubo_v0.44.0_linux-amd64.tar.gz"
	d.setSidecar("github", "0.44.0", name, strings.Repeat("cd", 64))
	d.files["/kubo/versions"] = []byte("v0.43.1\nv0.44.0\n")
	s, data, _ := testService(t, d, &memNode{}, map[string]string{"linux-amd64": sumOf(old)})
	job, err := s.Install(Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.State != "succeeded" {
		t.Fatalf("install %+v", done)
	}
	up, err := s.Upgrade(Actor{Username: "admin", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, s, up.ID)
	if done.State != "failed" || done.Error != CodeVerify {
		t.Fatalf("upgrade %+v", done)
	}
	st, err := readState(data)
	if err != nil || st.Current != Pinned {
		t.Fatalf("state %+v %v", st, err)
	}
}

func TestOfflineLeavesBinaryAlone(t *testing.T) {
	s := New(Options{
		DataDir:     t.TempDir(),
		RepoDir:     t.TempDir(),
		DistBase:    "http://127.0.0.1:1",
		GitHubBase:  "http://127.0.0.1:1",
		VersionsURL: "http://127.0.0.1:1/versions",
		GitHubAPI:   "http://127.0.0.1:1/releases",
		HTTP:        &http.Client{Timeout: 300 * time.Millisecond, Transport: &http.Transport{Proxy: nil}},
		GOOS:        "linux",
		GOARCH:      "amd64",
		Pins:        map[string]string{"linux-amd64": strings.Repeat("ab", 64)},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		LookPath:    func(string) (string, error) { return "", exec.ErrNotFound },
	})
	job, err := s.Install(Actor{Username: "admin", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, s, job.ID)
	if done.State != "failed" || done.Error != CodeOffline {
		t.Fatalf("job %+v", done)
	}
	if s.Status(context.Background()).Source != "missing" {
		t.Fatal("source changed")
	}
}

func TestFlagBinaryIsNotReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipfs")
	if err := os.WriteFile(path, []byte("expert"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := newDist(t)
	body := linuxArchive(Pinned, script(Pinned, 0, ""))
	d.add(Pinned, body, "linux", "amd64", ".tar.gz")
	s, data, _ := testService(t, d, &memNode{}, map[string]string{"linux-amd64": sumOf(body)})
	s.explicit = path
	if _, err := s.Install(Actor{Username: "admin", IP: "127.0.0.1"}); err == nil || err.Error() != CodeExternal {
		t.Fatalf("err %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "expert" {
		t.Fatalf("binary changed %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(data, "kubo-bin", "versions")); !os.IsNotExist(err) {
		t.Fatal("managed binary written")
	}
}

func TestRollbackWhenRepoVersionUnchanged(t *testing.T) {
	old := linuxArchive(Pinned, script(Pinned, 0, ""))
	next := linuxArchive("0.44.0", script("0.44.0", 1, ""))
	d := newDist(t)
	d.add(Pinned, old, "linux", "amd64", ".tar.gz")
	d.add("0.44.0", next, "linux", "amd64", ".tar.gz")
	d.files["/kubo/versions"] = []byte("v0.43.1\nv0.44.0\n")
	node := &memNode{}
	s, data, log := testService(t, d, node, map[string]string{"linux-amd64": sumOf(old)})
	if err := os.WriteFile(filepath.Join(s.repoDir, "version"), []byte("17\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	job, err := s.Install(Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.State != "succeeded" {
		t.Fatalf("install %+v", done)
	}
	up, err := s.Upgrade(Actor{Username: "admin", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, s, up.ID)
	if done.State != "rolled_back" {
		t.Fatalf("upgrade %+v log %s", done, log.String())
	}
	st, err := readState(data)
	if err != nil || st.Current != Pinned {
		t.Fatalf("state %+v %v", st, err)
	}
	ver, _ := os.ReadFile(filepath.Join(s.repoDir, "version"))
	if strings.TrimSpace(string(ver)) != "17" {
		t.Fatalf("repo version %s", ver)
	}
	deadline := time.Now().Add(time.Second)
	for node.holds() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if node.holds() {
		t.Fatal("hold leaked")
	}
	if !strings.Contains(log.String(), "result=rolled_back") || !strings.Contains(log.String(), "previous="+Pinned) {
		t.Fatalf("audit %s", log.String())
	}
}

func TestNoRollbackAfterRepoMoves(t *testing.T) {
	old := linuxArchive(Pinned, script(Pinned, 0, ""))
	next := linuxArchive("0.44.0", script("0.44.0", 1, "18"))
	d := newDist(t)
	d.add(Pinned, old, "linux", "amd64", ".tar.gz")
	d.add("0.44.0", next, "linux", "amd64", ".tar.gz")
	d.files["/kubo/versions"] = []byte("v0.43.1\nv0.44.0\n")
	node := &memNode{}
	s, data, _ := testService(t, d, node, map[string]string{"linux-amd64": sumOf(old)})
	if err := os.WriteFile(filepath.Join(s.repoDir, "version"), []byte("17\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	job, err := s.Install(Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.State != "succeeded" {
		t.Fatalf("install %+v", done)
	}
	starts, _ := node.counts()
	up, err := s.Upgrade(Actor{})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, s, up.ID)
	if done.State != "failed" || done.Error != CodeMigrate {
		t.Fatalf("upgrade %+v", done)
	}
	st, err := readState(data)
	if err != nil || st.Current != "0.44.0" {
		t.Fatalf("state %+v %v", st, err)
	}
	after, _ := node.counts()
	if after != starts {
		t.Fatalf("old binary was started, starts %d -> %d", starts, after)
	}
}

func TestUnsupportedPlatformDoesNotDownload(t *testing.T) {
	d := newDist(t)
	s, _, _ := testService(t, d, &memNode{}, nil)
	s.goarch = "386"
	if _, err := s.Install(Actor{}); err == nil || err.Error() != CodeUnsupported {
		t.Fatalf("err %v", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.hits) != 0 {
		t.Fatalf("hits %v", d.hits)
	}
}

func TestArchiveRejectsEscape(t *testing.T) {
	body := packTar(map[string]string{"../evil": "x"})
	err := extractBinary(writeTemp(t, body), t.TempDir(), false, "ipfs")
	if err == nil {
		t.Fatal("escape accepted")
	}
}

func TestZipExtractsWindowsBinary(t *testing.T) {
	body := packZip(map[string]string{"kubo/ipfs.exe": "not-a-real-binary"})
	dest := t.TempDir()
	if err := extractBinary(writeTemp(t, body), dest, true, "ipfs.exe"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "ipfs.exe"))
	if err != nil || string(got) != "not-a-real-binary" {
		t.Fatalf("got %q %v", got, err)
	}
}

func writeTemp(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPickRange(t *testing.T) {
	latest, upstream := pick([]string{"v0.43.0", "v0.43.1", "v0.44.0-rc1", "v0.44.2", "v0.45.0"})
	if latest != "0.44.2" || upstream != "0.45.0" {
		t.Fatalf("latest %s upstream %s", latest, upstream)
	}
}

func TestDownloadProgress(t *testing.T) {
	body := linuxArchive(Pinned, script(Pinned, 0, ""))
	sum := sumOf(body)
	d := newDist(t)
	d.add(Pinned, body, "linux", "amd64", ".tar.gz")
	d.delay = make(chan struct{})
	s, _, _ := testService(t, d, &memNode{}, map[string]string{"linux-amd64": sum})
	job, err := s.Install(Actor{})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	saw := false
	for time.Now().Before(deadline) {
		got, err := s.Job(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State == "running" && got.Phase == "downloading" && got.Received > 0 && got.Received < got.Total {
			saw = true
			break
		}
		if got.State != "running" {
			t.Fatalf("finished early %+v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(d.delay)
	if !saw {
		t.Fatal("no partial progress")
	}
	if done := waitJob(t, s, job.ID); done.State != "succeeded" {
		t.Fatalf("job %+v", done)
	}
}
