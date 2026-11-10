// Package explore reads one IPLD block from Kubo on a loopback API.
// It does not dial the gateway and it does not give the browser an RPC address.
package explore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/pins"
)

const (
	// DefaultTimeout is the deadline for one lookup. A network fetch of one
	// block fits in it. A whole DAG does not, and this page does not ask for one.
	DefaultTimeout = 60 * time.Second

	maxBody = 4 << 20
)

// Options configures a Client. A zero Timeout uses DefaultTimeout.
// BaseURL must be an http loopback origin with a port.
type Options struct {
	BaseURL string
	Timeout time.Duration
}

// Client talks to one Kubo RPC.
type Client struct {
	base    *url.URL
	http    *http.Client
	timeout time.Duration
}

// New checks the base URL and builds a client that dials loopback only.
func New(opt Options) (*Client, error) {
	base, err := kuborpc.ParseBase(opt.BaseURL)
	if err != nil {
		return nil, err
	}
	if opt.Timeout <= 0 {
		opt.Timeout = DefaultTimeout
	}
	return &Client{base: base, http: kuborpc.Client(), timeout: opt.Timeout}, nil
}

// Inspect returns the block named by a content ID or an /ipfs/ path.
func (c *Client) Inspect(ctx context.Context, target string) (Block, error) {
	norm, err := pins.ParseTarget(target)
	if err != nil {
		return Block{}, &OpError{Code: "invalid_target"}
	}
	info, err := pins.Inspect(norm)
	if err != nil {
		return Block{}, &OpError{Code: "invalid_target"}
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cid := pins.RootCID(norm)
	focus := ""
	if strings.HasPrefix(norm, "/ipfs/") && strings.Count(norm, "/") > 2 {
		resolved, rem, err := c.resolve(ctx, norm)
		if err != nil {
			return Block{}, err
		}
		info, err = pins.Inspect(resolved)
		if err != nil {
			return Block{}, &OpError{Code: "kubo_error"}
		}
		cid = resolved
		focus = rem
	}
	body, err := c.get(ctx, cid)
	if err != nil {
		return Block{}, err
	}
	block, err := interpret(info, body, focus)
	if err != nil {
		return Block{}, err
	}
	block.Target = norm
	block.CID = cid
	return block, nil
}

func (c *Client) resolve(ctx context.Context, target string) (string, string, error) {
	body, status, err := c.do(ctx, "dag/resolve", url.Values{"arg": {target}}, true)
	if err != nil {
		return "", "", err
	}
	if status >= 300 {
		return "", "", kuboResult(status, body)
	}
	cid, rem, err := parseResolve(body)
	if err != nil {
		return "", "", err
	}
	if _, err := pins.ParseTarget(cid); err != nil {
		return "", "", &OpError{Code: "kubo_error"}
	}
	return cid, rem, nil
}

func (c *Client) get(ctx context.Context, cid string) ([]byte, error) {
	q := url.Values{"arg": {cid}, "output-codec": {"dag-json"}}
	body, status, err := c.do(ctx, "dag/get", q, false)
	if err != nil {
		return nil, err
	}
	if status >= 300 {
		return nil, kuboResult(status, body)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, &OpError{Code: "kubo_error"}
	}
	return body, nil
}

func (c *Client) do(ctx context.Context, cmd string, q url.Values, jsonAccept bool) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(cmd, q), nil)
	if err != nil {
		return nil, 0, &OpError{Code: "unavailable"}
	}
	if jsonAccept {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, classify(err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, resp.StatusCode, classify(err)
	}
	if len(buf) > maxBody {
		return nil, resp.StatusCode, &OpError{Code: "too_large"}
	}
	return buf, resp.StatusCode, nil
}

func (c *Client) endpoint(cmd string, q url.Values) string {
	u := c.base.ResolveReference(&url.URL{Path: "/api/v0/" + cmd, RawQuery: q.Encode()})
	return u.String()
}

func parseResolve(body []byte) (string, string, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return "", "", &OpError{Code: "kubo_error"}
	}
	if body[0] == '{' {
		var wrap struct {
			CID json.RawMessage `json:"Cid"`
			Rem string          `json:"RemPath"`
		}
		if err := json.Unmarshal(body, &wrap); err != nil || len(wrap.CID) == 0 {
			return "", "", &OpError{Code: "kubo_error"}
		}
		cid, err := cidFromJSON(wrap.CID)
		if err != nil {
			return "", "", err
		}
		return cid, strings.Trim(wrap.Rem, "/"), nil
	}
	text := string(body)
	if i := strings.IndexByte(text, '/'); i >= 0 {
		return text[:i], strings.Trim(text[i+1:], "/"), nil
	}
	return text, "", nil
}

func cidFromJSON(raw json.RawMessage) (string, error) {
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil && plain != "" {
		return plain, nil
	}
	var boxed struct {
		Slash string `json:"/"`
	}
	if err := json.Unmarshal(raw, &boxed); err == nil && boxed.Slash != "" {
		return boxed.Slash, nil
	}
	return "", &OpError{Code: "kubo_error"}
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

func kuboResult(status int, body []byte) error {
	if status == http.StatusNotFound {
		return &OpError{Code: "not_found"}
	}
	msg := strings.ToLower(kuboMessage(body))
	switch {
	case strings.Contains(msg, "does not exist"), strings.Contains(msg, "not found"), strings.Contains(msg, "no such"), strings.Contains(msg, "no link"):
		return &OpError{Code: "not_found"}
	default:
		return &OpError{Code: "kubo_error"}
	}
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
