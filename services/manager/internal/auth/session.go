package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"sync"
	"time"
)

const (
	// IdleTimeout is how long a session may sit unused.
	IdleTimeout = 12 * time.Hour
	// AbsoluteTimeout is how long a session may live even if it is used.
	AbsoluteTimeout = 7 * 24 * time.Hour

	// SessionCookieName is the HttpOnly session cookie.
	SessionCookieName = "ipfs_panel_session"
	// CSRFCookieName is the readable double-submit cookie used before login.
	CSRFCookieName = "ipfs_panel_csrf"
	// CSRFHeader is required on state-changing requests.
	CSRFHeader = "X-CSRF-Token"
)

type session struct {
	username string
	csrf     string
	created  time.Time
	seen     time.Time
}

// SessionStore keeps opaque sessions in memory. A restart drops them.
type SessionStore struct {
	mu    sync.Mutex
	now   func() time.Time
	items map[[32]byte]session
}

func newSessionStore(now func() time.Time) *SessionStore {
	return &SessionStore{now: now, items: map[[32]byte]session{}}
}

func (s *SessionStore) setNow(now func() time.Time) {
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

func tokenKey(raw string) [32]byte {
	return sha256.Sum256([]byte(raw))
}

// Create returns the raw cookie value and the session CSRF secret.
func (s *SessionStore) Create(username string) (string, string, error) {
	raw, err := NewToken()
	if err != nil {
		return "", "", err
	}
	csrf, err := NewToken()
	if err != nil {
		return "", "", err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[tokenKey(raw)] = session{username: username, csrf: csrf, created: now, seen: now}
	return raw, csrf, nil
}

// Lookup returns the session and slides the idle clock when it is still valid.
func (s *SessionStore) Lookup(raw string) (string, string, bool) {
	if raw == "" {
		return "", "", false
	}
	key := tokenKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	if !ok {
		return "", "", false
	}
	now := s.now()
	if !sessionLive(item, now) {
		delete(s.items, key)
		return "", "", false
	}
	item.seen = now
	s.items[key] = item
	return item.username, item.csrf, true
}

func sessionLive(item session, now time.Time) bool {
	if now.Sub(item.created) > AbsoluteTimeout {
		return false
	}
	if now.Sub(item.seen) > IdleTimeout {
		return false
	}
	return true
}

// Delete removes one session. The username is returned for the audit line.
func (s *SessionStore) Delete(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	key := tokenKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	if !ok {
		return "", false
	}
	delete(s.items, key)
	return item.username, true
}

// RotateCSRF replaces the CSRF secret on this session and drops other
// sessions for the same username.
func (s *SessionStore) RotateCSRF(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	csrf, err := NewToken()
	if err != nil {
		return "", false
	}
	key := tokenKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	if !ok {
		return "", false
	}
	now := s.now()
	if !sessionLive(item, now) {
		delete(s.items, key)
		return "", false
	}
	for other, candidate := range s.items {
		if other != key && candidate.username == item.username {
			delete(s.items, other)
		}
	}
	item.csrf = csrf
	item.seen = now
	s.items[key] = item
	return csrf, true
}

// TokenEqual compares two secrets without leaking the matching prefix.
func TokenEqual(a, b string) bool {
	if len(a) != len(b) {
		// Keep the compare so a length mismatch is not a cheaper path
		// than a same-length mismatch of a similar size.
		subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
