package access

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteBind(t *testing.T) {
	local := []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"}
	for _, addr := range local {
		remote, err := RemoteBind(addr)
		if err != nil || remote {
			t.Fatalf("%s remote %v %v", addr, remote, err)
		}
	}
	far := []string{"0.0.0.0:8080", "[::]:8080", ":8080", "192.0.2.10:8443", "panel.example:443"}
	for _, addr := range far {
		remote, err := RemoteBind(addr)
		if err != nil || !remote {
			t.Fatalf("%s remote %v %v", addr, remote, err)
		}
	}
	if _, err := RemoteBind("not-an-address"); err == nil {
		t.Fatal("accepted a bare name")
	}
}

func TestRefuseRemoteWithoutTLS(t *testing.T) {
	_, err := Prepare(Options{Addr: "0.0.0.0:8080", DataDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "-insecure") {
		t.Fatalf("err %v", err)
	}
	prepared, err := Prepare(Options{Addr: "0.0.0.0:0", DataDir: t.TempDir(), Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.Insecure || prepared.TLSMode != "off" || !prepared.HostCheck {
		t.Fatalf("%+v", prepared)
	}
	if err := prepared.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = prepared.Close() })
	if !strings.HasPrefix(prepared.Bound(), "0.0.0.0:") {
		t.Fatalf("bound %s", prepared.Bound())
	}
}

func TestAutoCertStableFingerprint(t *testing.T) {
	dir := t.TempDir()
	first, err := Prepare(Options{Addr: "127.0.0.1:0", DataDir: dir, TLS: "auto", TLSHosts: []string{"panel.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Fingerprint == "" || first.Renewed || first.Scheme != "https" || first.HostCheck {
		t.Fatalf("%+v", first)
	}
	info, err := os.Stat(KeyPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerOnly(t, info)
	foundName := false
	for _, name := range first.names {
		if name == "panel.example" {
			foundName = true
		}
	}
	if !foundName {
		t.Fatalf("certificate names %v", first.names)
	}
	second, err := Prepare(Options{Addr: "127.0.0.1:0", DataDir: dir, TLS: "auto", TLSHosts: []string{"panel.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if second.Fingerprint != first.Fingerprint || second.Renewed {
		t.Fatalf("fingerprint changed %s %s renewed %v", first.Fingerprint, second.Fingerprint, second.Renewed)
	}
	if err := first.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			t.Error("handler did not see TLS")
		}
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{Handler: first.Guard(mux), ReadHeaderTimeout: time.Second}
	go func() { _ = server.Serve(first.Listener()) }()
	t.Cleanup(func() { _ = server.Close() })
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errString("no peer certificate")
			}
			if got := FingerprintOfDER(cs.PeerCertificates[0].Raw); got != first.Fingerprint {
				return errString("fingerprint " + got)
			}
			return nil
		},
	}}}
	response, err := client.Get("https://" + first.Bound() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	if response.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("frame %q", response.Header.Get("X-Frame-Options"))
	}
}

func TestGuardRejectsRebindingHost(t *testing.T) {
	prepared, err := Prepare(Options{
		Addr:       "0.0.0.0:8080",
		DataDir:    t.TempDir(),
		Insecure:   true,
		Hosts:      []string{"panel.example"},
		TrustProxy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := prepared.Guard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, host := range []string{"evil.example", "evil.example:8080", ""} {
		called = false
		req := httptest.NewRequest(http.MethodGet, "http://placeholder/health", nil)
		req.Host = host
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusMisdirectedRequest || called {
			t.Fatalf("host %q -> %d called %v", host, rr.Code, called)
		}
		if !strings.Contains(rr.Body.String(), "invalid_host") || strings.Contains(rr.Body.String(), "evil") {
			t.Fatalf("body %s", rr.Body.String())
		}
	}
	for _, host := range []string{"panel.example", "panel.example:8443", "198.51.100.10", "[::1]:8080", "localhost"} {
		called = false
		req := httptest.NewRequest(http.MethodGet, "http://placeholder/health", nil)
		req.Host = host
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent || !called {
			t.Fatalf("host %q -> %d called %v body %s", host, rr.Code, called, rr.Body.String())
		}
	}
}

func TestTrustedProxyPeer(t *testing.T) {
	nets, err := ParseNets([]string{"10.0.0.0/8", "192.0.2.5"})
	if err != nil {
		t.Fatal(err)
	}
	if !ProxyPeer("127.0.0.1:9", nil) || !ProxyPeer("[::1]:9", nil) {
		t.Fatal("loopback")
	}
	if ProxyPeer("203.0.113.4:9", nets) {
		t.Fatal("public peer was trusted")
	}
	if !ProxyPeer("10.1.2.3:9", nets) || !ProxyPeer("192.0.2.5:9", nets) {
		t.Fatal("configured proxy was not trusted")
	}
	if RightmostForwarded("1.2.3.4, 198.51.100.9") != "198.51.100.9" {
		t.Fatal("rightmost")
	}
	if RightmostForwarded("not-an-ip") != "" {
		t.Fatal("garbage forwarded header")
	}
	if _, err := ParseNets([]string{"nope"}); err == nil {
		t.Fatal("bad proxy")
	}
}

func TestFileCertAndAmbiguousTLS(t *testing.T) {
	dir := t.TempDir()
	made, err := Prepare(Options{Addr: "127.0.0.1:0", DataDir: dir, TLS: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Prepare(Options{Addr: "127.0.0.1:0", DataDir: dir, TLS: "auto", CertFile: CertPath(dir), KeyFile: KeyPath(dir)})
	if err == nil {
		t.Fatal("auto and file together")
	}
	_, err = Prepare(Options{Addr: "127.0.0.1:0", CertFile: CertPath(dir)})
	if err == nil {
		t.Fatal("cert without key")
	}
	got, err := Prepare(Options{Addr: "127.0.0.1:0", DataDir: t.TempDir(), CertFile: CertPath(dir), KeyFile: KeyPath(dir)})
	if err != nil {
		t.Fatal(err)
	}
	if got.TLSMode != "file" || got.Fingerprint != made.Fingerprint {
		t.Fatalf("mode %s fingerprint %s want %s", got.TLSMode, got.Fingerprint, made.Fingerprint)
	}
	body, err := os.ReadFile(CertPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(body)
	if block == nil {
		t.Fatal("pem")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(leaf.NotAfter) < 800*24*time.Hour {
		t.Fatalf("lifetime %s", leaf.NotAfter)
	}
}

func TestWebConfinesPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("home"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("js"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler, err := Web(dir)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Body.String() != "home" {
		t.Fatalf("index %s", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if rr.Body.String() != "home" || rr.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("spa %d %s %s", rr.Code, rr.Header().Get("Cache-Control"), rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if rr.Body.String() != "js" || !strings.Contains(rr.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset %s %s", rr.Header().Get("Cache-Control"), rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/../secret.txt", nil))
	if strings.Contains(rr.Body.String(), "secret") {
		t.Fatalf("escaped: %s", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/missing.js", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing ext %d", rr.Code)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
