package pins

import (
	"math/big"
	"strings"
	"unicode"
)

const (
	maxTargetBytes = 2048
	maxNameBytes   = 255
	maxDigest      = 128

	b58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b32Alphabet = "abcdefghijklmnopqrstuvwxyz234567"
)

// ParseTarget accepts a CID or an /ipfs/ path and returns the value to send
// to Kubo. Surrounding whitespace is ignored. Base32 and base16 are lowercased.
func ParseTarget(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || len(s) > maxTargetBytes {
		return "", invalidCID()
	}
	if strings.HasPrefix(s, "/ipfs/") {
		return parseIPFSPath(s)
	}
	if strings.Contains(s, "/") || strings.ContainsAny(s, " \t") {
		return "", invalidCID()
	}
	return normalizeCID(s)
}

// CIDInfo is the part of a content ID the Explore page can say out loud.
type CIDInfo struct {
	Version int
	Codec   uint64
	Hash    uint64
}

// Inspect reads the version, codec, and multihash code of a CID or /ipfs/ path.
func Inspect(raw string) (CIDInfo, error) {
	target, err := ParseTarget(raw)
	if err != nil {
		return CIDInfo{}, err
	}
	bin, err := cidBytes(RootCID(target))
	if err != nil {
		return CIDInfo{}, err
	}
	if cidV0(bin) {
		return CIDInfo{Version: 0, Codec: 0x70, Hash: 0x12}, nil
	}
	ver, n, err := readUvarint(bin)
	if err != nil || n == 0 {
		return CIDInfo{}, invalidCID()
	}
	bin = bin[n:]
	codec, n, err := readUvarint(bin)
	if err != nil || n == 0 {
		return CIDInfo{}, invalidCID()
	}
	bin = bin[n:]
	hash, n, err := readUvarint(bin)
	if err != nil || n == 0 {
		return CIDInfo{}, invalidCID()
	}
	return CIDInfo{Version: int(ver), Codec: codec, Hash: hash}, nil
}

func cidBytes(cid string) ([]byte, error) {
	if strings.HasPrefix(cid, "Qm") {
		raw, err := decodeBase58(cid)
		if err != nil || !cidV0(raw) {
			return nil, invalidCID()
		}
		return raw, nil
	}
	if len(cid) < 2 {
		return nil, invalidCID()
	}
	var raw []byte
	var err error
	switch cid[0] {
	case 'b':
		raw, err = decodeBase32(cid[1:])
	case 'z':
		raw, err = decodeBase58(cid[1:])
	case 'f':
		raw, err = decodeBase16(cid[1:])
	default:
		return nil, invalidCID()
	}
	if err != nil || !cidV1(raw) {
		return nil, invalidCID()
	}
	return raw, nil
}

// RootCID is the CID at the start of a target. A bare CID is itself.
func RootCID(target string) string {
	if strings.HasPrefix(target, "/ipfs/") {
		rest := strings.TrimPrefix(target, "/ipfs/")
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			return rest[:i]
		}
		return rest
	}
	return target
}

// ValidName accepts an empty label, or a short label that is not a path.
func ValidName(name string) error {
	if name == "" {
		return nil
	}
	if strings.TrimSpace(name) != name || len(name) > maxNameBytes {
		return invalidName()
	}
	if strings.ContainsAny(name, "/\\\"") {
		return invalidName()
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || unicode.Is(unicode.Cc, r) {
			return invalidName()
		}
	}
	return nil
}

func parseIPFSPath(s string) (string, error) {
	if strings.Contains(s, "\\") || strings.ContainsRune(s, 0) {
		return "", invalidCID()
	}
	rest := strings.TrimPrefix(s, "/ipfs/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", invalidCID()
	}
	cid, err := normalizeCID(parts[0])
	if err != nil {
		return "", err
	}
	out := "/ipfs/" + cid
	for _, part := range parts[1:] {
		if part == "" || part == "." || part == ".." || len(part) > maxNameBytes {
			return "", invalidCID()
		}
		for _, r := range part {
			if r < 0x20 || r == 0x7f {
				return "", invalidCID()
			}
		}
		out += "/" + part
	}
	if len(out) > maxTargetBytes {
		return "", invalidCID()
	}
	return out, nil
}

func normalizeCID(s string) (string, error) {
	if s == "" || strings.ContainsRune(s, 0) {
		return "", invalidCID()
	}
	if strings.HasPrefix(s, "Qm") {
		raw, err := decodeBase58(s)
		if err != nil || !cidV0(raw) {
			return "", invalidCID()
		}
		return s, nil
	}
	if len(s) < 2 {
		return "", invalidCID()
	}
	prefix := s[0]
	body := s[1:]
	var raw []byte
	var err error
	switch prefix {
	case 'b', 'B':
		raw, err = decodeBase32(body)
		prefix = 'b'
		body = strings.ToLower(body)
	case 'z':
		raw, err = decodeBase58(body)
	case 'f', 'F':
		raw, err = decodeBase16(body)
		prefix = 'f'
		body = strings.ToLower(body)
	default:
		return "", invalidCID()
	}
	if err != nil || !cidV1(raw) {
		return "", invalidCID()
	}
	return string(prefix) + body, nil
}

func cidV0(raw []byte) bool {
	return len(raw) == 34 && raw[0] == 0x12 && raw[1] == 0x20
}

func cidV1(raw []byte) bool {
	if len(raw) < 4 || len(raw) > 256 {
		return false
	}
	ver, n, err := readUvarint(raw)
	if err != nil || ver != 1 || n == 0 {
		return false
	}
	raw = raw[n:]
	if _, n, err = readUvarint(raw); err != nil || n == 0 {
		return false
	}
	raw = raw[n:]
	if _, n, err = readUvarint(raw); err != nil || n == 0 {
		return false
	}
	raw = raw[n:]
	ln, n, err := readUvarint(raw)
	if err != nil || n == 0 || ln == 0 || ln > maxDigest {
		return false
	}
	raw = raw[n:]
	return int(ln) == len(raw)
}

func readUvarint(b []byte) (uint64, int, error) {
	var x uint64
	var s uint
	for i, c := range b {
		if i > 9 {
			return 0, 0, invalidCID()
		}
		if c < 0x80 {
			if i == 9 && c > 1 {
				return 0, 0, invalidCID()
			}
			return x | uint64(c)<<s, i + 1, nil
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, 0, invalidCID()
}

func decodeBase58(s string) ([]byte, error) {
	if s == "" {
		return nil, invalidCID()
	}
	n := new(big.Int)
	for i := 0; i < len(s); i++ {
		c := strings.IndexByte(b58Alphabet, s[i])
		if c < 0 {
			return nil, invalidCID()
		}
		n.Mul(n, big.NewInt(58))
		n.Add(n, big.NewInt(int64(c)))
	}
	decoded := n.Bytes()
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	out := make([]byte, zeros+len(decoded))
	copy(out[zeros:], decoded)
	return out, nil
}

func decodeBase32(s string) ([]byte, error) {
	s = strings.ToLower(s)
	if s == "" || strings.Contains(s, "=") {
		return nil, invalidCID()
	}
	var buf uint64
	var bits uint
	out := make([]byte, 0, len(s)*5/8+1)
	for i := 0; i < len(s); i++ {
		c := strings.IndexByte(b32Alphabet, s[i])
		if c < 0 {
			return nil, invalidCID()
		}
		buf = (buf << 5) | uint64(c)
		bits += 5
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(buf>>bits))
			buf &= (1 << bits) - 1
		}
	}
	if bits > 0 && buf != 0 {
		return nil, invalidCID()
	}
	return out, nil
}

func decodeBase16(s string) ([]byte, error) {
	s = strings.ToLower(s)
	if len(s) == 0 || len(s)%2 != 0 {
		return nil, invalidCID()
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi := fromHex(s[i*2])
		lo := fromHex(s[i*2+1])
		if hi < 0 || lo < 0 {
			return nil, invalidCID()
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, nil
}

func fromHex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return -1
	}
}
