package access

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// RemoteIdle is how long a session may sit unused once the manager is
	// reachable from another machine.
	RemoteIdle = 4 * time.Hour
	// RemoteAbsolute is how long that session may live even if it is used.
	RemoteAbsolute = 24 * time.Hour
	localIdle      = 12 * time.Hour
	localAbsolute  = 7 * 24 * time.Hour
)

// Options is the listen decision. Paths and flags come from the process, never
// from an HTTP body.
type Options struct {
	Addr           string
	DataDir        string
	TLS            string
	CertFile       string
	KeyFile        string
	TLSHosts       []string
	Hosts          []string
	Insecure       bool
	TrustProxy     bool
	TrustedProxies []string
	WebDir         string
	Now            func() time.Time
}

// Snapshot is the access section of Settings. It has no key, no Kubo address,
// and no filesystem path.
type Snapshot struct {
	Loopback    bool     `json:"loopback"`
	Remote      bool     `json:"remote"`
	Scheme      string   `json:"scheme"`
	TLS         string   `json:"tls"`
	Insecure    bool     `json:"insecure"`
	TrustProxy  bool     `json:"trustProxy"`
	Fingerprint string   `json:"fingerprint"`
	Listen      []string `json:"listen"`
	Reachable   []string `json:"reachable"`
	Hosts       []string `json:"hosts"`
}

// Prepared is a validated listen configuration. Listen binds the port.
type Prepared struct {
	Remote      bool
	TrustProxy  bool
	Insecure    bool
	TLSMode     string
	Fingerprint string
	Scheme      string
	HostCheck   bool
	Idle        time.Duration
	Absolute    time.Duration
	ProxyNets   []*net.IPNet
	Web         http.Handler
	Renewed     bool
	Expired     bool
	names       []string
	tlsConfig   *tls.Config
	addr        string
	listener    net.Listener
	bound       string
}

// Prepare checks the flags and, when TLS is on, loads or writes the certificate.
// It does not bind a port.
func Prepare(opt Options) (*Prepared, error) {
	now := opt.Now
	if now == nil {
		now = time.Now
	}
	remote, err := RemoteBind(opt.Addr)
	if err != nil {
		return nil, err
	}
	mode, _, _, err := tlsChoice(opt)
	if err != nil {
		return nil, err
	}
	if remote && mode == "off" && !opt.Insecure {
		return nil, fmt.Errorf("refusing to listen on %s without TLS: pass -tls auto, or both -tls-cert and -tls-key, or -insecure to allow cleartext on a network you trust", opt.Addr)
	}
	nets, err := ParseNets(opt.TrustedProxies)
	if err != nil {
		return nil, err
	}
	web, err := Web(opt.WebDir)
	if err != nil {
		return nil, err
	}
	var mat *material
	if mode != "off" {
		mat, err = loadMaterial(opt, now())
		if err != nil {
			return nil, err
		}
	} else {
		mat = &material{}
	}
	hostCheck := remote || opt.TrustProxy
	names := allowNames(opt, mat.dnsNames)
	scheme := "http"
	if mode != "off" {
		scheme = "https"
	}
	idle, absolute := localIdle, localAbsolute
	if remote || opt.TrustProxy {
		idle, absolute = RemoteIdle, RemoteAbsolute
	}
	return &Prepared{
		Remote:      remote,
		TrustProxy:  opt.TrustProxy,
		Insecure:    remote && mode == "off" && opt.Insecure,
		TLSMode:     mode,
		Fingerprint: mat.fingerprint,
		Scheme:      scheme,
		HostCheck:   hostCheck,
		Idle:        idle,
		Absolute:    absolute,
		ProxyNets:   nets,
		Web:         web,
		Renewed:     mat.renewed,
		Expired:     mat.expired,
		names:       names,
		tlsConfig:   mat.config,
		addr:        opt.Addr,
	}, nil
}

func allowNames(opt Options, certDNS []string) []string {
	var names []string
	add := func(value string) {
		value = strings.TrimSuffix(strings.TrimSpace(value), ".")
		if value == "" || wildcardHost(value) || strings.EqualFold(value, "localhost") {
			return
		}
		if net.ParseIP(value) != nil {
			return
		}
		for _, existing := range names {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		names = append(names, value)
	}
	host, _, err := net.SplitHostPort(opt.Addr)
	if err == nil {
		add(host)
	}
	for _, name := range opt.Hosts {
		add(name)
	}
	for _, name := range opt.TLSHosts {
		add(name)
	}
	for _, name := range certDNS {
		add(name)
	}
	return names
}

// Listen binds Addr. A TLS configuration wraps the listener so the request
// sees a TLS connection state.
//
// The network is chosen from the address. Go's "tcp" network turns an IPv4
// wildcard into an IPv6 wildcard on a dual-stack host, and the settings page
// would then show the wrong address.
func (p *Prepared) Listen() error {
	ln, err := net.Listen(listenNetwork(p.addr), p.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", p.addr, err)
	}
	if p.tlsConfig != nil {
		ln = tls.NewListener(ln, p.tlsConfig)
	}
	p.listener = ln
	p.bound = ln.Addr().String()
	return nil
}

// Listener is the bound socket. Serve it with http.Server.
func (p *Prepared) Listener() net.Listener {
	return p.listener
}

// Bound is the address the socket actually took.
func (p *Prepared) Bound() string {
	if p.bound != "" {
		return p.bound
	}
	return p.addr
}

// Close closes the listener when one is open.
func (p *Prepared) Close() error {
	if p.listener == nil {
		return nil
	}
	return p.listener.Close()
}

// Snapshot copies the facts the settings page shows. Reachable addresses are
// read from the machine at call time.
func (p *Prepared) Snapshot() Snapshot {
	bound := p.Bound()
	listen := []string{}
	if bound != "" {
		listen = []string{bound}
	}
	hosts := append([]string(nil), p.names...)
	if hosts == nil {
		hosts = []string{}
	}
	reachable := []string{}
	if p.listener != nil {
		reachable = Reachable(bound)
	}
	return Snapshot{
		Loopback:    !p.Remote,
		Remote:      p.Remote,
		Scheme:      p.Scheme,
		TLS:         p.TLSMode,
		Insecure:    p.Insecure,
		TrustProxy:  p.TrustProxy,
		Fingerprint: p.Fingerprint,
		Listen:      listen,
		Reachable:   reachable,
		Hosts:       hosts,
	}
}

// AllowsHost reports whether a Host header may be served.
func (p *Prepared) AllowsHost(hostHeader string) bool {
	if !p.HostCheck {
		return true
	}
	host := Hostname(hostHeader)
	if host == "" || strings.ContainsAny(host, " \t/@\\") {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	for _, name := range p.names {
		if strings.EqualFold(name, host) {
			return true
		}
	}
	return false
}

// Guard rejects a Host the policy does not allow and sets the response headers
// that keep another site from framing or reading the panel.
func (p *Prepared) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		if p.HostCheck && !p.AllowsHost(r.Host) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusMisdirectedRequest)
			_, _ = w.Write([]byte("{\"error\":\"invalid_host\"}\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// InsecureIgnored reports that -insecure was set and TLS made it unnecessary.
func (p *Prepared) InsecureIgnored(flag bool) bool {
	return flag && p.TLSMode != "off"
}

// Reachable lists this machine's unicast addresses with the listener's port.
// Loopback and link-local addresses are left out. A loopback bind returns none.
func Reachable(bound string) []string {
	host, port, err := net.SplitHostPort(bound)
	if err != nil || port == "" {
		return []string{}
	}
	if loopbackHost(host) {
		return []string{}
	}
	if !wildcardHost(host) {
		if ip := net.ParseIP(host); ip != nil {
			return []string{net.JoinHostPort(canonIP(ip).String(), port)}
		}
		return []string{net.JoinHostPort(host, port)}
	}
	var out []string
	seen := map[string]struct{}{}
	for _, ip := range machineIPs() {
		text := net.JoinHostPort(ip.String(), port)
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		out = append(out, text)
		if len(out) == maxExtraIPs {
			break
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}
