// Package configedit reads and replaces the Kubo config through the loopback
// RPC. config/show is scrubbed again before it can leave this package, and
// config/replace never receives the private key.
package configedit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
)

const defaultTimeout = 20 * time.Second

// Client talks to one Kubo RPC.
type Client struct {
	base    *url.URL
	http    *http.Client
	timeout time.Duration
}

// NewClient checks the base URL and builds a client that dials loopback only.
// A non-positive timeout uses the default.
func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	base, err := kuborpc.ParseBase(baseURL)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{base: base, http: kuborpc.Client(), timeout: timeout}, nil
}

// Show returns the raw config/show body. The caller redacts it.
func (c *Client) Show(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	endpoint := c.base.ResolveReference(&url.URL{Path: "/api/v0/config/show"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, code(http.StatusServiceUnavailable, "unavailable")
	}
	return c.do(req)
}

// Replace sends document as the config/replace file argument.
func (c *Client) Replace(ctx context.Context, document []byte) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "config.json")
	if err != nil {
		return code(http.StatusServiceUnavailable, "unavailable")
	}
	if _, err := part.Write(document); err != nil {
		return code(http.StatusServiceUnavailable, "unavailable")
	}
	if err := writer.Close(); err != nil {
		return code(http.StatusServiceUnavailable, "unavailable")
	}
	endpoint := c.base.ResolveReference(&url.URL{Path: "/api/v0/config/replace"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), &buf)
	if err != nil {
		return code(http.StatusServiceUnavailable, "unavailable")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	_, err = c.do(req)
	return err
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classify(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxDocument+1))
	if err != nil {
		return nil, classify(err)
	}
	if len(body) > MaxDocument {
		return nil, code(http.StatusBadGateway, "kubo_rejected")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, codeDetail(http.StatusBadGateway, "kubo_rejected", kuboDetail(body))
	}
	return body, nil
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	if CodeOf(err) != "" {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return code(http.StatusGatewayTimeout, "timeout")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return code(http.StatusGatewayTimeout, "timeout")
	}
	if kuborpc.IsRefused(err) {
		return code(http.StatusServiceUnavailable, "node_not_running")
	}
	return code(http.StatusServiceUnavailable, "unavailable")
}
