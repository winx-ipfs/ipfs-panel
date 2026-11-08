package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type cmdOut struct {
	stdout []byte
	stderr string
	err    error
}

func withIPFSPath(env []string, repo string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, "IPFS_PATH=") {
			continue
		}
		out = append(out, entry)
	}
	out = append(out, "IPFS_PATH="+repo)
	return out
}

// run executes bin with args. It does not use a shell.
// The child IPFS_PATH is repo, even when the parent environment already has one.
func run(ctx context.Context, bin string, args []string, repo string) cmdOut {
	if err := ctx.Err(); err != nil {
		return cmdOut{err: err}
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = withIPFSPath(os.Environ(), repo)
	cmd.WaitDelay = 2 * time.Second
	var stdout headBuf
	stdout.max = 1 << 20
	stderr := newTail(detailLimit)
	cmd.Stdout = &stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	return cmdOut{stdout: stdout.buf, stderr: stderr.String(), err: err}
}

func resolveBin(explicit string) (string, *codedError) {
	if explicit != "" {
		info, err := os.Stat(explicit)
		if err != nil {
			return "", codeErr(CodeBinaryNotFound, err.Error())
		}
		if info.IsDir() {
			return "", codeErr(CodeBinaryNotFound, explicit+" is a directory")
		}
		return explicit, nil
	}
	path, err := exec.LookPath("ipfs")
	if err != nil {
		return "", codeErr(CodeBinaryNotFound, "ipfs not found on PATH")
	}
	return path, nil
}

func parseID(stdout []byte) (peerID, version string, err error) {
	var doc struct {
		ID           string `json:"ID"`
		AgentVersion string `json:"AgentVersion"`
	}
	if err := json.Unmarshal(stdout, &doc); err != nil {
		return "", "", err
	}
	if doc.ID == "" || doc.AgentVersion == "" {
		return "", "", errors.New("ipfs id missing ID or AgentVersion")
	}
	return doc.ID, doc.AgentVersion, nil
}

func spawnDetail(err error) *codedError {
	if err == nil {
		return codeErr(CodeUnknown, "spawn failed")
	}
	if errors.Is(err, exec.ErrNotFound) || os.IsNotExist(err) || os.IsPermission(err) {
		return codeErr(CodeBinaryNotFound, err.Error())
	}
	return codeErr(CodeUnknown, err.Error())
}

func ensureRepo(path string) *codedError {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return codeErr(CodeRepoNotWritable, err.Error())
	}
	f, err := os.CreateTemp(path, ".write-check-*")
	if err != nil {
		return codeErr(CodeRepoNotWritable, err.Error())
	}
	name := f.Name()
	if cerr := f.Close(); cerr != nil {
		_ = os.Remove(name)
		return codeErr(CodeRepoNotWritable, cerr.Error())
	}
	if err := os.Remove(name); err != nil {
		return codeErr(CodeRepoNotWritable, err.Error())
	}
	return nil
}

func fmtJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
