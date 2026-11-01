package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoopbackEndpoints(t *testing.T) {
	dir := writeRepoConfig(t, `{
		"Addresses": {
			"API": "/ip4/127.0.0.1/tcp/5001",
			"Gateway": ["/ip4/127.0.0.1/tcp/8081"]
		}
	}`)
	api, gw, err := LoopbackEndpoints(dir)
	if err != nil {
		t.Fatal(err)
	}
	if api != "http://127.0.0.1:5001" || gw != "http://127.0.0.1:8081" {
		t.Fatalf("api %s gateway %s", api, gw)
	}

	v6 := writeRepoConfig(t, `{
		"Addresses": {
			"API": "/ip6/::1/tcp/5001",
			"Gateway": ["/ip6/::1/tcp/8082"]
		}
	}`)
	api, gw, err = LoopbackEndpoints(v6)
	if err != nil {
		t.Fatal(err)
	}
	if api != "http://[::1]:5001" || gw != "http://[::1]:8082" {
		t.Fatalf("v6 api %s gateway %s", api, gw)
	}

	public := writeRepoConfig(t, `{
		"Addresses": {
			"API": "/ip4/0.0.0.0/tcp/5001",
			"Gateway": ["/ip4/127.0.0.1/tcp/8081"]
		}
	}`)
	if _, _, err := LoopbackEndpoints(public); err == nil {
		t.Fatal("public api accepted")
	}

	noGW := writeRepoConfig(t, `{"Addresses":{"API":"/ip4/127.0.0.1/tcp/5001"}}`)
	api, gw, err = LoopbackEndpoints(noGW)
	if err != nil || api != "http://127.0.0.1:5001" || gw != "" {
		t.Fatalf("no gateway %s %s %v", api, gw, err)
	}
}

func TestCachedWhileDown(t *testing.T) {
	sup, err := New(Config{RepoPath: t.TempDir(), ManagerAddr: "127.0.0.1:8080"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	ov, err := sup.Cached(context.Background())
	if err != nil || ov.State != StateDown {
		t.Fatalf("%#v %v", ov, err)
	}
	max, op, transfer := sup.FileLimits()
	if max != 512<<20 || op != 20*time.Second || transfer != 10*time.Minute {
		t.Fatalf("limits %d %s %s", max, op, transfer)
	}
}

func writeRepoConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}
