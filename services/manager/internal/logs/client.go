package logs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
)

const (
	// LevelTimeout is the deadline for log/level and log/ls.
	LevelTimeout = 10 * time.Second

	maxLevelBody = 64 << 10
	maxListBody  = 1 << 20
)

var (
	subsystemRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./-]{0,63}$`)
	levels      = map[string]struct{}{
		"debug": {}, "info": {}, "warn": {}, "error": {},
		"dpanic": {}, "panic": {}, "fatal": {}, "default": {},
	}
)

// ClientOptions configures a Client. A zero Timeout uses LevelTimeout.
// BaseURL must be an http loopback origin with a port.
type ClientOptions struct {
	BaseURL string
	Timeout time.Duration
}

// Client calls Kubo log/level and log/ls on loopback.
type Client struct {
	base    *url.URL
	http    *http.Client
	timeout time.Duration
}

// New checks the base URL and builds a client that dials loopback only.
func New(opt ClientOptions) (*Client, error) {
	base, err := kuborpc.ParseBase(opt.BaseURL)
	if err != nil {
		return nil, err
	}
	if opt.Timeout <= 0 {
		opt.Timeout = LevelTimeout
	}
	return &Client{base: base, http: kuborpc.Client(), timeout: opt.Timeout}, nil
}

// CheckLevel reports whether subsystem and level are ones this panel will send.
func CheckLevel(subsystem, level string) error {
	subsystem = strings.TrimSpace(subsystem)
	level = strings.TrimSpace(strings.ToLower(level))
	if subsystem != "all" && subsystem != "*" && !subsystemRE.MatchString(subsystem) {
		return &Error{Code: "bad_subsystem"}
	}
	if _, ok := levels[level]; !ok {
		return &Error{Code: "bad_level"}
	}
	return nil
}

// SetLevel sets one subsystem and returns Kubo's message.
func (c *Client) SetLevel(ctx context.Context, subsystem, level string) (string, error) {
	subsystem = strings.TrimSpace(subsystem)
	level = strings.TrimSpace(strings.ToLower(level))
	if err := CheckLevel(subsystem, level); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	body, status, err := c.do(ctx, "log/level", url.Values{"arg": {subsystem, level}}, maxLevelBody)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", &Error{Code: "kubo_rejected", Detail: kuboDetail(body)}
	}
	var out struct {
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &out); err != nil || strings.TrimSpace(out.Message) == "" {
		return "", &Error{Code: "kubo_rejected", Detail: clipDetail(string(body))}
	}
	return strings.TrimSpace(out.Message), nil
}

// Subsystems lists the logging subsystems Kubo currently has.
func (c *Client) Subsystems(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	body, status, err := c.do(ctx, "log/ls", nil, maxListBody)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, &Error{Code: "kubo_rejected", Detail: kuboDetail(body)}
	}
	var out struct {
		Strings []string `json:"Strings"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, &Error{Code: "kubo_rejected"}
	}
	names := make([]string, 0, len(out.Strings))
	seen := map[string]struct{}{}
	for _, name := range out.Strings {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (c *Client) do(ctx context.Context, cmd string, q url.Values, limit int) ([]byte, int, error) {
	endpoint := c.base.ResolveReference(&url.URL{Path: "/api/v0/" + cmd, RawQuery: q.Encode()})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, 0, &Error{Code: "unavailable"}
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, classify(err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return nil, resp.StatusCode, classify(err)
	}
	if len(buf) > limit {
		return nil, resp.StatusCode, &Error{Code: "too_large"}
	}
	return buf, resp.StatusCode, nil
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	if CodeOf(err) != "" {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: "timeout"}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &Error{Code: "timeout"}
	}
	if kuborpc.IsRefused(err) {
		return &Error{Code: "node_not_running"}
	}
	return &Error{Code: "unavailable"}
}

func kuboDetail(body []byte) string {
	var wrap struct {
		Message string `json:"Message"`
	}
	if json.Unmarshal(body, &wrap) == nil && strings.TrimSpace(wrap.Message) != "" {
		return clipDetail(wrap.Message)
	}
	return clipDetail(string(body))
}
