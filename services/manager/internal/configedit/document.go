package configedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/scenes"
)

// MaxDocument is the largest config text the editor accepts. It matches the
// cap used when the manager reads the Kubo config file.
const MaxDocument = 2 << 20

func parseText(text string) (map[string]any, error) {
	if len(text) > MaxDocument {
		return nil, code(http.StatusRequestEntityTooLarge, "too_large")
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, code(http.StatusBadRequest, "invalid_json")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, code(http.StatusBadRequest, "invalid_json")
	}
	doc, ok := value.(map[string]any)
	if !ok || doc == nil {
		return nil, code(http.StatusBadRequest, "not_object")
	}
	return doc, nil
}

func redactShow(raw []byte) (string, map[string]any, error) {
	if len(raw) > MaxDocument {
		return "", nil, code(http.StatusBadGateway, "kubo_rejected")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return "", nil, code(http.StatusBadGateway, "kubo_rejected")
	}
	doc, ok := value.(map[string]any)
	if !ok || doc == nil {
		return "", nil, code(http.StatusBadGateway, "kubo_rejected")
	}
	stripSecrets(doc)
	text, err := pretty(doc)
	if err != nil {
		return "", nil, code(http.StatusServiceUnavailable, "unavailable")
	}
	return text, doc, nil
}

func pretty(value any) (string, error) {
	body, err := prettyBytes(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func prettyBytes(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rejectSecrets(doc map[string]any) error {
	if ident, ok := child(doc, "Identity"); ok {
		if value, ok := field(ident, "PrivKey"); ok && !blankSecret(value) {
			return code(http.StatusBadRequest, "privkey_locked")
		}
	}
	if api, ok := child(doc, "API"); ok {
		if _, ok := field(api, "Authorizations"); ok {
			return code(http.StatusBadRequest, "secret_locked")
		}
	}
	if pinning, ok := child(doc, "Pinning"); ok {
		if services, ok := child(pinning, "RemoteServices"); ok {
			for _, item := range services {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				api, ok := child(entry, "API")
				if !ok {
					continue
				}
				if value, ok := field(api, "Key"); ok && !blankSecret(value) {
					return code(http.StatusBadRequest, "secret_locked")
				}
			}
		}
	}
	return nil
}

func stripSecrets(doc map[string]any) {
	if ident, ok := child(doc, "Identity"); ok {
		drop(ident, "PrivKey")
	}
	if api, ok := child(doc, "API"); ok {
		drop(api, "Authorizations")
	}
	if pinning, ok := child(doc, "Pinning"); ok {
		if services, ok := child(pinning, "RemoteServices"); ok {
			for _, item := range services {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if api, ok := child(entry, "API"); ok {
					drop(api, "Key")
				}
			}
		}
	}
}

func blankSecret(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && text == ""
}

func sameAPI(current, next map[string]any) error {
	left, err := apiAddresses(current)
	if err != nil {
		return code(http.StatusConflict, "api_locked")
	}
	right, err := apiAddresses(next)
	if err != nil {
		return code(http.StatusConflict, "api_locked")
	}
	if len(left) != len(right) {
		return code(http.StatusConflict, "api_locked")
	}
	for i := range left {
		if left[i] != right[i] {
			return code(http.StatusConflict, "api_locked")
		}
	}
	return nil
}

func apiAddresses(doc map[string]any) ([]string, error) {
	addresses, ok := child(doc, "Addresses")
	if !ok {
		return nil, errors.New("api")
	}
	value, ok := field(addresses, "API")
	if !ok {
		return nil, errors.New("api")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	list, err := node.ParseAddresses(raw)
	if err != nil || len(list) == 0 {
		return nil, errors.New("api")
	}
	return list, nil
}

func samePeer(current, next map[string]any) error {
	left, ok := peerID(current)
	if !ok {
		return code(http.StatusConflict, "identity_locked")
	}
	right, ok := peerID(next)
	if !ok || left != right {
		return code(http.StatusConflict, "identity_locked")
	}
	return nil
}

func peerID(doc map[string]any) (string, bool) {
	ident, ok := child(doc, "Identity")
	if !ok {
		return "", false
	}
	value, ok := field(ident, "PeerID")
	text, isString := value.(string)
	if !ok || !isString || strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}

func assess(body []byte, managerAddr string) error {
	err := node.AssessConfig(body, managerAddr)
	if err == nil {
		return nil
	}
	var cfg *node.ConfigError
	if errors.As(err, &cfg) && cfg.Code != "" {
		return code(http.StatusConflict, cfg.Code)
	}
	return code(http.StatusServiceUnavailable, "unavailable")
}

func sceneDiverged(before, after map[string]any, scene string) bool {
	if scene == "" {
		return false
	}
	for _, key := range scenes.ManagedKeys() {
		if canonAt(before, key) != canonAt(after, key) {
			return true
		}
	}
	return false
}

func canonAt(doc map[string]any, path string) string {
	var current any = doc
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return "null"
		}
		next, ok := field(object, part)
		if !ok {
			return "null"
		}
		current = next
	}
	raw, err := json.Marshal(current)
	if err != nil {
		return "null"
	}
	return string(raw)
}

func child(object map[string]any, name string) (map[string]any, bool) {
	value, ok := field(object, name)
	if !ok || value == nil {
		return nil, false
	}
	child, ok := value.(map[string]any)
	return child, ok
}

func field(object map[string]any, name string) (any, bool) {
	for key, value := range object {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	return nil, false
}

func drop(object map[string]any, name string) {
	for key := range object {
		if strings.EqualFold(key, name) {
			delete(object, key)
		}
	}
}

func kuboDetail(body []byte) string {
	var wrap struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return ""
	}
	return limitDetail(wrap.Message)
}

func limitDetail(text string) string {
	text = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, text)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if utf8.RuneCountInString(text) > 240 {
		text = string([]rune(text)[:240])
	}
	return text
}
