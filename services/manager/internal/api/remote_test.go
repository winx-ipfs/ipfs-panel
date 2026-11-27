package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/access"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
)

func TestAccessJSONOmitsKubo(t *testing.T) {
	_, mux, _ := authMux(t, false)
	body, cookies := statusBody(t, mux, "127.0.0.1:9")
	created := postAuth(mux, "/api/auth/setup", "127.0.0.1:9", "panel.local", []*http.Cookie{cookieNamed(cookies, auth.CSRFCookieName)}, body["csrfToken"].(string),
		`{"username":"ada","password":"correct horse battery"}`, false)
	if created.Code != http.StatusCreated {
		t.Fatal(created.Body.String())
	}
	session := cookieNamed(created.Result().Cookies(), auth.SessionCookieName)
	req := httptest.NewRequest(http.MethodGet, "/api/access", nil)
	req.RemoteAddr = "127.0.0.1:9"
	req.Host = "panel.local"
	req.AddCookie(session)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("access %d %s", rr.Code, rr.Body.Bytes())
	}
	text := rr.Body.String()
	for _, forbidden := range []string{"5001", "/ip4/", "multiaddr", "PrivKey", ".pem"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("access body contains %s: %s", forbidden, text)
		}
	}
	var snap access.Snapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if !snap.Loopback || snap.Remote || snap.TLS != "off" || snap.Scheme != "http" {
		t.Fatalf("%+v", snap)
	}
}

func TestLoginRateLimitUsesPeerNotForwardedHeader(t *testing.T) {
	_, mux, buf := authMux(t, true)
	body, cookies := statusBody(t, mux, "127.0.0.1:9")
	// Trust is on, but the peer is loopback and the forwarded client is too,
	// so setup does not need the token.
	setup := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"username":"ada","password":"correct horse battery"}`))
	setup.RemoteAddr = "127.0.0.1:9"
	setup.Host = "panel.local"
	setup.Header.Set("Content-Type", "application/json")
	setup.Header.Set("X-Forwarded-For", "127.0.0.1")
	setup.Header.Set(auth.CSRFHeader, body["csrfToken"].(string))
	setup.AddCookie(cookieNamed(cookies, auth.CSRFCookieName))
	created := httptest.NewRecorder()
	mux.ServeHTTP(created, setup)
	if created.Code != http.StatusCreated {
		t.Fatalf("setup %d %s", created.Code, created.Body.Bytes())
	}
	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		status, more := statusBody(t, mux, "203.0.113.9:9")
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"ada","password":"not-the-password"}`))
		req.RemoteAddr = "203.0.113.9:9"
		req.Host = "panel.local"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		req.Header.Set(auth.CSRFHeader, status["csrfToken"].(string))
		req.AddCookie(cookieNamed(more, auth.CSRFCookieName))
		last = httptest.NewRecorder()
		mux.ServeHTTP(last, req)
	}
	if last.Code != http.StatusTooManyRequests || last.Header().Get("Retry-After") == "" {
		t.Fatalf("rate %d retry %q body %s", last.Code, last.Header().Get("Retry-After"), last.Body.Bytes())
	}
	log := buf.String()
	if !strings.Contains(log, "ip=203.0.113.9") || !strings.Contains(log, "remote=true") || strings.Contains(log, "198.51.100.") {
		t.Fatalf("audit used a forged address\n%s", log)
	}
}

func TestSessionListAndRevoke(t *testing.T) {
	_, mux, buf := authMux(t, false)
	body, cookies := statusBody(t, mux, "127.0.0.1:9")
	created := postAuth(mux, "/api/auth/setup", "127.0.0.1:9", "panel.local", []*http.Cookie{cookieNamed(cookies, auth.CSRFCookieName)}, body["csrfToken"].(string),
		`{"username":"ada","password":"correct horse battery"}`, true)
	if created.Code != http.StatusCreated {
		t.Fatal(created.Body.String())
	}
	first := cookieNamed(created.Result().Cookies(), auth.SessionCookieName)
	var firstBody map[string]string
	if err := json.Unmarshal(created.Body.Bytes(), &firstBody); err != nil {
		t.Fatal(err)
	}
	if first == nil || !first.Secure || !first.HttpOnly || first.SameSite != http.SameSiteStrictMode || first.Path != "/" {
		t.Fatalf("tls cookie %#v", first)
	}
	status, more := statusBody(t, mux, "198.51.100.20:9")
	second := postAuth(mux, "/api/auth/login", "198.51.100.20:9", "panel.local", []*http.Cookie{cookieNamed(more, auth.CSRFCookieName)}, status["csrfToken"].(string),
		`{"username":"ada","password":"correct horse battery"}`, false)
	if second.Code != http.StatusOK {
		t.Fatalf("second login %d %s", second.Code, second.Body.Bytes())
	}
	other := cookieNamed(second.Result().Cookies(), auth.SessionCookieName)
	var secondBody map[string]string
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatal(err)
	}
	listReq := httptest.NewRequest(http.MethodGet, "/api/access/sessions", nil)
	listReq.RemoteAddr = "198.51.100.20:9"
	listReq.Host = "panel.local"
	listReq.AddCookie(other)
	listRR := httptest.NewRecorder()
	mux.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list %d %s", listRR.Code, listRR.Body.Bytes())
	}
	var listed struct {
		Sessions []sessionJSON `json:"sessions"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Sessions) != 2 {
		t.Fatalf("sessions %+v", listed.Sessions)
	}
	var current, previous string
	for _, item := range listed.Sessions {
		if item.Current {
			current = item.ID
			if item.IP != "198.51.100.20" {
				t.Fatalf("current ip %s", item.IP)
			}
		} else {
			previous = item.ID
		}
		if strings.Contains(item.ID, first.Value) || item.ID == "" {
			t.Fatalf("id leaks the cookie or is empty: %s", item.ID)
		}
	}
	if current == "" || previous == "" {
		t.Fatalf("ids current %q previous %q", current, previous)
	}
	drop := httptest.NewRequest(http.MethodPost, "/api/access/sessions/revoke", strings.NewReader(`{"id":"`+previous+`"}`))
	drop.RemoteAddr = "198.51.100.20:9"
	drop.Host = "panel.local"
	drop.Header.Set("Content-Type", "application/json")
	drop.Header.Set(auth.CSRFHeader, secondBody["csrfToken"])
	drop.AddCookie(other)
	dropRR := httptest.NewRecorder()
	mux.ServeHTTP(dropRR, drop)
	if dropRR.Code != http.StatusNoContent {
		t.Fatalf("revoke %d %s", dropRR.Code, dropRR.Body.Bytes())
	}
	gone := httptest.NewRequest(http.MethodGet, "/api/access/sessions", nil)
	gone.RemoteAddr = "127.0.0.1:9"
	gone.Host = "panel.local"
	gone.AddCookie(first)
	goneRR := httptest.NewRecorder()
	mux.ServeHTTP(goneRR, gone)
	if goneRR.Code != http.StatusUnauthorized {
		t.Fatalf("revoked cookie %d", goneRR.Code)
	}
	bare := httptest.NewRequest(http.MethodPost, "/api/access/sessions/revoke-others", strings.NewReader(`{}`))
	bare.RemoteAddr = "198.51.100.20:9"
	bare.Host = "panel.local"
	bare.Header.Set("Content-Type", "application/json")
	bare.AddCookie(other)
	bareRR := httptest.NewRecorder()
	mux.ServeHTTP(bareRR, bare)
	if bareRR.Code != http.StatusForbidden || !strings.Contains(bareRR.Body.String(), "csrf") {
		t.Fatalf("csrf %d %s", bareRR.Code, bareRR.Body.Bytes())
	}
	origin := httptest.NewRequest(http.MethodPost, "/api/access/sessions/revoke", strings.NewReader(`{"id":"`+current+`"}`))
	origin.RemoteAddr = "198.51.100.20:9"
	origin.Host = "panel.local"
	origin.Header.Set("Content-Type", "application/json")
	origin.Header.Set("Origin", "https://evil.example")
	origin.Header.Set(auth.CSRFHeader, secondBody["csrfToken"])
	origin.AddCookie(other)
	originRR := httptest.NewRecorder()
	mux.ServeHTTP(originRR, origin)
	if originRR.Code != http.StatusForbidden {
		t.Fatalf("origin %d %s", originRR.Code, originRR.Body.Bytes())
	}
	self := httptest.NewRequest(http.MethodPost, "/api/access/sessions/revoke", strings.NewReader(`{"id":"`+current+`"}`))
	self.RemoteAddr = "198.51.100.20:9"
	self.Host = "panel.local"
	self.Header.Set("Content-Type", "application/json")
	self.Header.Set(auth.CSRFHeader, secondBody["csrfToken"])
	self.AddCookie(other)
	selfRR := httptest.NewRecorder()
	mux.ServeHTTP(selfRR, self)
	if selfRR.Code != http.StatusNoContent {
		t.Fatalf("self revoke %d %s", selfRR.Code, selfRR.Body.Bytes())
	}
	if cleared := cookieNamed(selfRR.Result().Cookies(), auth.SessionCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("cleared %#v", cleared)
	}
	log := buf.String()
	if !strings.Contains(log, "event=session_revoke") || !strings.Contains(log, "ip=198.51.100.20") || !strings.Contains(log, "remote=true") {
		t.Fatalf("audit\n%s", log)
	}
	if strings.Contains(log, first.Value) || strings.Contains(log, other.Value) || strings.Contains(log, "correct horse battery") {
		t.Fatal("audit logged a secret")
	}
}

func TestHostGuardAroundAPI(t *testing.T) {
	_, mux, _ := authMux(t, false)
	prepared, err := access.Prepare(access.Options{Addr: "0.0.0.0:8080", DataDir: t.TempDir(), Insecure: true, Hosts: []string{"panel.example"}})
	if err != nil {
		t.Fatal(err)
	}
	handler := prepared.Guard(mux)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Host = "evil.example"
	req.RemoteAddr = "198.51.100.4:9"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMisdirectedRequest {
		t.Fatalf("evil host %d %s", rr.Code, rr.Body.Bytes())
	}
	ok := httptest.NewRequest(http.MethodGet, "/health", nil)
	ok.Host = "panel.example"
	ok.RemoteAddr = "198.51.100.4:9"
	okRR := httptest.NewRecorder()
	handler.ServeHTTP(okRR, ok)
	if okRR.Code != http.StatusOK {
		t.Fatalf("allowed host %d %s", okRR.Code, okRR.Body.Bytes())
	}
}

func TestShorterRemoteLifetime(t *testing.T) {
	if access.RemoteIdle != 4*time.Hour || access.RemoteAbsolute != 24*time.Hour {
		t.Fatalf("lifetimes %s %s", access.RemoteIdle, access.RemoteAbsolute)
	}
}
