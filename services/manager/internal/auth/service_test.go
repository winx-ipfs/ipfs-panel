package auth

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newService(t *testing.T, now func() time.Time) (*Service, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	svc, err := New(t.TempDir(), "", Options{
		Params: TestingParams(),
		Logger: logger,
		Now:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, &buf
}

func TestSetupLoopbackSessionAndLogout(t *testing.T) {
	start := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	now := start
	svc, buf := newService(t, func() time.Time { return now })
	token, ok := svc.TakeSetupToken()
	if !ok || token == "" {
		t.Fatal("missing setup token")
	}
	if _, ok := svc.TakeSetupToken(); ok {
		t.Fatal("token handed out twice")
	}

	res, err := svc.Setup("ada", "correct horse battery", "127.0.0.1:40000", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Username != "ada" || res.Session == "" || res.CSRF == "" {
		t.Fatalf("%+v", res)
	}
	username, csrf, ok := svc.LookupSession(res.Session)
	if !ok || username != "ada" || csrf != res.CSRF {
		t.Fatalf("lookup %s %s %v", username, csrf, ok)
	}

	now = start.Add(IdleTimeout + time.Second)
	if _, _, ok := svc.LookupSession(res.Session); ok {
		t.Fatal("idle session survived")
	}

	again, err := svc.Login("ada", "correct horse battery", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	for step := time.Duration(0); step < AbsoluteTimeout-time.Hour; step += 11 * time.Hour {
		now = start.Add(IdleTimeout + time.Second + step)
		if _, _, ok := svc.LookupSession(again.Session); !ok {
			t.Fatalf("session died at %s", step)
		}
	}
	now = start.Add(IdleTimeout + time.Second + AbsoluteTimeout + time.Second)
	if _, _, ok := svc.LookupSession(again.Session); ok {
		t.Fatal("absolute session survived")
	}

	fresh, err := svc.Login("ada", "correct horse battery", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	svc.Logout(fresh.Session, "127.0.0.1")
	if _, _, ok := svc.LookupSession(fresh.Session); ok {
		t.Fatal("logged out session survived")
	}

	log := buf.String()
	for _, secret := range []string{token, fresh.Session, fresh.CSRF, "correct horse battery"} {
		if strings.Contains(log, secret) {
			t.Fatalf("log contains secret %q\n%s", secret, log)
		}
	}
	if !strings.Contains(log, "event=setup") || !strings.Contains(log, "event=login") || !strings.Contains(log, "event=logout") {
		t.Fatalf("audit missing\n%s", log)
	}
}

func TestSetupRemoteTokenAndSecondSetup(t *testing.T) {
	svc, _ := newService(t, nil)
	if _, err := svc.Setup("ada", "correct horse battery", "203.0.113.9:9", ""); err != ErrSetupForbidden {
		t.Fatalf("remote without token: %v", err)
	}
	if _, err := svc.Setup("ada", "correct horse battery", "203.0.113.9:9", "not-the-token"); err != ErrSetupForbidden {
		t.Fatalf("bad token: %v", err)
	}
	// A forwarded loopback address is not the TCP peer.
	if IsLoopback("203.0.113.9:9") || !IsLoopback("127.0.0.1:9") || !IsLoopback("[::1]:9") {
		t.Fatal("loopback detection")
	}
	token, ok := svc.TakeSetupToken()
	if !ok {
		t.Fatal("token")
	}
	if _, err := svc.Setup("ada", "correct horse battery", "203.0.113.9:9", token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Setup("ada", "correct horse battery", "127.0.0.1:9", token); err != ErrAlreadySetup {
		t.Fatalf("second setup: %v", err)
	}
	if _, err := svc.Login("ada", "wrong-password-1", "203.0.113.9"); err != ErrInvalidCredentials {
		t.Fatalf("bad password: %v", err)
	}
	if _, err := svc.Login("no-such-user", "wrong-password-1", "203.0.113.9"); err != ErrInvalidCredentials {
		t.Fatalf("missing user: %v", err)
	}
}

func TestRateLimitPerIPAndUsername(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	now := start
	svc, _ := newService(t, func() time.Time { return now })
	if _, err := svc.Setup("ada", "correct horse battery", "127.0.0.1:1", ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		_, err := svc.Login("ada", "wrong-password-1", "203.0.113.4")
		if err != ErrInvalidCredentials {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	_, err := svc.Login("ada", "wrong-password-1", "203.0.113.4")
	limited, ok := err.(*RateLimitError)
	if !ok || limited.RetryAfter < time.Second {
		t.Fatalf("expected backoff, got %v", err)
	}
	// The username bucket blocks a different IP. The IP bucket blocks a different name.
	if _, err := svc.Login("ada", "wrong-password-1", "203.0.113.5"); err == nil {
		t.Fatal("username bucket did not trip")
	}
	if _, err := svc.Login("other", "wrong-password-1", "203.0.113.4"); err == nil {
		t.Fatal("ip bucket did not trip")
	}
	if _, err := svc.Login("other", "wrong-password-1", "198.51.100.8"); err != ErrInvalidCredentials {
		t.Fatalf("unrelated client: %v", err)
	}
	now = start.Add(limited.RetryAfter)
	if _, err := svc.Login("ada", "correct horse battery", "203.0.113.4"); err != nil {
		t.Fatalf("login after wait: %v", err)
	}
	if _, err := svc.Login("missing", "wrong-password-1", "203.0.113.4"); err != ErrInvalidCredentials {
		t.Fatalf("bucket not cleared: %v", err)
	}
}

func TestChangePasswordRotatesCSRF(t *testing.T) {
	svc, buf := newService(t, time.Now)
	first, err := svc.Setup("ada", "correct horse battery", "127.0.0.1:1", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.Login("ada", "correct horse battery", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := svc.ChangePassword(first.Session, "correct horse battery", "a different passphrase", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if changed.CSRF == first.CSRF {
		t.Fatal("csrf was not rotated")
	}
	if _, _, ok := svc.LookupSession(other.Session); ok {
		t.Fatal("other session survived a password change")
	}
	if _, _, ok := svc.LookupSession(first.Session); !ok {
		t.Fatal("current session was dropped")
	}
	if _, err := svc.Login("ada", "correct horse battery", "127.0.0.1"); err != ErrInvalidCredentials {
		t.Fatal("old password still works")
	}
	if _, err := svc.ChangePassword(first.Session, "a different passphrase", "a different passphrase", "127.0.0.1"); err != ErrPasswordUnchanged {
		t.Fatalf("reuse: %v", err)
	}
	if !strings.Contains(buf.String(), "event=password_change") {
		t.Fatalf("audit\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "a different passphrase") || strings.Contains(buf.String(), changed.CSRF) {
		t.Fatal("audit logged a secret")
	}
}

func TestCredentialFileModeAndRepoGuard(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "kubo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(repo, repo, Options{Params: TestingParams()}); err == nil {
		t.Fatal("auth file inside the repo was accepted")
	}
	data := filepath.Join(dir, "panel")
	svc, err := New(data, repo, Options{Params: TestingParams()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Setup("ada", "correct horse battery", "127.0.0.1:1", ""); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(AuthPath(data))
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerOnly(t, info)
	dirInfo, err := os.Stat(data)
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerOnlyDir(t, dirInfo)
	body, err := os.ReadFile(AuthPath(data))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "correct horse battery") {
		t.Fatal("password stored in clear")
	}
	matches, err := filepath.Glob(filepath.Join(data, ".auth-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}
