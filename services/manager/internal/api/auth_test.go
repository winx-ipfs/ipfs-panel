package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

func authMux(t *testing.T, trust bool) (*auth.Service, http.Handler, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	svc, err := auth.New(t.TempDir(), "", auth.Options{
		Params:     auth.TestingParams(),
		TrustProxy: trust,
		Logger:     slog.New(slog.NewTextHandler(&buf, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	sup, err := node.New(node.Config{
		ManagerAddr:     "127.0.0.1:8080",
		RepoPath:        t.TempDir(),
		KuboBin:         t.TempDir() + "/missing-ipfs",
		InitTimeout:     time.Second,
		ConfigTimeout:   time.Second,
		IDTimeout:       time.Second,
		ReadyBudget:     time.Second,
		ShutdownTimeout: time.Second,
		StopKillAfter:   time.Second,
		PollInterval:    10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = sup.Shutdown(ctx)
	})
	return svc, NewMux(sup, svc), &buf
}

func statusBody(t *testing.T, mux http.Handler, remote string) (map[string]any, []*http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/setup-status", nil)
	req.RemoteAddr = remote
	req.Host = "panel.local"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d %s", rr.Code, rr.Body.Bytes())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body, rr.Result().Cookies()
}

func postAuth(mux http.Handler, path, remote, host string, cookies []*http.Cookie, header string, payload string, tlsOn bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
	req.RemoteAddr = remote
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeader, header)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if tlsOn {
		req.TLS = &tls.ConnectionState{}
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestSetupGatingAndLogin(t *testing.T) {
	svc, mux, buf := authMux(t, false)
	body, cookies := statusBody(t, mux, "127.0.0.1:9")
	if body["setupRequired"] != true || body["setupTokenRequired"] != false {
		t.Fatalf("local status %#v", body)
	}
	token, _ := body["csrfToken"].(string)
	csrf := cookieNamed(cookies, auth.CSRFCookieName)
	if csrf == nil || csrf.Value != token || csrf.HttpOnly {
		t.Fatalf("csrf cookie %#v", csrf)
	}

	remoteBody, _ := statusBody(t, mux, "203.0.113.10:9")
	if remoteBody["setupTokenRequired"] != true {
		t.Fatalf("remote status %#v", remoteBody)
	}

	// A spoofed forwarded address does not count as loopback.
	spoof := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"username":"ada","password":"correct horse battery"}`))
	spoof.RemoteAddr = "203.0.113.10:9"
	spoof.Host = "panel.local"
	spoof.Header.Set("Content-Type", "application/json")
	spoof.Header.Set("X-Forwarded-For", "127.0.0.1")
	spoof.Header.Set(auth.CSRFHeader, token)
	spoof.AddCookie(csrf)
	spoofed := httptest.NewRecorder()
	mux.ServeHTTP(spoofed, spoof)
	if spoofed.Code != http.StatusForbidden {
		t.Fatalf("spoofed loopback %d %s", spoofed.Code, spoofed.Body.Bytes())
	}

	setupToken, ok := svc.TakeSetupToken()
	if !ok {
		t.Fatal("setup token")
	}
	bad := postAuth(mux, "/api/auth/setup", "203.0.113.10:9", "panel.local", []*http.Cookie{csrf}, token, `{"username":"ada","password":"correct horse battery"}`, false)
	if bad.Code != http.StatusForbidden {
		t.Fatalf("remote setup %d", bad.Code)
	}
	// Refresh CSRF because setup-status rotates it and the old cookie is what we hold.
	// The forbidden response did not rotate. Reuse the same cookie.
	good := postAuth(mux, "/api/auth/setup", "203.0.113.10:9", "panel.local", []*http.Cookie{csrf}, token,
		`{"username":"ada","password":"correct horse battery","setupToken":"`+setupToken+`"}`, false)
	if good.Code != http.StatusCreated {
		t.Fatalf("setup %d %s", good.Code, good.Body.Bytes())
	}
	session := cookieNamed(good.Result().Cookies(), auth.SessionCookieName)
	if session == nil || !session.HttpOnly || session.SameSite != http.SameSiteStrictMode || session.Secure {
		t.Fatalf("session cookie %#v", session)
	}
	again := postAuth(mux, "/api/auth/setup", "127.0.0.1:9", "panel.local", []*http.Cookie{csrf}, token,
		`{"username":"ada","password":"correct horse battery"}`, false)
	if again.Code != http.StatusConflict {
		t.Fatalf("second setup %d", again.Code)
	}

	status, loginCookies := statusBody(t, mux, "127.0.0.1:9")
	loginCSRF, _ := status["csrfToken"].(string)
	loginCookie := cookieNamed(loginCookies, auth.CSRFCookieName)
	missing := postAuth(mux, "/api/auth/login", "198.51.100.4:9", "panel.local", []*http.Cookie{loginCookie}, loginCSRF,
		`{"username":"nobody","password":"not-the-password"}`, false)
	wrong := postAuth(mux, "/api/auth/login", "198.51.100.4:9", "panel.local", []*http.Cookie{loginCookie}, loginCSRF,
		`{"username":"ada","password":"not-the-password"}`, false)
	if missing.Code != http.StatusUnauthorized || wrong.Code != http.StatusUnauthorized || missing.Body.String() != wrong.Body.String() {
		t.Fatalf("generic\nmissing %d %s\nwrong %d %s", missing.Code, missing.Body.Bytes(), wrong.Code, wrong.Body.Bytes())
	}
	evil := postAuth(mux, "/api/auth/login", "198.51.100.4:9", "panel.local", []*http.Cookie{loginCookie}, loginCSRF,
		`{"username":"ada","password":"correct horse battery"}`, false)
	evil.Result()
	// Origin mismatch.
	originReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"ada","password":"correct horse battery"}`))
	originReq.RemoteAddr = "198.51.100.4:9"
	originReq.Host = "panel.local"
	originReq.Header.Set("Content-Type", "application/json")
	originReq.Header.Set("Origin", "https://evil.example")
	originReq.Header.Set(auth.CSRFHeader, loginCSRF)
	originReq.AddCookie(loginCookie)
	originRR := httptest.NewRecorder()
	mux.ServeHTTP(originRR, originReq)
	if originRR.Code != http.StatusForbidden {
		t.Fatalf("origin %d", originRR.Code)
	}

	okLogin := postAuth(mux, "/api/auth/login", "198.51.100.4:9", "panel.local", []*http.Cookie{loginCookie}, loginCSRF,
		`{"username":"ada","password":"correct horse battery"}`, true)
	if okLogin.Code != http.StatusOK {
		t.Fatalf("login %d %s", okLogin.Code, okLogin.Body.Bytes())
	}
	logged := cookieNamed(okLogin.Result().Cookies(), auth.SessionCookieName)
	if logged == nil || !logged.Secure || !logged.HttpOnly {
		t.Fatalf("tls cookie %#v", logged)
	}
	var loggedBody map[string]string
	if err := json.Unmarshal(okLogin.Body.Bytes(), &loggedBody); err != nil {
		t.Fatal(err)
	}

	node := httptest.NewRequest(http.MethodGet, "/api/node", nil)
	nodeRR := httptest.NewRecorder()
	mux.ServeHTTP(nodeRR, node)
	if nodeRR.Code != http.StatusUnauthorized {
		t.Fatalf("node without session %d", nodeRR.Code)
	}

	authed := httptest.NewRequest(http.MethodPost, "/api/node/stop", strings.NewReader("{}"))
	authed.RemoteAddr = "198.51.100.4:9"
	authed.Host = "panel.local"
	authed.Header.Set("Content-Type", "application/json")
	authed.AddCookie(logged)
	noCSRF := httptest.NewRecorder()
	mux.ServeHTTP(noCSRF, authed)
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("csrf %d %s", noCSRF.Code, noCSRF.Body.Bytes())
	}

	authed.Header.Set(auth.CSRFHeader, loggedBody["csrfToken"])
	withCSRF := httptest.NewRecorder()
	mux.ServeHTTP(withCSRF, authed)
	if withCSRF.Code != http.StatusOK {
		t.Fatalf("stop %d %s", withCSRF.Code, withCSRF.Body.Bytes())
	}

	log := buf.String()
	if strings.Contains(log, "correct horse battery") || strings.Contains(log, setupToken) || strings.Contains(log, logged.Value) {
		t.Fatal("audit or handler logged a secret")
	}
}

func TestSecureCookieFollowsProxyOnlyWhenTrusted(t *testing.T) {
	_, mux, _ := authMux(t, false)
	_, cookies := statusBody(t, mux, "127.0.0.1:9")
	token := cookies[0].Value
	// Find the csrf token from a fresh status; cookies[0] may be the csrf cookie.
	body, cookies := statusBody(t, mux, "127.0.0.1:9")
	csrfToken, _ := body["csrfToken"].(string)
	csrf := cookieNamed(cookies, auth.CSRFCookieName)
	rr := postAuth(mux, "/api/auth/setup", "127.0.0.1:9", "panel.local", []*http.Cookie{csrf}, csrfToken,
		`{"username":"ada","password":"correct horse battery"}`, false)
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup %d %s", rr.Code, rr.Body.Bytes())
	}
	plain := cookieNamed(rr.Result().Cookies(), auth.SessionCookieName)
	if plain == nil || plain.Secure {
		t.Fatalf("plain cookie %#v", plain)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"ada","password":"correct horse battery"}`))
	req.RemoteAddr = "10.0.0.8:9"
	req.Host = "panel.local"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	status, more := statusBody(t, mux, "10.0.0.8:9")
	req.Header.Set(auth.CSRFHeader, status["csrfToken"].(string))
	req.AddCookie(cookieNamed(more, auth.CSRFCookieName))
	got := httptest.NewRecorder()
	mux.ServeHTTP(got, req)
	if got.Code != http.StatusOK {
		t.Fatalf("login %d %s", got.Code, got.Body.Bytes())
	}
	if cookie := cookieNamed(got.Result().Cookies(), auth.SessionCookieName); cookie == nil || cookie.Secure {
		t.Fatalf("untrusted proxy cookie %#v", cookie)
	}

	trusted, trustedMux, _ := authMux(t, true)
	_ = trusted
	tBody, tCookies := statusBody(t, trustedMux, "127.0.0.1:9")
	created := postAuth(trustedMux, "/api/auth/setup", "127.0.0.1:9", "panel.local", []*http.Cookie{cookieNamed(tCookies, auth.CSRFCookieName)}, tBody["csrfToken"].(string),
		`{"username":"ada","password":"correct horse battery"}`, false)
	if created.Code != http.StatusCreated {
		t.Fatalf("trusted setup %d %s", created.Code, created.Body.Bytes())
	}
	sBody, sCookies := statusBody(t, trustedMux, "10.1.0.5:9")
	proxied := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"ada","password":"correct horse battery"}`))
	proxied.RemoteAddr = "10.1.0.5:9"
	proxied.Host = "panel.example"
	proxied.Header.Set("Content-Type", "application/json")
	proxied.Header.Set("X-Forwarded-Proto", "https")
	proxied.Header.Set(auth.CSRFHeader, sBody["csrfToken"].(string))
	proxied.AddCookie(cookieNamed(sCookies, auth.CSRFCookieName))
	proxiedRR := httptest.NewRecorder()
	trustedMux.ServeHTTP(proxiedRR, proxied)
	if proxiedRR.Code != http.StatusOK {
		t.Fatalf("proxied login %d %s", proxiedRR.Code, proxiedRR.Body.Bytes())
	}
	if cookie := cookieNamed(proxiedRR.Result().Cookies(), auth.SessionCookieName); cookie == nil || !cookie.Secure {
		t.Fatalf("trusted proxy cookie %#v", cookie)
	}
	_ = token
	_ = io.EOF
}

func TestPasswordChangeAndLogoutHTTP(t *testing.T) {
	_, mux, _ := authMux(t, false)
	body, cookies := statusBody(t, mux, "127.0.0.1:9")
	created := postAuth(mux, "/api/auth/setup", "127.0.0.1:9", "panel.local", []*http.Cookie{cookieNamed(cookies, auth.CSRFCookieName)}, body["csrfToken"].(string),
		`{"username":"ada","password":"correct horse battery"}`, false)
	if created.Code != http.StatusCreated {
		t.Fatal(created.Body.String())
	}
	session := cookieNamed(created.Result().Cookies(), auth.SessionCookieName)
	var createdBody map[string]string
	if err := json.Unmarshal(created.Body.Bytes(), &createdBody); err != nil {
		t.Fatal(err)
	}
	change := httptest.NewRequest(http.MethodPost, "/api/auth/password", strings.NewReader(`{"currentPassword":"correct horse battery","newPassword":"a different passphrase"}`))
	change.RemoteAddr = "127.0.0.1:9"
	change.Host = "panel.local"
	change.Header.Set("Content-Type", "application/json")
	change.Header.Set(auth.CSRFHeader, createdBody["csrfToken"])
	change.AddCookie(session)
	changed := httptest.NewRecorder()
	mux.ServeHTTP(changed, change)
	if changed.Code != http.StatusOK {
		t.Fatalf("change %d %s", changed.Code, changed.Body.Bytes())
	}
	var changedBody map[string]string
	if err := json.Unmarshal(changed.Body.Bytes(), &changedBody); err != nil {
		t.Fatal(err)
	}
	out := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	out.RemoteAddr = "127.0.0.1:9"
	out.Host = "panel.local"
	out.Header.Set("Content-Type", "application/json")
	out.Header.Set(auth.CSRFHeader, changedBody["csrfToken"])
	out.AddCookie(session)
	loggedOut := httptest.NewRecorder()
	mux.ServeHTTP(loggedOut, out)
	if loggedOut.Code != http.StatusNoContent {
		t.Fatalf("logout %d %s", loggedOut.Code, loggedOut.Body.Bytes())
	}
	after := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	after.AddCookie(session)
	afterRR := httptest.NewRecorder()
	mux.ServeHTTP(afterRR, after)
	if afterRR.Code != http.StatusUnauthorized {
		t.Fatalf("session after logout %d", afterRR.Code)
	}
}
