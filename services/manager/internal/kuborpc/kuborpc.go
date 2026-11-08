// Package kuborpc holds the transport rules shared by every client that talks
// to a Kubo RPC on loopback: the base-URL check, the loopback-only dialer, and
// the error classification both panel clients need.
package kuborpc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// ParseBase accepts an http loopback origin with an explicit port and no
// credentials, path, query, or fragment. Anything else is refused.
func ParseBase(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return nil, errors.New("kubo rpc must be an http loopback origin")
	}
	if u.Scheme != "http" || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("kubo rpc must be an http loopback origin")
	}
	if u.Port() == "" {
		return nil, errors.New("kubo rpc must have a port")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("kubo rpc must be an http loopback origin")
	}
	u.Path = ""
	return u, nil
}

// Client returns an http.Client whose every dial is checked to be loopback.
// Environment proxies are ignored and redirects are not followed, so the
// client cannot be steered off the local machine by a proxy setting or a
// 302 from a hostile process on a loopback port.
func Client() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return DialLoopback(ctx, dialer, network, addr)
			},
			DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
				return nil, errors.New("kubo rpc tls is not used")
			},
			DisableCompression: true,
			ForceAttemptHTTP2:  false,
			MaxIdleConns:       4,
			IdleConnTimeout:    30 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// DialLoopback dials addr only when it resolves to a loopback IP literal.
func DialLoopback(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("refusing non-loopback kubo rpc")
	}
	if ip.To4() != nil {
		network = "tcp4"
	} else if network == "tcp" {
		network = "tcp6"
	}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

// IsRefused reports whether err is a refused TCP connection. Windows returns
// WSAECONNREFUSED (10061) rather than the POSIX ECONNREFUSED constant, so both
// are compared directly instead of relying on the textual message.
func IsRefused(err error) bool {
	if err == nil {
		return false
	}
	const wsaECONNREFUSED = syscall.Errno(10061)
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, wsaECONNREFUSED)
}
