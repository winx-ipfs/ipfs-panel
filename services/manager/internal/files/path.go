package files

import "strings"

const (
	maxPathBytes = 4096
	maxNameBytes = 255
)

// Clean checks an absolute mutable-file path.
// It rejects traversal instead of resolving it.
func Clean(p string) (string, error) {
	if p == "" || len(p) > maxPathBytes || !strings.HasPrefix(p, "/") {
		return "", invalidPath()
	}
	if strings.ContainsRune(p, 0) || strings.Contains(p, "\\") || strings.Contains(p, "\"") {
		return "", invalidPath()
	}
	if p == "/" {
		return "/", nil
	}
	raw := strings.TrimRight(p, "/")
	if raw == "" {
		return "/", nil
	}
	parts := strings.Split(raw, "/")
	clean := make([]string, 0, len(parts))
	for i, part := range parts {
		if i == 0 {
			if part != "" {
				return "", invalidPath()
			}
			continue
		}
		if !validSegment(part) {
			return "", invalidPath()
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		return "", invalidPath()
	}
	return "/" + strings.Join(clean, "/"), nil
}

// Segment reports whether name can be one path segment.
func Segment(name string) bool {
	return validSegment(name)
}

// Join builds dir/name after checking both.
func Join(dir, name string) (string, error) {
	parent, err := Clean(dir)
	if err != nil {
		return "", err
	}
	if !validSegment(name) {
		return "", invalidPath()
	}
	if parent == "/" {
		return "/" + name, nil
	}
	return parent + "/" + name, nil
}

// Parent returns the folder that holds p. The parent of / is /.
func Parent(p string) (string, error) {
	clean, err := Clean(p)
	if err != nil {
		return "", err
	}
	if clean == "/" {
		return "/", nil
	}
	i := strings.LastIndex(clean, "/")
	if i <= 0 {
		return "/", nil
	}
	return clean[:i], nil
}

// Base returns the last segment. The root has none.
func Base(p string) (string, error) {
	clean, err := Clean(p)
	if err != nil || clean == "/" {
		return "", invalidPath()
	}
	return clean[strings.LastIndex(clean, "/")+1:], nil
}

// Inside reports whether child is parent or a path under parent.
func Inside(parent, child string) bool {
	if parent == "/" {
		return true
	}
	return child == parent || strings.HasPrefix(child, parent+"/")
}

func validSegment(seg string) bool {
	if seg == "" || seg == "." || seg == ".." || len(seg) > maxNameBytes {
		return false
	}
	if strings.TrimSpace(seg) != seg {
		return false
	}
	for _, r := range seg {
		if r < 0x20 || r == 0x7f || r == '\\' || r == '/' || r == '"' {
			return false
		}
	}
	return true
}

func invalidPath() error {
	return &OpError{Code: "invalid_path"}
}
