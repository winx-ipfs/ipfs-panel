package node

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/files"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/pins"
)

// TestRealKuboOptional runs the manager against a released Kubo binary
// as a subprocess. It skips unless IPFS_PANEL_INTEGRATION=1 and ipfs is
// on PATH. CI sets both. A local run without them still skips.
func TestRealKuboOptional(t *testing.T) {
	if os.Getenv("IPFS_PANEL_INTEGRATION") != "1" {
		t.Skip("set IPFS_PANEL_INTEGRATION=1 to run against a real ipfs binary")
	}
	bin, err := exec.LookPath("ipfs")
	if err != nil {
		t.Skip("ipfs not on PATH")
	}
	if strings.Contains(bin, "ipfs-panel-fakeipfs") {
		t.Skip("PATH is the test double, not Kubo")
	}
	repo := t.TempDir()
	sup, err := New(Config{
		ManagerAddr: "127.0.0.1:8080",
		RepoPath:    repo,
		KuboBin:     bin,
		ReadyBudget: 90 * time.Second,
		InitTimeout: 90 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	if _, err := sup.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	up := waitFor(t, sup, 2*time.Minute, func(ov Overview) bool {
		return ov.State == StateUp || ov.State == StateFailed
	})
	if up.State != StateUp || up.PeerID == nil || *up.PeerID == "" || up.Version == nil {
		code, detail := "", ""
		if up.Error != nil {
			code, detail = up.Error.Code, up.Error.Detail
		}
		t.Fatalf("state %s code %s detail %s", up.State, code, detail)
	}
	t.Logf("agent %s", *up.Version)
	release := os.Getenv("IPFS_PANEL_KUBO_VERSION")
	if release != "" && !strings.Contains(*up.Version, "kubo/"+release) {
		t.Fatalf("agent %q, pinned release %s", *up.Version, release)
	}
	if release != "" && release != "0.43.1" {
		t.Fatalf("no settled expectations for Kubo %s; update TestRealKuboOptional in the pin pull request", release)
	}
	apiBase, _, err := LoopbackEndpoints(repo)
	if err != nil {
		t.Fatal(err)
	}
	client, err := files.New(files.Options{
		BaseURL:         apiBase,
		OpTimeout:       30 * time.Second,
		TransferTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	roundTripMFS(t, client, bin, repo, apiBase, release)
	if _, err := sup.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	down := waitFor(t, sup, 20*time.Second, func(ov Overview) bool { return ov.State == StateDown })
	if down.Error != nil {
		t.Fatalf("%#v", down)
	}
	assertStoppedNode(t, apiBase)
}

func roundTripMFS(t *testing.T, client *files.Client, bin, repo, apiBase, release string) {
	t.Helper()
	ctx := context.Background()
	const root = "/panel-integration"
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = client.Remove(cctx, root)
	})
	if err := client.Mkdir(ctx, root); err != nil {
		t.Fatal(err)
	}
	// files/write is multipart on an io.Pipe, so Content-Length stays unset.
	// A Kubo that rejects that body fails here.
	entry, err := client.Write(ctx, root+"/hello.txt", strings.NewReader("hello"), false)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Size != 5 || entry.CID == "" || entry.Type != "file" {
		t.Fatalf("write %#v", entry)
	}
	if err := client.Mkdir(ctx, root+"/sub"); err != nil {
		t.Fatal(err)
	}
	nested, err := client.Write(ctx, root+"/sub/nested.txt", strings.NewReader("nested"), false)
	if err != nil {
		t.Fatal(err)
	}
	if nested.Type != "file" || nested.CID == "" {
		t.Fatalf("nested %#v", nested)
	}
	dirStat, err := client.Stat(ctx, root+"/sub")
	if err != nil {
		t.Fatal(err)
	}
	fileStat, err := client.Stat(ctx, root+"/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if dirStat.Type != "directory" || fileStat.Type != "file" || dirStat.Type == fileStat.Type {
		t.Fatalf("stat dir %#v file %#v", dirStat, fileStat)
	}
	if fileStat.CID == "" || fileStat.CID != entry.CID || fileStat.Size != 5 {
		t.Fatalf("file stat %#v write %#v", fileStat, entry)
	}
	const touched = int64(1730000000)
	touchMTime(t, bin, repo, root+"/hello.txt", touched)
	fileStat, err = client.Stat(ctx, root+"/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	wantMTime := time.Unix(touched, 0).UTC()
	if !fileStat.Modified.Equal(wantMTime) {
		t.Fatalf("files/stat mtime %s", fileStat.Modified)
	}
	list, err := client.List(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	var sawFile, sawDir bool
	for _, item := range list {
		switch item.Name {
		case "hello.txt":
			sawFile = item.Type == "file" && item.Size == 5 && item.CID == fileStat.CID && item.Modified.Equal(wantMTime)
		case "sub":
			sawDir = item.Type == "directory" && item.CID != ""
		}
	}
	if !sawFile || !sawDir {
		t.Fatalf("files/ls long %#v", list)
	}
	assertOverMaxBytes(t, apiBase, root)
	roundTripPin(t, apiBase, entry.CID, nested.CID, dirStat.CID, release)
	body, err := client.Open(ctx, root+"/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("read %q", got)
	}
	if err := client.Copy(ctx, root+"/hello.txt", root+"/copy.txt"); err != nil {
		t.Fatal(err)
	}
	if err := client.Move(ctx, root+"/copy.txt", root+"/moved.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Rename(ctx, root+"/moved.txt", "final.txt"); err != nil {
		t.Fatal(err)
	}
	if err := client.Remove(ctx, root+"/final.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Stat(ctx, root+"/final.txt"); files.CodeOf(err) != "not_found" {
		t.Fatalf("stat after remove: %v", err)
	}
}

func assertOverMaxBytes(t *testing.T, apiBase, root string) {
	t.Helper()
	limited, err := files.New(files.Options{
		BaseURL:         apiBase,
		MaxBytes:        4,
		OpTimeout:       30 * time.Second,
		TransferTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	path := root + "/oversize.txt"
	_, err = limited.Write(ctx, path, strings.NewReader("hello"), false)
	if files.CodeOf(err) != "too_large" {
		t.Fatalf("over MaxBytes: %v", err)
	}
	if _, err := limited.Stat(ctx, path); files.CodeOf(err) != "not_found" {
		t.Fatalf("partial write left the path: %v", err)
	}
	t.Log("over MaxBytes returned too_large and the path is gone")
}

func touchMTime(t *testing.T, bin, repo, mfsPath string, unix int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "files", "touch", "--mtime="+strconv.FormatInt(unix, 10), mfsPath)
	cmd.Env = withIPFSPath(os.Environ(), repo)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("files touch: %v\n%s", err, out)
	}
}

func roundTripPin(t *testing.T, apiBase, fileCID, nestedCID, dirCID, release string) {
	t.Helper()
	pc, err := pins.New(pins.Options{
		BaseURL:         apiBase,
		OpTimeout:       30 * time.Second,
		TransferTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	support := &pins.Support{}
	var progress []int
	pinned, err := pc.Add(ctx, fileCID, "panel-integration", true, func(n int) {
		progress = append(progress, n)
	}, support)
	if err != nil {
		t.Fatal(err)
	}
	if pinned == "" {
		t.Fatal("pin/add returned an empty cid")
	}
	t.Logf("pin/add progress %v", progress)
	for _, n := range progress {
		if n <= 0 {
			t.Fatalf("progress %d", n)
		}
	}
	if release == "0.43.1" && len(progress) == 0 {
		t.Fatal("v0.43.1 pin/add --progress emits a Progress value before Pins")
	}
	t.Cleanup(func() {
		cctx, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		_ = pc.Remove(cctx, pinned, "recursive")
	})
	list, err := pc.List(ctx, support)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var gotName string
	for _, pin := range list {
		if pin.CID == pinned && pin.Type == "recursive" {
			found = true
			gotName = pin.Name
		}
	}
	if !found {
		t.Fatalf("pin list missing %s in %#v", pinned, list)
	}
	switch support.NamesFrom() {
	case "kubo":
		if gotName != "panel-integration" {
			t.Fatalf("names=true accepted but name is %q", gotName)
		}
		t.Log("pin/ls names=true accepted")
	case "panel":
		t.Log("pin/ls names= fallback taken")
		if release == "0.43.1" {
			t.Fatal("v0.43.1 accepts pin/ls names=true; the 400 fallback must not run")
		}
	default:
		t.Fatalf("namesFrom %q", support.NamesFrom())
	}
	sized := []pins.Pin{{CID: pinned, Type: "recursive"}}
	pc.FillSizes(ctx, sized)
	if sized[0].Size != nil && *sized[0].Size <= 0 {
		t.Fatalf("object/stat size %d", *sized[0].Size)
	}
	if sized[0].Size == nil {
		t.Log("object/stat miss; size left unset")
	} else {
		t.Logf("object/stat CumulativeSize %d", *sized[0].Size)
	}
	if release == "0.43.1" && sized[0].Size != nil {
		t.Fatalf("v0.43.1 removed object/stat; FillSizes invented a size %d", *sized[0].Size)
	}
	dirPin, err := pc.Add(ctx, dirCID, "panel-dir", true, nil, support)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cctx, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		_ = pc.Remove(cctx, dirPin, "recursive")
	})
	err = pc.Remove(ctx, nestedCID, "direct")
	if pins.CodeOf(err) != "indirect" {
		t.Fatalf("pin/rm indirect: %v", err)
	}
	t.Log("pin/rm on an indirect pin returned indirect")
}

func assertStoppedNode(t *testing.T, apiBase string) {
	t.Helper()
	client, err := files.New(files.Options{
		BaseURL:         apiBase,
		OpTimeout:       5 * time.Second,
		TransferTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_, err = client.Stat(ctx, "/panel-integration")
	if files.CodeOf(err) != "node_not_running" {
		t.Fatalf("stopped node: %v", err)
	}
	t.Log("stopped node returned node_not_running")
}
