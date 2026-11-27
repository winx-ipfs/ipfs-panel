package access

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/atomicfile"
)

const (
	certLifetime = 825 * 24 * time.Hour
	renewWithin  = 14 * 24 * time.Hour
	maxExtraIPs  = 16
)

func fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	var b strings.Builder
	for i, part := range sum {
		if i > 0 {
			b.WriteByte(':')
		}
		_, _ = fmt.Fprintf(&b, "%02x", part)
	}
	return b.String()
}

// FingerprintOfDER is the SHA-256 of a certificate, shown as colon-separated hex.
func FingerprintOfDER(der []byte) string {
	return fingerprint(der)
}

type material struct {
	config      *tls.Config
	fingerprint string
	dnsNames    []string
	renewed     bool
	expired     bool
}

func loadMaterial(opt Options, now time.Time) (*material, error) {
	mode, certFile, keyFile, err := tlsChoice(opt)
	if err != nil {
		return nil, err
	}
	switch mode {
	case "off":
		return &material{}, nil
	case "file":
		return loadFilePair(certFile, keyFile, now)
	default:
		return loadAuto(opt, now)
	}
}

func tlsChoice(opt Options) (mode, certFile, keyFile string, err error) {
	mode = strings.ToLower(strings.TrimSpace(opt.TLS))
	if mode == "" {
		mode = "off"
	}
	certFile = strings.TrimSpace(opt.CertFile)
	keyFile = strings.TrimSpace(opt.KeyFile)
	switch {
	case (certFile == "") != (keyFile == ""):
		return "", "", "", fmt.Errorf("tls: pass both -tls-cert and -tls-key, or neither")
	case certFile != "" && mode == "auto":
		return "", "", "", fmt.Errorf("tls: -tls auto and -tls-cert are mutually exclusive")
	case certFile != "":
		return "file", certFile, keyFile, nil
	case mode == "off" || mode == "auto":
		return mode, "", "", nil
	default:
		return "", "", "", fmt.Errorf("tls: -tls must be off or auto, not %q", opt.TLS)
	}
}

func loadFilePair(certFile, keyFile string, now time.Time) (*material, error) {
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tls certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("tls certificate: %w", err)
	}
	return &material{
		config:      serverConfig(&pair),
		fingerprint: fingerprint(pair.Certificate[0]),
		dnsNames:    append([]string(nil), leaf.DNSNames...),
		expired:     now.After(leaf.NotAfter),
	}, nil
}

func loadAuto(opt Options, now time.Time) (*material, error) {
	if strings.TrimSpace(opt.DataDir) == "" {
		return nil, fmt.Errorf("tls auto: data directory is required")
	}
	dir := filepath.Join(opt.DataDir, "tls")
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	_, statErr := os.Stat(certPath)
	existed := statErr == nil
	dns, ips := desiredNames(opt)
	if pair, leaf, err := readPair(certPath, keyPath); err == nil && covers(leaf, dns, ips, now) {
		return &material{
			config:      serverConfig(pair),
			fingerprint: fingerprint(pair.Certificate[0]),
			dnsNames:    append([]string(nil), leaf.DNSNames...),
		}, nil
	}
	pair, der, err := issue(dns, ips, now)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := atomicfile.Write(certPath, certPEM, 0o600); err != nil {
		return nil, fmt.Errorf("tls certificate: %w", err)
	}
	if err := atomicfile.Write(keyPath, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("tls key: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &material{
		config:      serverConfig(pair),
		fingerprint: fingerprint(der),
		dnsNames:    append([]string(nil), leaf.DNSNames...),
		renewed:     existed,
	}, nil
}

func readPair(certPath, keyPath string) (*tls.Certificate, *x509.Certificate, error) {
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, nil, err
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, nil, err
	}
	return &pair, leaf, nil
}

func covers(leaf *x509.Certificate, dns []string, ips []net.IP, now time.Time) bool {
	if now.Add(renewWithin).After(leaf.NotAfter) || now.Before(leaf.NotBefore) {
		return false
	}
	have := map[string]struct{}{}
	for _, name := range leaf.DNSNames {
		have[strings.ToLower(name)] = struct{}{}
	}
	for _, name := range dns {
		if _, ok := have[strings.ToLower(name)]; !ok {
			return false
		}
	}
	for _, ip := range ips {
		found := false
		for _, existing := range leaf.IPAddresses {
			if canonIP(existing).Equal(canonIP(ip)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func issue(dns []string, ips []net.IP, now time.Time) (*tls.Certificate, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "ipfs-panel"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(certLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dns,
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	pair := &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}
	return pair, der, nil
}

func serverConfig(pair *tls.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{*pair},
		NextProtos:   []string{"h2", "http/1.1"},
	}
}

func desiredNames(opt Options) (dns []string, ips []net.IP) {
	addDNS := func(name string) {
		name = strings.TrimSuffix(strings.TrimSpace(name), ".")
		if name == "" || wildcardHost(name) {
			return
		}
		for _, existing := range dns {
			if strings.EqualFold(existing, name) {
				return
			}
		}
		dns = append(dns, name)
	}
	addIP := func(ip net.IP) {
		if ip == nil {
			return
		}
		ip = canonIP(ip)
		for _, existing := range ips {
			if existing.Equal(ip) {
				return
			}
		}
		ips = append(ips, ip)
	}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || wildcardHost(value) {
			return
		}
		if ip := net.ParseIP(value); ip != nil {
			addIP(ip)
			return
		}
		addDNS(value)
	}
	addDNS("localhost")
	addIP(net.ParseIP("127.0.0.1"))
	addIP(net.ParseIP("::1"))
	host := ""
	if h, _, err := net.SplitHostPort(opt.Addr); err == nil {
		host = h
	}
	add(host)
	for _, name := range opt.TLSHosts {
		add(name)
	}
	extra := 0
	for _, ip := range machineIPs() {
		if extra >= maxExtraIPs {
			break
		}
		before := len(ips)
		addIP(ip)
		if len(ips) != before {
			extra++
		}
	}
	return dns, ips
}

func machineIPs() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			network, ok := addr.(*net.IPNet)
			if !ok || network.IP == nil {
				continue
			}
			ip := canonIP(network.IP)
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
				continue
			}
			out = append(out, ip)
		}
	}
	return out
}

// KeyPath is where -tls auto stores the private key. Tests check its mode.
func KeyPath(dataDir string) string {
	return filepath.Join(dataDir, "tls", "key.pem")
}

// CertPath is where -tls auto stores the certificate.
func CertPath(dataDir string) string {
	return filepath.Join(dataDir, "tls", "cert.pem")
}
