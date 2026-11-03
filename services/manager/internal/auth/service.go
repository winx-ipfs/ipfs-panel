package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

const usernameMax = 64

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var (
	// ErrAlreadySetup means an admin file is already there.
	ErrAlreadySetup = errors.New("already_setup")
	// ErrSetupForbidden means the peer is not loopback and the token is wrong.
	ErrSetupForbidden = errors.New("setup_forbidden")
	// ErrInvalidUsername means the username failed the shape rule.
	ErrInvalidUsername = errors.New("invalid_username")
	// ErrInvalidPassword means the new password failed the rule.
	ErrInvalidPassword = errors.New("invalid_password")
	// ErrInvalidCredentials is the generic login and current-password failure.
	ErrInvalidCredentials = errors.New("invalid_credentials")
	// ErrPasswordUnchanged means the new password matches the current one.
	ErrPasswordUnchanged = errors.New("password_unchanged")
	// ErrRateLimited is also wrapped by RateLimitError.
	ErrRateLimited = errors.New("rate_limited")
)

// RateLimitError is a login or password-change refusal with a wait.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return ErrRateLimited.Error() }

func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

// Result is a session the caller may put in a cookie.
// Session is the raw cookie value. The server stores only its hash.
type Result struct {
	Username string
	Session  string
	CSRF     string
}

// Options configures a Service. Zero Params is rejected; use DefaultParams.
type Options struct {
	Params     Params
	TrustProxy bool
	Logger     *slog.Logger
	Now        func() time.Time
}

// Service is the admin account, its sessions, and the one-time setup token.
type Service struct {
	store    *Store
	sessions *SessionStore
	limiter  *Limiter
	params   Params
	dummy    string
	log      *slog.Logger
	clock    func() time.Time
	trust    bool

	mu        sync.Mutex
	setupHash []byte
	setupRaw  string
}

// New opens the data directory and, when no admin exists, mints a setup token.
// repo is the Kubo repo. The credential file must not live inside it.
func New(dataDir, repo string, opt Options) (*Service, error) {
	params := opt.Params
	if params == (Params{}) {
		params = DefaultParams()
	}
	params, err := params.normalize()
	if err != nil {
		return nil, err
	}
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	logger := opt.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	store, err := openStore(dataDir, repo)
	if err != nil {
		return nil, err
	}
	if _, _, err := store.Load(); err != nil {
		return nil, err
	}
	dummyPassword, err := NewToken()
	if err != nil {
		return nil, err
	}
	dummy, err := Hash(dummyPassword, params)
	if err != nil {
		return nil, err
	}
	svc := &Service{
		store:    store,
		sessions: newSessionStore(now),
		limiter:  newLimiter(now),
		params:   params,
		dummy:    dummy,
		log:      logger,
		clock:    now,
		trust:    opt.TrustProxy,
	}
	exists, err := svc.SetupRequired()
	if err != nil {
		return nil, err
	}
	if exists {
		raw, err := NewToken()
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256([]byte(raw))
		svc.setupHash = append([]byte(nil), sum[:]...)
		svc.setupRaw = raw
	}
	return svc, nil
}

// SetNow replaces the clock used by sessions and the rate limiter.
// Tests use it. A nil clock restores time.Now.
func (s *Service) SetNow(now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	s.mu.Lock()
	s.clock = now
	s.mu.Unlock()
	s.sessions.setNow(now)
	s.limiter.setNow(now)
}

// TrustProxy reports whether forwarded headers may affect cookies and client IP.
func (s *Service) TrustProxy() bool { return s.trust }

// SetupRequired reports whether the UI should show the setup screen.
func (s *Service) SetupRequired() (bool, error) {
	_, exists, err := s.store.Load()
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// TakeSetupToken returns the raw token once, for the startup log line.
// Later compares use the hash. A second call returns false.
func (s *Service) TakeSetupToken() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setupRaw == "" {
		return "", false
	}
	token := s.setupRaw
	s.setupRaw = ""
	return token, true
}

// Setup creates the admin when the peer is loopback or token matches.
func (s *Service) Setup(username, password, remoteAddr, token string) (Result, error) {
	ip := hostOnly(remoteAddr)
	required, err := s.SetupRequired()
	if err != nil {
		return Result{}, err
	}
	if !required {
		s.audit("setup", "exists", username, ip)
		return Result{}, ErrAlreadySetup
	}
	loopback := IsLoopback(remoteAddr)
	if !loopback && !s.tokenMatches(token) {
		s.audit("setup", "forbidden", username, ip)
		return Result{}, ErrSetupForbidden
	}
	if err := ValidateUsername(username); err != nil {
		s.audit("setup", "invalid", username, ip)
		return Result{}, err
	}
	if err := ValidatePassword(password); err != nil {
		s.audit("setup", "invalid", username, ip)
		return Result{}, err
	}
	encoded, err := Hash(password, s.params)
	if err != nil {
		return Result{}, err
	}
	rec := adminRecord{Username: username, PasswordHash: encoded, UpdatedAt: s.now()}
	if err := s.commitSetup(rec, loopback, token); err != nil {
		if errors.Is(err, ErrAlreadySetup) {
			s.audit("setup", "exists", username, ip)
		}
		if errors.Is(err, ErrSetupForbidden) {
			s.audit("setup", "forbidden", username, ip)
		}
		return Result{}, err
	}
	raw, csrf, err := s.sessions.Create(username)
	if err != nil {
		return Result{}, err
	}
	s.audit("setup", "success", username, ip)
	return Result{Username: username, Session: raw, CSRF: csrf}, nil
}

func (s *Service) commitSetup(rec adminRecord, loopback bool, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists, err := s.store.Load()
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadySetup
	}
	if !loopback && !tokenMatchesHash(s.setupHash, token) {
		return ErrSetupForbidden
	}
	if err := s.store.Save(rec); err != nil {
		return err
	}
	s.setupHash = nil
	s.setupRaw = ""
	return nil
}

// Login checks the password and opens a session.
// A missing user and a wrong password return the same error.
func (s *Service) Login(username, password, ip string) (Result, error) {
	if ok, wait := s.limiter.Allow(ip, username); !ok {
		s.audit("login", "rate_limited", username, ip)
		return Result{}, &RateLimitError{RetryAfter: wait}
	}
	admin, exists, err := s.store.Load()
	if err != nil {
		return Result{}, err
	}
	hash := s.dummy
	userOK := false
	if exists && constUsername(admin.Username, username) {
		hash = admin.PasswordHash
		userOK = true
	}
	passOK := Verify(password, hash)
	if !exists || !userOK || !passOK {
		s.limiter.Fail(ip, username)
		s.audit("login", "failure", username, ip)
		return Result{}, ErrInvalidCredentials
	}
	s.limiter.Success(ip, username)
	raw, csrf, err := s.sessions.Create(admin.Username)
	if err != nil {
		return Result{}, err
	}
	s.audit("login", "success", admin.Username, ip)
	return Result{Username: admin.Username, Session: raw, CSRF: csrf}, nil
}

// LookupSession slides the idle clock when the cookie is still valid.
func (s *Service) LookupSession(raw string) (string, string, bool) {
	return s.sessions.Lookup(raw)
}

// Logout drops the session and writes an audit line when it existed.
func (s *Service) Logout(raw, ip string) {
	username, ok := s.sessions.Delete(raw)
	if !ok {
		return
	}
	s.audit("logout", "success", username, ip)
}

// ChangePassword checks the current password, stores a new hash, and rotates CSRF.
func (s *Service) ChangePassword(raw, current, next, ip string) (Result, error) {
	username, _, ok := s.sessions.Lookup(raw)
	if !ok {
		return Result{}, ErrInvalidCredentials
	}
	if allowed, wait := s.limiter.Allow(ip, username); !allowed {
		s.audit("password_change", "rate_limited", username, ip)
		return Result{}, &RateLimitError{RetryAfter: wait}
	}
	admin, exists, err := s.store.Load()
	if err != nil {
		return Result{}, err
	}
	if !exists || !constUsername(admin.Username, username) || !Verify(current, admin.PasswordHash) {
		s.limiter.Fail(ip, username)
		s.audit("password_change", "failure", username, ip)
		return Result{}, ErrInvalidCredentials
	}
	if Verify(next, admin.PasswordHash) {
		s.audit("password_change", "invalid", username, ip)
		return Result{}, ErrPasswordUnchanged
	}
	if err := ValidatePassword(next); err != nil {
		s.audit("password_change", "invalid", username, ip)
		return Result{}, err
	}
	encoded, err := Hash(next, s.params)
	if err != nil {
		return Result{}, err
	}
	rec := adminRecord{Username: admin.Username, PasswordHash: encoded, UpdatedAt: s.now()}
	if err := s.store.Save(rec); err != nil {
		return Result{}, err
	}
	s.limiter.Success(ip, username)
	csrf, rotated := s.sessions.RotateCSRF(raw)
	if !rotated {
		return Result{}, ErrInvalidCredentials
	}
	s.audit("password_change", "success", admin.Username, ip)
	return Result{Username: admin.Username, Session: raw, CSRF: csrf}, nil
}

func (s *Service) now() time.Time {
	s.mu.Lock()
	clock := s.clock
	s.mu.Unlock()
	if clock == nil {
		return time.Now()
	}
	return clock()
}

func (s *Service) tokenMatches(token string) bool {
	s.mu.Lock()
	hash := append([]byte(nil), s.setupHash...)
	s.mu.Unlock()
	return tokenMatchesHash(hash, token)
}

func tokenMatchesHash(hash []byte, token string) bool {
	token = strings.TrimSpace(token)
	sum := sha256.Sum256([]byte(token))
	if len(hash) != sha256.Size {
		subtle.ConstantTimeCompare(sum[:], sum[:])
		return false
	}
	return subtle.ConstantTimeCompare(hash, sum[:]) == 1
}

func (s *Service) audit(event, result, username, ip string) {
	s.log.Info("audit",
		"event", event,
		"result", result,
		"username", username,
		"ip", ip,
	)
}

// ValidateUsername checks the admin name. It is case-sensitive at login.
func ValidateUsername(name string) error {
	if len(name) == 0 || len(name) > usernameMax || !usernamePattern.MatchString(name) {
		return ErrInvalidUsername
	}
	return nil
}

func constUsername(stored, given string) bool {
	if len(stored) > usernameMax || len(given) > usernameMax {
		return false
	}
	var a, b [usernameMax]byte
	copy(a[:], stored)
	copy(b[:], given)
	same := subtle.ConstantTimeCompare(a[:], b[:])
	sameLen := subtle.ConstantTimeEq(int32(len(stored)), int32(len(given)))
	return same&sameLen == 1
}

// IsLoopback reports whether remoteAddr is a loopback TCP peer.
// It does not read X-Forwarded-For. That header is not a proof of locality.
func IsLoopback(remoteAddr string) bool {
	host := hostOnly(remoteAddr)
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hostOnly(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
