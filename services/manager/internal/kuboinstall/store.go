package kuboinstall

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/atomicfile"
)

type diskState struct {
	Current        string `json:"current"`
	Previous       string `json:"previous"`
	SHA512         string `json:"sha512"`
	PreviousSHA512 string `json:"previousSha512,omitempty"`
}

func rootDir(dataDir string) string {
	return filepath.Join(dataDir, "kubo-bin")
}

func statePath(dataDir string) string {
	return filepath.Join(rootDir(dataDir), "state.json")
}

func versionDir(dataDir, version string) string {
	return filepath.Join(rootDir(dataDir), "versions", version)
}

func stagingDir(dataDir string) string {
	return filepath.Join(rootDir(dataDir), "staging")
}

func readState(dataDir string) (diskState, error) {
	b, err := os.ReadFile(statePath(dataDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return diskState{}, nil
		}
		return diskState{}, err
	}
	var st diskState
	if err := json.Unmarshal(b, &st); err != nil {
		return diskState{}, err
	}
	return st, nil
}

func writeState(dataDir string, st diskState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return atomicfile.Write(statePath(dataDir), b, 0o600)
}

// CurrentPath is the managed binary the supervisor should run, or "" when
// this data directory has none. It follows runtime.GOOS because that is the
// process that would execute the file.
func CurrentPath(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return ""
	}
	st, err := readState(dataDir)
	if err != nil || st.Current == "" {
		return ""
	}
	path := filepath.Join(versionDir(dataDir, st.Current), exeName(runtime.GOOS))
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func repoVersion(repo string) (int, bool) {
	if strings.TrimSpace(repo) == "" {
		return 0, false
	}
	b, err := os.ReadFile(filepath.Join(repo, "version"))
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, false
	}
	return n, true
}
