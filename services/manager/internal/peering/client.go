package peering

import (
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
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peers"
)

const (
	rpcTimeout = 20 * time.Second
	maxSwarm   = 8 << 20
	maxPeering = 1 << 20
)

// Client talks to one Kubo RPC. BaseURL must be an http loopback origin.
type Client struct {
	base *url.URL
	http *http.Client
}

// NewClient checks the base URL and builds a client that dials loopback only.
func NewClient(baseURL string) (*Client, error) {
	base, err := kuborpc.ParseBase(baseURL)
	if err != nil {
		return nil, err
	}
	return &Client{base: base, http: kuborpc.Client()}, nil
}

// List returns the peers in the running peering service.
func (c *Client) List(ctx context.Context) ([]Saved, error) {
	body, status, err := c.post(ctx, "swarm/peering/ls", nil, maxPeering)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, code(http.StatusBadGateway, "kubo_error")
	}
	return parseLive(body)
}

// Add registers multiaddrs with the running peering service.
func (c *Client) Add(ctx context.Context, addrs []string) error {
	if len(addrs) == 0 {
		return code(http.StatusBadRequest, "invalid_address")
	}
	_, status, err := c.post(ctx, "swarm/peering/add", url.Values{"arg": addrs}, maxPeering)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return code(http.StatusBadGateway, "kubo_error")
	}
	return nil
}

// Remove drops a peer id from the running peering service.
func (c *Client) Remove(ctx context.Context, peerID string) error {
	_, status, err := c.post(ctx, "swarm/peering/rm", url.Values{"arg": {peerID}}, maxPeering)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return code(http.StatusBadGateway, "kubo_error")
	}
	return nil
}

// Swarm returns the connected peer ids and, for the manager's own use, the
// addresses Kubo reported. Those addresses are not a browser response.
func (c *Client) Swarm(ctx context.Context) (map[string]struct{}, map[string][]string, error) {
	body, status, err := c.post(ctx, "swarm/peers", nil, maxSwarm)
	if err != nil {
		return nil, nil, err
	}
	if status != http.StatusOK {
		return nil, nil, code(http.StatusBadGateway, "kubo_error")
	}
	return parseSwarm(body)
}

func (c *Client) post(ctx context.Context, cmd string, q url.Values, limit int) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	endpoint := c.base.ResolveReference(&url.URL{Path: "/api/v0/" + cmd, RawQuery: q.Encode()})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, 0, code(http.StatusServiceUnavailable, "unavailable")
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
		return nil, resp.StatusCode, code(http.StatusRequestEntityTooLarge, "too_large")
	}
	return buf, resp.StatusCode, nil
}

func parseLive(body []byte) ([]Saved, error) {
	var doc struct {
		Peers []map[string]any `json:"Peers"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, code(http.StatusBadGateway, "kubo_error")
	}
	out := make([]Saved, 0, len(doc.Peers))
	for _, item := range doc.Peers {
		id := firstString(item, "ID", "Id", "id")
		canon, ok := peers.CanonicalPeerID(id)
		if !ok {
			continue
		}
		out = append(out, Saved{PeerID: canon, Addrs: stringList(item["Addrs"], item["addrs"])})
	}
	return out, nil
}

func parseSwarm(body []byte) (map[string]struct{}, map[string][]string, error) {
	var doc struct {
		Peers []map[string]any `json:"Peers"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, nil, code(http.StatusBadGateway, "kubo_error")
	}
	connected := map[string]struct{}{}
	addrs := map[string][]string{}
	for _, item := range doc.Peers {
		id := firstString(item, "Peer", "ID", "Id", "id")
		canon, ok := peers.CanonicalPeerID(id)
		if !ok {
			continue
		}
		connected[canon] = struct{}{}
		addr := firstString(item, "Addr", "addr")
		if addr == "" {
			continue
		}
		if !containsAddr(addrs[canon], addr) {
			addrs[canon] = append(addrs[canon], addr)
		}
	}
	return connected, addrs, nil
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := item[key].(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func stringList(values ...any) []string {
	out := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		items, _ := value.([]any)
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				continue
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			if _, dup := seen[text]; dup {
				continue
			}
			seen[text] = struct{}{}
			out = append(out, text)
		}
	}
	return out
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
