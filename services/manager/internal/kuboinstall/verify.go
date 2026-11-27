package kuboinstall

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// parseSHA512 reads a sidecar the way sha512sum writes it: the hash is the
// first field. The file name after it is not trusted.
func parseSHA512(body []byte) (string, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", fmt.Errorf("empty checksum")
	}
	field := strings.Fields(text)[0]
	field = strings.ToLower(field)
	if len(field) != sha512.Size*2 {
		return "", fmt.Errorf("checksum length")
	}
	if _, err := hex.DecodeString(field); err != nil {
		return "", err
	}
	return field, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
