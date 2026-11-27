package kuboinstall

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

func withRepo(env []string, repo string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, "IPFS_PATH=") {
			continue
		}
		out = append(out, entry)
	}
	return append(out, "IPFS_PATH="+repo)
}

// binaryVersion runs the extracted file only after its archive hashed.
// `version --number` is what current Kubo prints. An older binary that
// rejects the flag falls through to `version`.
func binaryVersion(ctx context.Context, bin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "version", "--number").Output()
	if v, ok := cleanVersion(string(out)); err == nil && ok {
		return v, nil
	}
	out, err = exec.CommandContext(ctx, bin, "version").Output()
	if v, ok := cleanVersion(string(out)); ok {
		return v, nil
	}
	if err == nil {
		err = errCode(CodeVerify)
	}
	return "", err
}

func cleanVersion(out string) (string, bool) {
	text := strings.TrimSpace(out)
	if text == "" {
		return "", false
	}
	fields := strings.Fields(text)
	last := strings.TrimPrefix(fields[len(fields)-1], "v")
	if _, ok := parseVer(last); !ok {
		return "", false
	}
	return last, true
}

func runMigrate(ctx context.Context, bin, repo string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "repo", "migrate")
	cmd.Env = withRepo(os.Environ(), repo)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
