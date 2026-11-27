package access

import (
	"fmt"
	"io"
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
	// http.Dir cleans the URL path and refuses a name that leaves the root.
	// The request path is never joined and passed to os.Open from here.
	name := path.Clean("/" + r.URL.Path)
	if file, info, ok := openFile(http.Dir(s.root), name); ok {
		defer func() { _ = file.Close() }()
		if strings.HasPrefix(name, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
		return
	}
	if path.Ext(name) != "" {
		http.NotFound(w, r)
		return
	}
	index, err := os.Open(s.index)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = index.Close() }()
	info, err := index.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", info.ModTime(), index)
}

// openFile returns a regular file inside dir. Directories and missing names
// are not files. dir is http.Dir so the URL path cannot escape it.
func openFile(dir http.Dir, name string) (io.ReadSeekCloser, os.FileInfo, bool) {
	if strings.ContainsRune(name, filepath.Separator) && filepath.Separator != '/' {
		return nil, nil, false
	}
	opened, err := dir.Open(name)
	if err != nil {
		return nil, nil, false
	}
	info, err := opened.Stat()
	if err != nil || info.IsDir() {
		_ = opened.Close()
		return nil, nil, false
	}
	file, ok := opened.(io.ReadSeekCloser)
	if !ok {
		_ = opened.Close()
		return nil, nil, false
	}
	return file, info, true
}
