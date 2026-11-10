package node

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const maxKuboConfig = 2 << 20

// LoopbackEndpoints reads the Kubo repo config and returns HTTP origins for
// the API and the gateway. Every address must be a loopback TCP multiaddr.
// An empty gatewayBase means the share link should be hidden.
// The API origin is for the manager only. It must not be sent to the browser.
func LoopbackEndpoints(repoPath string) (apiBase, gatewayBase string, err error) {
	f, err := os.Open(filepath.Join(repoPath, "config"))
	if err != nil {
		return "", "", fmt.Errorf("kubo config: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxKuboConfig+1))
	if err != nil {
		return "", "", fmt.Errorf("kubo config: %w", err)
	}
	if len(data) > maxKuboConfig {
		return "", "", fmt.Errorf("kubo config is too large")
	}
	var root struct {
		Addresses addressDoc `json:"Addresses"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return "", "", fmt.Errorf("kubo config: %w", err)
	}
	_, api, err := addressValues(root.Addresses.API)
	if err != nil {
		return "", "", err
	}
	apiBase, err = loopbackOrigin(api)
	if err != nil {
		return "", "", err
	}
	_, gw, err := addressValues(root.Addresses.Gateway)
	if err != nil {
		return "", "", err
	}
	gatewayBase = loopbackGateway(gw)
	return apiBase, gatewayBase, nil
}

// loopbackGateway returns an HTTP origin only for a loopback gateway address.
// A public gateway has no local link. It is not an error: the API can still
// be dialed.
func loopbackGateway(addrs []string) string {
	for _, addr := range addrs {
		if !isLoopback(addr) {
			continue
		}
		origin, err := multiaddrHTTP(addr)
		if err != nil {
			continue
		}
		return origin
	}
	return ""
}

func loopbackOrigin(addrs []string) (string, error) {
	if len(addrs) == 0 {
		return "", fmt.Errorf("address list is empty")
	}
	first := ""
	for _, addr := range addrs {
		origin, err := multiaddrHTTP(addr)
		if err != nil {
			return "", err
		}
		if first == "" {
			first = origin
		}
	}
	return first, nil
}

func multiaddrHTTP(addr string) (string, error) {
	if !isLoopback(addr) {
		return "", fmt.Errorf("address is not loopback")
	}
	port, ok := tcpPort(addr)
	if !ok {
		return "", fmt.Errorf("address has no tcp port")
	}
	parts := strings.Split(addr, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("address is not loopback")
	}
	ip := net.ParseIP(parts[2])
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("address is not loopback")
	}
	return "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}
