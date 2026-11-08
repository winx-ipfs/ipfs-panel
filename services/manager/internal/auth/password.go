package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Params are argon2id settings. Memory is KiB, matching the PHC `m` field.
type Params struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

// DefaultParams is the OWASP interactive argon2id setting: 19 MiB, two
// passes, one lane. New passwords use this. Verify trusts the PHC string,
// within the caps in normalize.
func DefaultParams() Params {
	return Params{Time: 2, Memory: 19 * 1024, Threads: 1, KeyLen: 32}
}

// TestingParams is cheap on purpose so unit tests do not spend a fifth of a
// second per hash. Production code must not use it.
func TestingParams() Params {
	return Params{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: 32}
}

const (
	minPasswordRunes = 12
	maxPasswordRunes = 128
	maxPasswordBytes = 512
	saltLen          = 16
)

func (p Params) normalize() (Params, error) {
	if p.Time == 0 || p.Memory == 0 || p.Threads == 0 || p.KeyLen == 0 {
		return Params{}, errors.New("argon2 parameters are incomplete")
	}
	// Caps stop a hand-edited hash from asking the process for a huge region.
	if p.Time > 10 || p.Threads > 4 || p.Threads < 1 || p.KeyLen < 16 || p.KeyLen > 64 {
		return Params{}, errors.New("argon2 parameters out of range")
	}
	minMem := uint32(8) * uint32(p.Threads)
	if p.Memory < minMem || p.Memory > 256*1024 {
		return Params{}, errors.New("argon2 memory out of range")
	}
	return p, nil
}

// Hash returns a PHC argon2id string. The password is not retained.
func Hash(password string, p Params) (string, error) {
	p, err := p.normalize()
	if err != nil {
		return "", err
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		p.Memory, p.Time, p.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

// Verify reports whether password matches a PHC string from Hash.
// A malformed string or a parameter set outside the caps is a mismatch.
func Verify(password, encoded string) bool {
	p, salt, want, err := parsePHC(encoded)
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func parsePHC(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, errors.New("bad phc")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return Params{}, nil, nil, errors.New("bad version")
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, errors.New("bad params")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return Params{}, nil, nil, errors.New("bad salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, errors.New("bad tag")
	}
	p.KeyLen = uint32(len(want))
	if _, err := p.normalize(); err != nil {
		return Params{}, nil, nil, err
	}
	return p, salt, want, nil
}

// ValidatePassword is the server rule. The UI meter is only a hint.
func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < minPasswordRunes || n > maxPasswordRunes || len(password) > maxPasswordBytes {
		return ErrInvalidPassword
	}
	if strings.IndexByte(password, 0) >= 0 || repeated(password) || denied(password) {
		return ErrInvalidPassword
	}
	return nil
}

func repeated(password string) bool {
	r, size := utf8.DecodeRuneInString(password)
	if r == utf8.RuneError && size < 2 {
		return true
	}
	for _, c := range password {
		if c != r {
			return false
		}
	}
	return true
}

func denied(password string) bool {
	_, ok := passwordDenylist[strings.ToLower(password)]
	return ok
}

// passwordDenylist is exact and short. It is not a guess of every weak
// passphrase. Length is still the main rule.
var passwordDenylist = map[string]struct{}{
	"password1234":     {},
	"password12345":    {},
	"passwordpassword": {},
	"123456789012":     {},
	"1234567890123":    {},
	"qwertyuiopas":     {},
	"qwertyuiop123":    {},
	"adminadminadmin":  {},
	"letmeinletmein":   {},
	"changeme1234":     {},
	"ipfs-panel123":    {},
	"kubokubokubo1":    {},
}
