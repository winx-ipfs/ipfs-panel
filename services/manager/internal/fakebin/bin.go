// Package fakebin builds the test double for the ipfs binary.
package fakebin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	once sync.Once
	bin  string
	err  error
)

// Path returns the fake ipfs binary, built once per test process.
func Path(tb testing.TB) string {
	tb.Helper()
	once.Do(build)
	if err != nil {
		tb.Fatalf("build fake ipfs: %v", err)
	}
	return bin
}

func build() {
	dir, mkErr := os.MkdirTemp("", "ipfs-panel-fakeipfs")
	if mkErr != nil {
		err = mkErr
		return
	}
	out := filepath.Join(dir, "ipfs")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "github.com/winx-ipfs/ipfs-panel/services/manager/internal/node/fakeipfs")
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		err = fmt.Errorf("%w\n%s", runErr, output)
		return
	}
	bin = out
}
