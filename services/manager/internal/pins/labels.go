package pins

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/atomicfile"
)

const labelFileName = "pin-names.json"

// Labels is the optional CID to name map in the panel data directory.
// A corrupt file is kept as-is and not overwritten.
type Labels struct {
	mu     sync.Mutex
	path   string
	frozen bool
	names  map[string]string
}

// OpenLabels reads path, or keeps names in memory when path is empty.
// path is the data directory, not the file.
func OpenLabels(dataDir string) *Labels {
	s := &Labels{names: map[string]string{}}
	if dataDir == "" {
		return s
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		s.frozen = true
		return s
	}
	_ = os.Chmod(dataDir, 0o700)
	s.path = filepath.Join(dataDir, labelFileName)
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s
	}
	if err != nil {
		s.frozen = true
		return s
	}
	_ = os.Chmod(s.path, 0o600)
	var file struct {
		Names map[string]string `json:"names"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		s.frozen = true
		return s
	}
	if file.Names == nil {
		file.Names = map[string]string{}
	}
	for cid, name := range file.Names {
		if _, err := ParseTarget(cid); err != nil || ValidName(name) != nil || name == "" {
			continue
		}
		s.names[cid] = name
	}
	return s
}

// Get returns a stored label.
func (s *Labels) Get(cid string) string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.names[cid]
}

// Set stores a label. A frozen store returns an error and does not write.
func (s *Labels) Set(cid, name string) error {
	if s == nil || cid == "" || name == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.frozen {
		return &OpError{Code: "unavailable"}
	}
	s.names[cid] = name
	return s.flushLocked()
}

// Delete removes a label.
func (s *Labels) Delete(cid string) error {
	if s == nil || cid == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.names[cid]; !ok {
		return nil
	}
	if s.frozen {
		return &OpError{Code: "unavailable"}
	}
	delete(s.names, cid)
	return s.flushLocked()
}

// Apply fills empty names from the label file. Kubo names are left alone.
func (s *Labels) Apply(in []Pin) []Pin {
	if s == nil {
		out := make([]Pin, len(in))
		copy(out, in)
		return out
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Pin, len(in))
	copy(out, in)
	for i := range out {
		if out[i].Name == "" {
			if name := s.names[out[i].CID]; name != "" {
				out[i].Name = name
			}
		}
	}
	return out
}

func (s *Labels) flushLocked() error {
	if s.path == "" {
		return nil
	}
	body, err := json.MarshalIndent(struct {
		Names map[string]string `json:"names"`
	}{Names: s.names}, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return atomicfile.Write(s.path, body, 0o600)
}
