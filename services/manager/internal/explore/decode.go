package explore

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/pins"
)

const (
	maxLinks = 200
	maxText  = 4096
	maxData  = 8192
)

// Link is one edge out of the block on screen.
type Link struct {
	Name string
	CID  string
	Size *int64
}

// Block is the one node Explore shows.
type Block struct {
	Target         string
	CID            string
	Version        int
	Codec          string
	Hash           string
	Kind           string
	Size           *int64
	Focus          string
	Text           string
	Data           json.RawMessage
	DataOmitted    bool
	Links          []Link
	LinksTruncated bool
}

func codecName(code uint64) string {
	switch code {
	case 0x55:
		return "raw"
	case 0x70:
		return "dag-pb"
	case 0x71:
		return "dag-cbor"
	case 0x0129:
		return "dag-json"
	case 0x0200:
		return "json"
	case 0x51:
		return "cbor"
	default:
		return "unknown"
	}
}

func hashName(code uint64) string {
	switch code {
	case 0x12:
		return "sha2-256"
	case 0x00:
		return "identity"
	default:
		return ""
	}
}

func interpret(info pins.CIDInfo, body []byte, focus string) (Block, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return Block{}, &OpError{Code: "kubo_error"}
	}
	codec := codecName(info.Codec)
	block := Block{
		CID:     "",
		Version: info.Version,
		Codec:   codec,
		Hash:    hashName(info.Hash),
		Focus:   focus,
		Links:   []Link{},
	}
	switch codec {
	case "raw":
		raw, ok := asBytes(value)
		if !ok {
			block.Kind = "bytes"
			return block, nil
		}
		block.Kind = "bytes"
		n := int64(len(raw))
		block.Size = &n
		block.Text = previewText(raw)
		return block, nil
	case "dag-pb":
		return fillPB(block, value), nil
	default:
		if codec == "unknown" {
			if _, ok := pbLinks(value); ok && hasPBShape(value) {
				return fillPB(block, value), nil
			}
		}
		block.Kind = "data"
		block.Links, block.LinksTruncated = capLinks(walkLinks(value))
		raw, err := json.Marshal(value)
		if err != nil {
			return Block{}, &OpError{Code: "kubo_error"}
		}
		if len(raw) <= maxData {
			block.Data = raw
		} else {
			block.DataOmitted = true
		}
		return block, nil
	}
}

func fillPB(block Block, value any) Block {
	links, _ := pbLinks(value)
	block.Links, block.LinksTruncated = capLinks(links)
	raw, ok := asBytes(field(value, "Data"))
	if !ok {
		block.Kind = "block"
		return block
	}
	kind, payload, size, unix := readUnixFS(raw)
	if !unix {
		block.Kind = "block"
		return block
	}
	block.Kind = kind
	block.Size = size
	if kind == "folder" && size != nil && *size == 0 {
		block.Size = nil
	}
	if kind == "file" || kind == "symlink" {
		block.Text = previewText(payload)
	}
	return block
}

func hasPBShape(value any) bool {
	m, ok := value.(map[string]any)
	if !ok {
		return false
	}
	_, data := m["Data"]
	_, links := m["Links"]
	return data || links
}

func field(value any, name string) any {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return m[name]
}

func pbLinks(value any) ([]Link, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	raw, exists := m["Links"]
	if !exists || raw == nil {
		return []Link{}, true
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	links := make([]Link, 0, len(arr))
	for _, item := range arr {
		lm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cid, ok := asCID(lm["Hash"])
		if !ok {
			continue
		}
		name, _ := lm["Name"].(string)
		link := Link{Name: name, CID: cid}
		if size, ok := asInt(lm["Tsize"]); ok {
			link.Size = &size
		}
		links = append(links, link)
	}
	return links, true
}

func walkLinks(value any) []Link {
	var links []Link
	var walk func(v any, path string)
	walk = func(v any, path string) {
		if cid, ok := asCID(v); ok {
			links = append(links, Link{Name: strings.TrimPrefix(path, "/"), CID: cid})
			return
		}
		if _, ok := asBytes(v); ok {
			return
		}
		switch n := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(n))
			for key := range n {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				next := key
				if path != "" {
					next = path + "/" + key
				}
				walk(n[key], next)
			}
		case []any:
			for i, item := range n {
				next := strconv.Itoa(i)
				if path != "" {
					next = path + "/" + next
				}
				walk(item, next)
			}
		}
	}
	walk(value, "")
	if links == nil {
		return []Link{}
	}
	return links
}

func capLinks(links []Link) ([]Link, bool) {
	if links == nil {
		return []Link{}, false
	}
	if len(links) <= maxLinks {
		return links, false
	}
	return links[:maxLinks], true
}

func asCID(value any) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok || len(m) != 1 {
		return "", false
	}
	s, ok := m["/"].(string)
	if !ok || s == "" {
		return "", false
	}
	if _, err := pins.ParseTarget(s); err != nil {
		return "", false
	}
	return s, true
}

func asBytes(value any) ([]byte, bool) {
	m, ok := value.(map[string]any)
	if !ok || len(m) != 1 {
		return nil, false
	}
	inner, ok := m["/"].(map[string]any)
	if !ok || len(inner) != 1 {
		return nil, false
	}
	s, ok := inner["bytes"].(string)
	if !ok {
		return nil, false
	}
	for _, enc := range []*base64.Encoding{
		base64.RawStdEncoding,
		base64.StdEncoding,
		base64.RawURLEncoding,
		base64.URLEncoding,
	} {
		decoded, err := enc.DecodeString(s)
		if err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func asInt(value any) (int64, bool) {
	switch n := value.(type) {
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return i, true
		}
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return int64(f), true
	case float64:
		return int64(n), true
	default:
		return 0, false
	}
}

func previewText(raw []byte) string {
	if len(raw) == 0 || len(raw) > maxText {
		return ""
	}
	if bytes.Contains(raw, []byte{0}) || !utf8.Valid(raw) {
		return ""
	}
	return string(raw)
}

// readUnixFS reads the UnixFS Data protobuf enough to name the block.
// Field 1 is the type. The directory example in Kubo's changelog is the
// bytes base64 "CAE", field 1 set to 1.
func readUnixFS(data []byte) (kind string, payload []byte, size *int64, ok bool) {
	if len(data) == 0 {
		return "", nil, nil, false
	}
	i := 0
	got := false
	var typ uint64
	var fileSize int64
	var hasSize bool
	for i < len(data) {
		key, n := uvarint(data[i:])
		if n <= 0 {
			return "", nil, nil, false
		}
		i += n
		fieldNum := int(key >> 3)
		wire := int(key & 7)
		if fieldNum == 0 {
			return "", nil, nil, false
		}
		switch wire {
		case 0:
			v, n := uvarint(data[i:])
			if n <= 0 {
				return "", nil, nil, false
			}
			i += n
			if fieldNum == 1 {
				typ = v
				got = true
			}
			if fieldNum == 3 {
				fileSize = int64(v)
				hasSize = true
			}
		case 1:
			if i+8 > len(data) {
				return "", nil, nil, false
			}
			i += 8
		case 2:
			ln, n := uvarint(data[i:])
			if n <= 0 {
				return "", nil, nil, false
			}
			i += n
			if ln > uint64(len(data)-i) {
				return "", nil, nil, false
			}
			chunk := data[i : i+int(ln)]
			i += int(ln)
			if fieldNum == 2 {
				payload = append([]byte(nil), chunk...)
			}
		case 5:
			if i+4 > len(data) {
				return "", nil, nil, false
			}
			i += 4
		default:
			return "", nil, nil, false
		}
	}
	if !got {
		return "", nil, nil, false
	}
	var sz *int64
	if hasSize {
		v := fileSize
		sz = &v
	}
	switch typ {
	case 1, 5:
		return "folder", payload, sz, true
	case 2:
		return "file", payload, sz, true
	case 4:
		return "symlink", payload, sz, true
	default:
		return "block", payload, sz, true
	}
}

func uvarint(b []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, c := range b {
		if i > 9 {
			return 0, 0
		}
		if c < 0x80 {
			if i == 9 && c > 1 {
				return 0, 0
			}
			return x | uint64(c)<<s, i + 1
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, 0
}
