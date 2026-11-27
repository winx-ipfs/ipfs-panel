package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/access"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeAuthErr(w http.ResponseWriter, err error) {
	var limited *auth.RateLimitError
	switch {
	case errors.As(err, &limited):
		secs := int(limited.RetryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		writeProblem(w, &statusError{code: http.StatusTooManyRequests, msg: "rate_limited"})
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "invalid_credentials"})
	case errors.Is(err, auth.ErrAlreadySetup):
		writeProblem(w, &statusError{code: http.StatusConflict, msg: "already_setup"})
	case errors.Is(err, auth.ErrSetupForbidden):
		writeProblem(w, &statusError{code: http.StatusForbidden, msg: "setup_forbidden"})
	case errors.Is(err, auth.ErrInvalidUsername):
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "invalid_username"})
	case errors.Is(err, auth.ErrInvalidPassword):
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "invalid_password"})
	case errors.Is(err, auth.ErrPasswordUnchanged):
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "password_unchanged"})
	default:
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
	}
}

// secureCookie reports whether the Set-Cookie header may carry Secure. A TLS
// connection is enough. Behind a trusted reverse proxy only the first
// X-Forwarded-Proto value counts, and a client-supplied header is ignored
// unless that trust was turned on. The panel itself serves plain HTTP on
// loopback, where a Secure cookie would not be stored at all, so the answer
// is "secure only when the browser sees HTTPS".
func secureCookie(r *http.Request, trustProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if !trustProxy || !access.ProxyPeer(r.RemoteAddr, netsFrom(r)) {
		return false
	}
	first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(first), "https")
}

func cookieLifetime(d time.Duration) int {
	if d <= 0 {
		d = auth.AbsoluteTimeout
	}
	return int(d.Seconds())
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, raw string, trustProxy bool, lifetime time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    raw,
		Path:     "/",
		MaxAge:   cookieLifetime(lifetime),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie(r, trustProxy),
	})
}

func setCSRFCookie(w http.ResponseWriter, r *http.Request, token string, trustProxy bool, lifetime time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   cookieLifetime(lifetime),
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie(r, trustProxy),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request, trustProxy bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie(r, trustProxy),
	})
}

func clearCSRFCookie(w http.ResponseWriter, r *http.Request, trustProxy bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie(r, trustProxy),
	})
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func doubleSubmit(r *http.Request) bool {
	return auth.TokenEqual(r.Header.Get(auth.CSRFHeader), cookieValue(r, auth.CSRFCookieName))
}

func originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func clientIP(r *http.Request, trustProxy bool) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if trustProxy && access.ProxyPeer(r.RemoteAddr, netsFrom(r)) {
		if ip := access.RightmostForwarded(r.Header.Get("X-Forwarded-For")); ip != "" {
			return ip
		}
	}
	return host
}
