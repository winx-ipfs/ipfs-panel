package peers

import (
	"math/big"
	"net"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxAddressBytes = 2048
	maxDigest       = 128

	b58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b32Alphabet = "abcdefghijklmnopqrstuvwxyz234567"
)

// ParseAddress checks one libp2p multiaddr and returns the address to dial
// plus the peer id at the end of it. The address is not resolved.
func ParseAddress(raw string) (string, string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || len(s) > maxAddressBytes || !strings.HasPrefix(s, "/") {
		return "", "", invalidAddress()
	}
	for _, r := range s {
		if r > 127 || unicode.IsSpace(r) || r < 0x20 || r == 0x7f || strings.ContainsRune(`\?#@`, r) {
			return "", "", invalidAddress()
		}
	}
	parts := strings.Split(s, "/")
	if len(parts) < 3 || parts[0] != "" {
		return "", "", invalidAddress()
	}
	i := 1
	endedWithPeer := false
	var peerID string
	var peerAt int
	for i < len(parts) {
		name := parts[i]
		if name == "" {
			return "", "", invalidAddress()
		}
		if bare(name) {
			endedWithPeer = false
			i++
			continue
		}
		if !valued(name) || i+1 >= len(parts) {
			return "", "", invalidAddress()
		}
		value := parts[i+1]
		if !validValue(name, value) {
			return "", "", invalidAddress()
		}
		if name == "p2p" || name == "ipfs" {
			endedWithPeer = true
			peerID = canonicalPeer(value)
			peerAt = i + 1
		} else {
			endedWithPeer = false
		}
		i += 2
	}
	if !endedWithPeer || peerID == "" {
		return "", "", invalidAddress()
	}
	parts[peerAt] = peerID
	return strings.Join(parts, "/"), peerID, nil
}

func bare(name string) bool {
	switch name {
	case "quic", "quic-v1", "webtransport", "webrtc", "webrtc-direct", "tls", "ws", "wss", "p2p-circuit":
		return true
	default:
		return false
	}
}

func valued(name string) bool {
	switch name {
	case "ip4", "ip6", "tcp", "udp", "dns", "dns4", "dns6", "dnsaddr", "sni", "certhash", "p2p", "ipfs":
		return true
	default:
		return false
	}
}

func validValue(name, value string) bool {
	switch name {
	case "ip4":
		ip := net.ParseIP(value)
		return ip != nil && ip.To4() != nil
	case "ip6":
		if strings.Contains(value, "%") {
			return false
		}
		ip := net.ParseIP(value)
		return ip != nil && ip.To4() == nil
	case "tcp", "udp":
		return validPort(value)
	case "dns", "dns4", "dns6", "dnsaddr", "sni":
		return validHostname(value)
	case "certhash":
		return validToken(value)
	case "p2p", "ipfs":
		return validPeerID(value)
	default:
		return false
	}
}

func validPort(value string) bool {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(value)
	return err == nil && n >= 1 && n <= 65535
}

func validHostname(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	labels := strings.Split(value, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			case c == '-' && i > 0 && i < len(label)-1:
			default:
				return false
			}
		}
	}
	return true
}

func validToken(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '+' || c == '/' || c == '=' || c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

func canonicalPeer(s string) string {
	if strings.HasPrefix(s, "Qm") || strings.HasPrefix(s, "1") || strings.HasPrefix(s, "z") {
		return s
	}
	if len(s) > 1 && (s[0] == 'b' || s[0] == 'B') {
		return "b" + strings.ToLower(s[1:])
	}
	if len(s) > 1 && (s[0] == 'f' || s[0] == 'F') {
		return "f" + strings.ToLower(s[1:])
	}
	return s
}

func validPeerID(s string) bool {
	if s == "" || len(s) > 256 {
		return false
	}
	if strings.HasPrefix(s, "Qm") {
		raw, err := decodeBase58(s)
		return err == nil && cidV0(raw)
	}
	if strings.HasPrefix(s, "1") {
		raw, err := decodeBase58(s)
		return err == nil && identityOrSHA2(raw)
	}
	raw, ok := cidBytes(s)
	if !ok {
		return false
	}
	ver, rest, ok := takeUvarint(raw)
	if !ok || ver != 1 {
		return false
	}
	codec, rest, ok := takeUvarint(rest)
	if !ok || codec != 0x72 {
		return false
	}
	return identityOrSHA2(rest)
}

func identityOrSHA2(raw []byte) bool {
	code, rest, ok := takeUvarint(raw)
	if !ok {
		return false
	}
	ln, digest, ok := takeUvarint(rest)
	if !ok || ln == 0 || ln > maxDigest || int(ln) != len(digest) {
		return false
	}
	if code == 0x00 {
		return true
	}
	return code == 0x12 && ln == 32
}

func cidV0(raw []byte) bool {
	return len(raw) == 34 && raw[0] == 0x12 && raw[1] == 0x20
}

func cidBytes(s string) ([]byte, bool) {
	if len(s) < 2 {
		return nil, false
	}
	var raw []byte
	var err error
	switch s[0] {
	case 'b', 'B':
		raw, err = decodeBase32(s[1:])
	case 'z':
		raw, err = decodeBase58(s[1:])
	case 'f', 'F':
		raw, err = decodeBase16(s[1:])
	default:
		return nil, false
	}
	return raw, err == nil
}

func takeUvarint(b []byte) (uint64, []byte, bool) {
	var x uint64
	var s uint
	for i, c := range b {
		if i > 9 {
			return 0, nil, false
		}
		if c < 0x80 {
			if i == 9 && c > 1 {
				return 0, nil, false
			}
			return x | uint64(c)<<s, b[i+1:], true
		}
		x |= uint64(c&0x7f) << s
		s += 7
	}
	return 0, nil, false
}

func decodeBase58(s string) ([]byte, error) {
	if s == "" {
		return nil, invalidAddress()
	}
	n := new(big.Int)
	for i := 0; i < len(s); i++ {
		c := strings.IndexByte(b58Alphabet, s[i])
		if c < 0 {
			return nil, invalidAddress()
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
		return nil, invalidAddress()
	}
	var buf uint64
	var bits uint
	out := make([]byte, 0, len(s)*5/8+1)
	for i := 0; i < len(s); i++ {
		c := strings.IndexByte(b32Alphabet, s[i])
		if c < 0 {
			return nil, invalidAddress()
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
		return nil, invalidAddress()
	}
	return out, nil
}

func decodeBase16(s string) ([]byte, error) {
	s = strings.ToLower(s)
	if len(s) == 0 || len(s)%2 != 0 {
		return nil, invalidAddress()
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi := fromHex(s[i*2])
		lo := fromHex(s[i*2+1])
		if hi < 0 || lo < 0 {
			return nil, invalidAddress()
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
