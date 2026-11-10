package scenes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/atomicfile"
)

const maxConfig = 2 << 20

func configPath(repo string) string { return filepath.Join(repo, "config") }

func loadOptional(repo string) (map[string]any, bool, error) {
	doc, err := load(repo)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return doc, true, nil
}

func load(repo string) (map[string]any, error) {
	f, err := os.Open(configPath(repo))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxConfig+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfig {
		return nil, fmt.Errorf("kubo config is too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("kubo config: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func save(repo string, body []byte) error {
	return atomicfile.Write(configPath(repo), body, 0o600)
}

func marshalDoc(doc map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func snapshot(doc map[string]any) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(managedKeys))
	for _, key := range managedKeys {
		raw, ok, err := rawAt(doc, key)
		if err != nil {
			return nil, err
		}
		if !ok {
			out[key] = json.RawMessage("null")
			continue
		}
		out[key] = raw
	}
	return out, nil
}

func restore(doc map[string]any, values map[string]json.RawMessage) error {
	for _, key := range managedKeys {
		raw, ok := values[key]
		if !ok || len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			removeKey(doc, key)
			continue
		}
		if err := setRaw(doc, key, string(raw)); err != nil {
			return err
		}
	}
	return nil
}

func sameKey(doc map[string]any, key, want string) (bool, error) {
	raw, ok, err := rawAt(doc, key)
	if err != nil {
		return false, err
	}
	if key == "Addresses.Gateway" {
		if !ok {
			return false, nil
		}
		return sameGateway(string(raw), want), nil
	}
	current := string(raw)
	if !ok {
		def, has := implicitDefault(key)
		if !has {
			return false, nil
		}
		current = def
	}
	return canonical(current) == canonical(want), nil
}

func implicitDefault(key string) (string, bool) {
	switch key {
	case "Routing.Type":
		return `"auto"`, true
	case "AutoNAT.ServiceMode":
		return `"enabled"`, true
	case "Swarm.RelayService.Enabled", "Provide.Enabled", "Gateway.ExposeRoutingAPI":
		return `true`, true
	case "Swarm.ConnMgr.Type":
		return `"basic"`, true
	case "Swarm.ConnMgr.LowWater":
		return `32`, true
	case "Swarm.ConnMgr.HighWater":
		return `96`, true
	case "Swarm.ConnMgr.GracePeriod":
		return `"20s"`, true
	case "Provide.Strategy":
		return `"all"`, true
	case "Gateway.NoFetch", "Gateway.NoDNSLink":
		return `false`, true
	default:
		return "", false
	}
}

func canonical(raw string) string {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return string(b)
}

func sameGateway(current, want string) bool {
	got, ok := gatewayList(current)
	exp, ok2 := gatewayList(want)
	if !ok || !ok2 || len(got) != len(exp) {
		return false
	}
	seen := map[string]int{}
	for _, addr := range got {
		seen[addr]++
	}
	for _, addr := range exp {
		if seen[addr] == 0 {
			return false
		}
		seen[addr]--
	}
	return true
}

func gatewayList(raw string) ([]string, bool) {
	var one string
	if err := json.Unmarshal([]byte(raw), &one); err == nil && one != "" {
		return []string{one}, true
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil || len(list) == 0 {
		return nil, false
	}
	return list, true
}

func rawAt(doc map[string]any, path string) (json.RawMessage, bool, error) {
	v, ok := lookup(doc, path)
	if !ok {
		return nil, false, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func setRaw(doc map[string]any, path, raw string) error {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	return assign(doc, path, v)
}

func lookup(doc map[string]any, path string) (any, bool) {
	cur := any(doc)
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[part]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func assign(doc map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	cur := doc
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part]
		if !ok {
			child := map[string]any{}
			cur[part] = child
			cur = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("%s is not an object", part)
		}
		cur = child
	}
	cur[parts[len(parts)-1]] = value
	return nil
}

func removeKey(doc map[string]any, path string) {
	parts := strings.Split(path, ".")
	cur := doc
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part]
		if !ok {
			return
		}
		child, ok := next.(map[string]any)
		if !ok {
			return
		}
		cur = child
	}
	delete(cur, parts[len(parts)-1])
}
