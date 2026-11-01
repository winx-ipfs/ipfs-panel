package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const fileName = "auth.json"

// AuthPath is the credential file inside the manager data directory.
func AuthPath(dataDir string) string {
	return filepath.Join(dataDir, fileName)
}

// OutsideRepo reports an error when file is inside the Kubo repo.
// An empty repo skips the check.
func OutsideRepo(file, repo string) error {
	if strings.TrimSpace(repo) == "" {
		return nil
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRepo, absFile)
	if err != nil {
		return err
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("auth file %s must not live in the Kubo repo", file)
	}
	return nil
}

type adminRecord struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Store is the 0600 credential file. It never contains a session.
type Store struct {
	mu   sync.Mutex
	path string
}

func openStore(dataDir, repo string) (*Store, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("data dir is empty")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dataDir, 0o700); err != nil {
		return nil, err
	}
	path := AuthPath(dataDir)
	if err := OutsideRepo(path, repo); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

func (s *Store) Load() (adminRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return adminRecord{}, false, nil
	}
	if err != nil {
		return adminRecord{}, false, err
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return adminRecord{}, false, err
	}
	var rec adminRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return adminRecord{}, false, fmt.Errorf("auth file: %w", err)
	}
	if rec.Username == "" || rec.PasswordHash == "" {
		return adminRecord{}, false, errors.New("auth file is incomplete")
	}
	return rec, true, nil
}

func (s *Store) Save(rec adminRecord) error {
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeAtomic(s.path, body)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".auth-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmpName, path); err2 != nil {
			return err2
		}
	}
	cleanup = false
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := dirFile.Sync()
	closeErr := dirFile.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
