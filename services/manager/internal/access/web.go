package access

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Web serves a built panel from dir. An empty dir serves nothing.
// The directory must contain index.html. Paths cannot escape it.
func Web(dir string) (http.Handler, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("web directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("web directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("web directory %s: not a directory", dir)
	}
	index := filepath.Join(root, "index.html")
	if _, err := os.Stat(index); err != nil {
		return nil, fmt.Errorf("web directory %s: index.html: %w", dir, err)
	}
	return spa{root: root, index: index}, nil
}

type spa struct {
	root  string
	index string
}

func (s spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := path.Clean("/" + r.URL.Path)
	target := filepath.Join(s.root, filepath.FromSlash(rel))
	rootWithSep := s.root + string(os.PathSeparator)
	if target != s.root && !strings.HasPrefix(target, rootWithSep) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(target)
	if err == nil && !info.IsDir() {
		if strings.HasPrefix(rel, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFile(w, r, target)
		return
	}
	if path.Ext(rel) != "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, s.index)
}
