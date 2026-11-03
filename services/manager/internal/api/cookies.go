package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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

func secureCookie(r *http.Request, trustProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if !trustProxy {
		return false
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		return false
	}
	first := strings.TrimSpace(strings.Split(proto, ",")[0])
	return strings.EqualFold(first, "https")
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, raw string, trustProxy bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    raw,
		Path:     "/",
		MaxAge:   int(auth.AbsoluteTimeout.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie(r, trustProxy),
	})
}

func setCSRFCookie(w http.ResponseWriter, r *http.Request, token string, trustProxy bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.AbsoluteTimeout.Seconds()),
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   secureCookie(r, trustProxy),
	})
}

func clearCookie(w http.ResponseWriter, r *http.Request, name string, httpOnly, trustProxy bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: httpOnly,
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
	if trustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			ip := strings.TrimSpace(parts[len(parts)-1])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
