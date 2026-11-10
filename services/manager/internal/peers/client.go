// Package peers reads the swarm and dials one address on a loopback Kubo RPC.
// It does not return peer multiaddrs to the caller of List.
package peers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
)

const (
	// ListTimeout is the deadline for swarm/peers. The call reads the local
	// peerstore. It does not dial the peers.
	ListTimeout = 20 * time.Second

	// ConnectTimeout is the deadline for one swarm/connect dial.
	ConnectTimeout = 30 * time.Second

	maxListBody    = 8 << 20
	maxConnectBody = 1 << 20
)

// Options configures a Client. Zero timeouts use the defaults.
// BaseURL must be an http loopback origin with a port.
type Options struct {
	BaseURL        string
	ListTimeout    time.Duration
	ConnectTimeout time.Duration
}

// Client talks to one Kubo RPC.
type Client struct {
	base           *url.URL
	http           *http.Client
	listTimeout    time.Duration
	connectTimeout time.Duration
}

// New checks the base URL and builds a client that dials loopback only.
func New(opt Options) (*Client, error) {
	base, err := kuborpc.ParseBase(opt.BaseURL)
	if err != nil {
		return nil, err
	}
	if opt.ListTimeout <= 0 {
		opt.ListTimeout = ListTimeout
	}
	if opt.ConnectTimeout <= 0 {
		opt.ConnectTimeout = ConnectTimeout
	}
	return &Client{
		base:           base,
		http:           kuborpc.Client(),
		listTimeout:    opt.ListTimeout,
		connectTimeout: opt.ConnectTimeout,
	}, nil
}

// List returns the distinct connected peers.
func (c *Client) List(ctx context.Context) (List, error) {
	ctx, cancel := context.WithTimeout(ctx, c.listTimeout)
	defer cancel()
	q := url.Values{
		"latency":   {"true"},
		"direction": {"true"},
		"identify":  {"true"},
	}
	body, status, err := c.do(ctx, "swarm/peers", q, maxListBody)
	if err != nil {
		return List{}, err
	}
	if status != http.StatusOK {
		return List{}, &OpError{Code: "kubo_error"}
	}
	return ParseList(body)
}

// Connect dials one address and returns the peer id parsed from it.
func (c *Client) Connect(ctx context.Context, address string) (string, error) {
	dial, peerID, err := ParseAddress(address)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, c.connectTimeout)
	defer cancel()
	body, status, err := c.do(ctx, "swarm/connect", url.Values{"arg": {dial}}, maxConnectBody)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", &OpError{Code: "connect_failed"}
	}
	var discard any
	if err := json.Unmarshal(body, &discard); err != nil {
		return "", &OpError{Code: "kubo_error"}
	}
	return peerID, nil
}

func (c *Client) do(ctx context.Context, cmd string, q url.Values, limit int) ([]byte, int, error) {
	endpoint := c.base.ResolveReference(&url.URL{Path: "/api/v0/" + cmd, RawQuery: q.Encode()})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, 0, &OpError{Code: "unavailable"}
	}
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
		if limit == maxListBody {
			return nil, resp.StatusCode, &OpError{Code: "too_large"}
		}
		return nil, resp.StatusCode, &OpError{Code: "kubo_error"}
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
		return &OpError{Code: "timeout"}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &OpError{Code: "timeout"}
	}
	if kuborpc.IsRefused(err) {
		return &OpError{Code: "node_not_running"}
	}
	return &OpError{Code: "unavailable"}
}
