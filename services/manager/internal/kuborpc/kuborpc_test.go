package kuborpc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestParseBaseAcceptsOnlyLoopbackHTTPWithPort(t *testing.T) {
	for _, raw := range []string{
		"http://192.0.2.1:5001",
		"https://127.0.0.1:5001",
		"http://127.0.0.1",
		"http://localhost:5001",
		"http://127.0.0.1:5001/api",
		"http://user:pass@127.0.0.1:5001",
		"http://127.0.0.1:5001?x=1",
		"",
	} {
		if _, err := ParseBase(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	for _, raw := range []string{"http://127.0.0.1:5001", "http://[::1]:5001", "http://127.0.0.1:5001/"} {
		if _, err := ParseBase(raw); err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
	}
}

func TestDialLoopbackRefusesPublicAddress(t *testing.T) {
	dialer := &net.Dialer{Timeout: time.Second}
	if _, err := DialLoopback(context.Background(), dialer, "tcp", "192.0.2.1:5001"); err == nil {
		t.Fatal("dialed a public address")
	}
	if _, err := DialLoopback(context.Background(), dialer, "tcp", "example.com:5001"); err == nil {
		t.Fatal("dialed a hostname")
	}
}

func TestClientRefusesRedirectToPublic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://192.0.2.1/nope", http.StatusFound)
	}))
	defer srv.Close()
	resp, err := Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("followed the redirect: %d", resp.StatusCode)
	}
}

// IsRefused must recognise a refused loopback connection on every supported
// platform. Windows reports WSAECONNREFUSED (10061) instead of the POSIX
// ECONNREFUSED, so a plain errors.Is against the POSIX constant silently
// turns "the node is down" into "the RPC is unavailable" there.
func TestIsRefusedDetectsClosedLoopbackPort(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	_, dialErr := DialLoopback(context.Background(), dialer, "tcp4", addr)
	if dialErr == nil {
		t.Fatal("dialed a closed port")
	}
	if !IsRefused(dialErr) {
		t.Fatalf("did not classify %v as refused", dialErr)
	}
	if IsRefused(nil) {
		t.Fatal("nil classified as refused")
	}
	if IsRefused(errors.New("something else")) {
		t.Fatal("unrelated error classified as refused")
	}
	if IsRefused(context.DeadlineExceeded) {
		t.Fatal("deadline classified as refused")
	}
}

func TestIsRefusedMatchesWSAConstant(t *testing.T) {
	// On Windows the dial error carries syscall.Errno(10061), wrapped by
	// *os.SyscallError and then *net.OpError. Build that exact shape here so
	// the check is exercised on the Linux runner too.
	wsa := syscall.Errno(10061)
	wrapped := &net.OpError{
		Op:  "dial",
		Net: "tcp4",
		Err: &os.SyscallError{Syscall: "connectex", Err: wsa},
	}
	if !IsRefused(wrapped) {
		t.Fatalf("did not match a wrapped WSAECONNREFUSED: %v", wrapped)
	}
	if IsRefused(&net.OpError{Op: "dial", Net: "tcp4", Err: syscall.Errno(10060)}) {
		t.Fatal("matched a different Winsock error")
	}
}
