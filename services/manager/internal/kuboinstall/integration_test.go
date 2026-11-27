package kuboinstall

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRealKuboInstall downloads the pinned release through this service,
// checks the compiled hash, and runs the extracted binary's own version
// and init. It does not start a daemon; the other TestRealKubo tests do.
func TestRealKuboInstall(t *testing.T) {
	if os.Getenv("IPFS_PANEL_INTEGRATION") != "1" {
		t.Skip("set IPFS_PANEL_INTEGRATION=1 to install the pinned Kubo release")
	}
	if !supportedPlatform(goos(), goarch()) {
		t.Fatalf("integration host %s/%s is not one of the six targets", goos(), goarch())
	}
	data := t.TempDir()
	repo := t.TempDir()
	s := New(Options{
		DataDir:   data,
		RepoDir:   repo,
		LookPath:  func(string) (string, error) { return "", os.ErrNotExist },
		StartWait: time.Second,
		StopWait:  time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	job, err := s.Install(Actor{Username: "ci", IP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Minute)
	var done Job
	for {
		if time.Now().After(deadline) || ctx.Err() != nil {
			t.Fatal("install did not finish")
		}
		done, err = s.Job(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if done.State != "running" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if done.State != "succeeded" {
		t.Fatalf("install %+v", done)
	}
	st, err := readState(data)
	if err != nil {
		t.Fatal(err)
	}
	want := PinnedDigest(goos(), goarch())
	if st.SHA512 != want || want == "" {
		t.Fatalf("sha512 %s want %s", st.SHA512, want)
	}
	bin := CurrentPath(data)
	out, err := exec.Command(bin, "version", "--number").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != Pinned {
		t.Fatalf("version %q", out)
	}
	cmd := exec.Command(bin, "init")
	cmd.Env = withRepo(os.Environ(), repo)
	cmd.Dir = repo
	initOut, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init: %v\n%s", err, initOut)
	}
	if _, err := os.Stat(filepath.Join(repo, "config")); err != nil {
		t.Fatal(err)
	}
}

func goos() string   { return New(Options{}).goos }
func goarch() string { return New(Options{}).goarch }
