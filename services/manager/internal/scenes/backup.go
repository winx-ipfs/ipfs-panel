package scenes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/atomicfile"
)

const maxBackups = 5

type backup struct {
	Scene  string                     `json:"scene"`
	At     time.Time                  `json:"at"`
	Values map[string]json.RawMessage `json:"values"`
}

func backupDir(dataDir string) string {
	return filepath.Join(dataDir, "scene-backups")
}

func writeBackup(dataDir string, item backup) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("data directory is empty")
	}
	dir := backupDir(dataDir)
	body, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return "", err
	}
	body = append(body, '\n')
	name := fmt.Sprintf("%020d-%s.json", item.At.UnixNano(), item.Scene)
	path := filepath.Join(dir, name)
	if err := atomicfile.Write(path, body, 0o600); err != nil {
		return "", err
	}
	if err := trimBackups(dir); err != nil {
		return "", err
	}
	return path, nil
}

func trimBackups(dir string) error {
	names, err := backupNames(dir)
	if err != nil {
		return err
	}
	if len(names) <= maxBackups {
		return nil
	}
	for _, name := range names[:len(names)-maxBackups] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func hasBackup(dataDir string) (bool, error) {
	if strings.TrimSpace(dataDir) == "" {
		return false, nil
	}
	names, err := backupNames(backupDir(dataDir))
	if err != nil {
		return false, err
	}
	return len(names) > 0, nil
}

func latestBackup(dataDir string) (backup, string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return backup{}, "", nil
	}
	dir := backupDir(dataDir)
	names, err := backupNames(dir)
	if err != nil || len(names) == 0 {
		return backup{}, "", err
	}
	path := filepath.Join(dir, names[len(names)-1])
	f, err := os.Open(path)
	if err != nil {
		return backup{}, "", err
	}
	defer func() { _ = f.Close() }()
	var item backup
	if err := json.NewDecoder(f).Decode(&item); err != nil {
		return backup{}, "", err
	}
	return item, path, nil
}

func backupNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}
