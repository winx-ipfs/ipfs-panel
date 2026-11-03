package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
)

const maxAuthBody = 8192

type authHTTP struct {
	svc *auth.Service
}

type principal struct {
	raw      string
	username string
	csrf     string
}

type principalKey struct{}

func (h *authHTTP) setupStatus(w http.ResponseWriter, r *http.Request) {
	token, err := auth.NewToken()
	if err != nil {
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
		return
	}
	setCSRFCookie(w, r, token, h.svc.TrustProxy())
	required, err := h.svc.SetupRequired()
	if err != nil {
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setupRequired":      required,
		"setupTokenRequired": required && !auth.IsLoopback(r.RemoteAddr),
		"csrfToken":          token,
	})
}

func (h *authHTTP) setup(w http.ResponseWriter, r *http.Request) {
	if !originOK(r) || !doubleSubmit(r) {
		writeProblem(w, &statusError{code: http.StatusForbidden, msg: "csrf"})
		return
	}
	var body struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		SetupToken string `json:"setupToken"`
	}
	if err := readAuthJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	res, err := h.svc.Setup(body.Username, body.Password, r.RemoteAddr, body.SetupToken)
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	setSessionCookie(w, r, res.Session, h.svc.TrustProxy())
	setCSRFCookie(w, r, res.CSRF, h.svc.TrustProxy())
	writeJSON(w, http.StatusCreated, map[string]string{
		"username":  res.Username,
		"csrfToken": res.CSRF,
	})
}

func (h *authHTTP) login(w http.ResponseWriter, r *http.Request) {
	if !originOK(r) || !doubleSubmit(r) {
		writeProblem(w, &statusError{code: http.StatusForbidden, msg: "csrf"})
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readAuthJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	res, err := h.svc.Login(body.Username, body.Password, clientIP(r, h.svc.TrustProxy()))
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	setSessionCookie(w, r, res.Session, h.svc.TrustProxy())
	setCSRFCookie(w, r, res.CSRF, h.svc.TrustProxy())
	writeJSON(w, http.StatusOK, map[string]string{
		"username":  res.Username,
		"csrfToken": res.CSRF,
	})
}

func (h *authHTTP) session(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r)
	if !ok {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	setCSRFCookie(w, r, p.csrf, h.svc.TrustProxy())
	writeJSON(w, http.StatusOK, map[string]string{
		"username":  p.username,
		"csrfToken": p.csrf,
	})
}

func (h *authHTTP) logout(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r)
	if !ok {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	h.svc.Logout(p.raw, clientIP(r, h.svc.TrustProxy()))
	clearCookie(w, r, auth.SessionCookieName, true, h.svc.TrustProxy())
	clearCookie(w, r, auth.CSRFCookieName, false, h.svc.TrustProxy())
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHTTP) password(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r)
	if !ok {
		writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := readAuthJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	res, err := h.svc.ChangePassword(p.raw, body.CurrentPassword, body.NewPassword, clientIP(r, h.svc.TrustProxy()))
	if err != nil {
		writeAuthErr(w, err)
		return
	}
	setCSRFCookie(w, r, res.CSRF, h.svc.TrustProxy())
	writeJSON(w, http.StatusOK, map[string]string{
		"username":  res.Username,
		"csrfToken": res.CSRF,
	})
}

func protect(svc *auth.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicAuth(r) {
			next.ServeHTTP(w, r)
			return
		}
		raw := cookieValue(r, auth.SessionCookieName)
		username, csrf, ok := svc.LookupSession(raw)
		if !ok {
			w.Header().Set("Cache-Control", "no-store")
			writeProblem(w, &statusError{code: http.StatusUnauthorized, msg: "unauthorized"})
			return
		}
		if stateChanging(r.Method) {
			if !originOK(r) || !auth.TokenEqual(r.Header.Get(auth.CSRFHeader), csrf) {
				writeProblem(w, &statusError{code: http.StatusForbidden, msg: "csrf"})
				return
			}
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal{raw: raw, username: username, csrf: csrf})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isPublicAuth(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/auth/setup-status":
		return r.Method == http.MethodGet
	case "/api/auth/setup", "/api/auth/login":
		return r.Method == http.MethodPost
	default:
		return false
	}
}

func stateChanging(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func principalFrom(r *http.Request) (principal, bool) {
	p, ok := r.Context().Value(principalKey{}).(principal)
	return p, ok
}

func readAuthJSON(w http.ResponseWriter, r *http.Request, dest any) *statusError {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &statusError{code: http.StatusUnsupportedMediaType, msg: "unsupported_media_type"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	return nil
}
