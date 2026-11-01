package auth

import (
	"strings"
	"sync"
	"time"
)

const (
	// failuresFree is how many failures happen before a wait starts.
	failuresFree = 4
	backoffCap   = 15 * time.Minute
	limiterCap   = 4096
)

type bucket struct {
	fails int
	until time.Time
}

// Limiter backs off login attempts per IP and per username.
// It does not sleep. The caller returns 429 when Allow is false.
type Limiter struct {
	mu   sync.Mutex
	now  func() time.Time
	hits map[string]*bucket
}

func newLimiter(now func() time.Time) *Limiter {
	return &Limiter{now: now, hits: map[string]*bucket{}}
}

func (l *Limiter) setNow(now func() time.Time) {
	l.mu.Lock()
	l.now = now
	l.mu.Unlock()
}

func ipKey(ip string) string     { return "ip:" + ip }
func userKey(name string) string { return "user:" + strings.ToLower(name) }

// Allow reports whether a try may proceed. The duration is how long to wait
// when it may not.
func (l *Limiter) Allow(ip, username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	wait := time.Duration(0)
	blocked := false
	for _, key := range []string{ipKey(ip), userKey(username)} {
		b := l.hits[key]
		if b == nil || !now.Before(b.until) {
			continue
		}
		blocked = true
		if d := b.until.Sub(now); d > wait {
			wait = d
		}
	}
	return !blocked, wait
}

// Fail records one failure against both keys.
func (l *Limiter) Fail(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for _, key := range []string{ipKey(ip), userKey(username)} {
		b := l.hits[key]
		if b == nil {
			b = &bucket{}
			l.hits[key] = b
		}
		b.fails++
		b.until = now.Add(backoff(b.fails))
	}
	l.prune(now)
}

// Success clears both keys so a good password is not stuck behind old failures.
func (l *Limiter) Success(ip, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, ipKey(ip))
	delete(l.hits, userKey(username))
}

func (l *Limiter) prune(now time.Time) {
	if len(l.hits) <= limiterCap {
		return
	}
	for key, b := range l.hits {
		if !now.Before(b.until) {
			delete(l.hits, key)
		}
	}
	if len(l.hits) > limiterCap {
		l.hits = map[string]*bucket{}
	}
}

func backoff(fails int) time.Duration {
	if fails <= failuresFree {
		return 0
	}
	shift := fails - failuresFree - 1
	if shift > 10 {
		shift = 10
	}
	d := time.Second << shift
	if d > backoffCap {
		return backoffCap
	}
	return d
}
