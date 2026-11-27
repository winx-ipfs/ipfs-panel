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
	id       string
	username string
	csrf     string
	created  time.Time
	seen     time.Time
	ip       string
}

// SessionView is one live session the settings page can show.
// It does not include the cookie or the CSRF secret.
type SessionView struct {
	ID      string
	Current bool
	Created time.Time
	Seen    time.Time
	IP      string
}

// SessionStore keeps opaque sessions in memory. A restart drops them.
type SessionStore struct {
	mu       sync.Mutex
	now      func() time.Time
	idle     time.Duration
	absolute time.Duration
	items    map[[32]byte]session
}

func newSessionStore(now func() time.Time, idle, absolute time.Duration) *SessionStore {
	if idle <= 0 {
		idle = IdleTimeout
	}
	if absolute <= 0 {
		absolute = AbsoluteTimeout
	}
	return &SessionStore{now: now, idle: idle, absolute: absolute, items: map[[32]byte]session{}}
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
// ip is the client address stored for the session list.
func (s *SessionStore) Create(username, ip string) (string, string, error) {
	raw, err := NewToken()
	if err != nil {
		return "", "", err
	}
	csrf, err := NewToken()
	if err != nil {
		return "", "", err
	}
	id, err := NewToken()
	if err != nil {
		return "", "", err
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[tokenKey(raw)] = session{id: id, username: username, csrf: csrf, created: now, seen: now, ip: ip}
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
	if !s.live(item, now) {
		delete(s.items, key)
		return "", "", false
	}
	item.seen = now
	s.items[key] = item
	return item.username, item.csrf, true
}

// Touch records the client address of a request that already presented a live session.
func (s *SessionStore) Touch(raw, ip string) {
	if raw == "" || ip == "" {
		return
	}
	key := tokenKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	if !ok {
		return
	}
	item.ip = ip
	s.items[key] = item
}

// List returns the live sessions. The one matching raw is marked current.
func (s *SessionStore) List(raw string) []SessionView {
	key := tokenKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	out := []SessionView{}
	for id, item := range s.items {
		if !s.live(item, now) {
			delete(s.items, id)
			continue
		}
		out = append(out, SessionView{
			ID:      item.id,
			Current: raw != "" && id == key,
			Created: item.created.UTC(),
			Seen:    item.seen.UTC(),
			IP:      item.ip,
		})
	}
	sortSessions(out)
	return out
}

// RevokeID drops the session with id. current is true when it was the caller's.
func (s *SessionStore) RevokeID(raw, id string) (username string, current bool, ok bool) {
	if raw == "" || id == "" {
		return "", false, false
	}
	key := tokenKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	caller, exists := s.items[key]
	if !exists {
		return "", false, false
	}
	for other, item := range s.items {
		if item.id != id {
			continue
		}
		delete(s.items, other)
		return caller.username, other == key, true
	}
	return "", false, false
}

// RevokeOthers drops every session except the caller's.
func (s *SessionStore) RevokeOthers(raw string) (username string, n int, ok bool) {
	if raw == "" {
		return "", 0, false
	}
	key := tokenKey(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	caller, exists := s.items[key]
	if !exists {
		return "", 0, false
	}
	for other := range s.items {
		if other == key {
			continue
		}
		delete(s.items, other)
		n++
	}
	return caller.username, n, true
}

func (s *SessionStore) live(item session, now time.Time) bool {
	if now.Sub(item.created) > s.absolute {
		return false
	}
	if now.Sub(item.seen) > s.idle {
		return false
	}
	return true
}

func sortSessions(items []SessionView) {
	// Current first, then most recently seen. A stable insertion keeps tests quiet.
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && sessionLess(items[j], items[j-1]) {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}

func sessionLess(a, b SessionView) bool {
	if a.Current != b.Current {
		return a.Current
	}
	return a.Seen.After(b.Seen)
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
	if !s.live(item, now) {
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
