package pins

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

func parseBase(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return nil, &OpError{Code: "unavailable"}
	}
	if u.Scheme != "http" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, &OpError{Code: "unavailable"}
	}
	if u.Port() == "" {
		return nil, &OpError{Code: "unavailable"}
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, &OpError{Code: "unavailable"}
	}
	u.Path = ""
	return u, nil
}

func loopbackClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialLoopback(ctx, dialer, network, addr)
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

func dialLoopback(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
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

func classify(err error) error {
	if err == nil {
		return nil
	}
	if CodeOf(err) != "" {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return &OpError{Code: "cancelled"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &OpError{Code: "timeout"}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &OpError{Code: "timeout"}
	}
	if errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "connection refused") {
		return &OpError{Code: "node_not_running"}
	}
	return &OpError{Code: "unavailable"}
}

func kuboResult(status int, body []byte) error {
	if unsupportedArg(status, body, "names") {
		return &OpError{Code: "names_unsupported"}
	}
	if unsupportedArg(status, body, "name") {
		return &OpError{Code: "name_unsupported"}
	}
	msg := strings.ToLower(kuboMessage(body))
	switch {
	case strings.Contains(msg, "indirect"):
		return &OpError{Code: "indirect"}
	case strings.Contains(msg, "not pinned"), strings.Contains(msg, "not found"), strings.Contains(msg, "no such"), strings.Contains(msg, "does not exist"):
		return &OpError{Code: "not_found"}
	default:
		if status == http.StatusNotFound {
			return &OpError{Code: "not_found"}
		}
		return &OpError{Code: "kubo_error"}
	}
}

func unsupportedArg(status int, body []byte, arg string) bool {
	if status < 400 || status >= 500 {
		return false
	}
	msg := strings.ToLower(kuboMessage(body))
	if !mentionsArg(msg, arg) {
		return false
	}
	return strings.Contains(msg, "argument") || strings.Contains(msg, "option") || strings.Contains(msg, "flag") ||
		strings.Contains(msg, "unrecognized") || strings.Contains(msg, "unused") || strings.Contains(msg, "unknown")
}

func mentionsArg(msg, arg string) bool {
	if strings.Contains(msg, `"`+arg+`"`) || strings.Contains(msg, "'"+arg+"'") {
		return true
	}
	for _, field := range strings.FieldsFunc(msg, func(r rune) bool {
		return r != '_' && !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if field == arg {
			return true
		}
	}
	return false
}

func kuboMessage(body []byte) string {
	var wrap struct {
		Message string `json:"Message"`
	}
	if json.Unmarshal(body, &wrap) == nil && strings.TrimSpace(wrap.Message) != "" {
		return wrap.Message
	}
	return strings.TrimSpace(string(body))
}
