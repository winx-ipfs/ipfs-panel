// Package activity reads peer count and bandwidth from a loopback Kubo RPC.
// It discards listen addresses before returning.
package activity

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

const maxBody = 2 << 20

// Fetch asks swarm/peers and stats/bw. A nil result means neither call
// answered. A nil rate means bandwidth was not available.
func Fetch(ctx context.Context, apiBase string) (*node.Activity, error) {
	base, err := kuborpc.ParseBase(apiBase)
	if err != nil {
		return nil, err
	}
	client := kuborpc.Client()
	act := &node.Activity{}
	ok := false
	if n, err := peers(ctx, client, base.String()); err == nil {
		act.Peers = &n
		ok = true
	}
	if in, out, err := bandwidth(ctx, client, base.String()); err == nil {
		act.RateIn = &in
		act.RateOut = &out
		ok = true
	}
	if !ok {
		return nil, errUnavailable
	}
	return act, nil
}

var errUnavailable = errString("activity unavailable")

type errString string

func (e errString) Error() string { return string(e) }

func peers(ctx context.Context, client *http.Client, base string) (int, error) {
	body, err := post(ctx, client, base+"/api/v0/swarm/peers")
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Peers []struct {
			Peer string `json:"Peer"`
		} `json:"Peers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}
	seen := map[string]struct{}{}
	for _, peer := range parsed.Peers {
		id := strings.TrimSpace(peer.Peer)
		if id == "" {
			continue
		}
		seen[id] = struct{}{}
	}
	return len(seen), nil
}

func bandwidth(ctx context.Context, client *http.Client, base string) (float64, float64, error) {
	body, err := post(ctx, client, base+"/api/v0/stats/bw")
	if err != nil {
		return 0, 0, err
	}
	var parsed struct {
		RateIn  float64 `json:"RateIn"`
		RateOut float64 `json:"RateOut"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0, err
	}
	return parsed.RateIn, parsed.RateOut, nil
}

func post(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, errUnavailable
	}
	if res.StatusCode != http.StatusOK {
		return nil, errUnavailable
	}
	return body, nil
}
