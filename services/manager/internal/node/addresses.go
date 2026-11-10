package node

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// GatewayListenPort is the gateway port the panel chose for this manager
// address: 8081, or 8082 when the manager is already on 8081.
func GatewayListenPort(managerAddr string) int {
	return gatewayPort(managerAddr)
}

func gatewayPort(managerAddr string) int {
	port, err := managerPort(managerAddr)
	if err == nil && port == 8081 {
		return 8082
	}
	return 8081
}

func managerPort(addr string) (int, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("manager address %q: %w", addr, err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("manager port %q", port)
	}
	return n, nil
}

func apiMultiaddr() string {
	return "/ip4/127.0.0.1/tcp/5001"
}

func gatewayMultiaddr(managerAddr string) string {
	return fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort(managerAddr))
}

// addressKind is "string" or "array", matching the JSON Kubo stored.
func addressValues(raw json.RawMessage) (kind string, values []string, err error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil, nil
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", nil, err
		}
		return "string", []string{s}, nil
	case '[':
		var list []string
		if err := json.Unmarshal(raw, &list); err != nil {
			return "", nil, err
		}
		return "array", list, nil
	default:
		return "", nil, fmt.Errorf("address JSON must be a string or a list of strings")
	}
}

func isLoopback(addr string) bool {
	parts := strings.Split(addr, "/")
	if len(parts) < 3 || parts[0] != "" {
		return false
	}
	switch parts[1] {
	case "ip4":
		return parts[2] == "127.0.0.1"
	case "ip6":
		return parts[2] == "::1"
	default:
		return false
	}
}

func tcpPort(addr string) (int, bool) {
	parts := strings.Split(addr, "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] != "tcp" {
			continue
		}
		n, err := strconv.Atoi(parts[i+1])
		if err != nil || n < 1 || n > 65535 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

func usesPort(addrs []string, port int) bool {
	for _, addr := range addrs {
		if p, ok := tcpPort(addr); ok && p == port {
			return true
		}
	}
	return false
}

func allLoopback(addrs []string) bool {
	if len(addrs) == 0 {
		return false
	}
	for _, addr := range addrs {
		if !isLoopback(addr) {
			return false
		}
	}
	return true
}

type addressDoc struct {
	API     json.RawMessage `json:"API"`
	Gateway json.RawMessage `json:"Gateway"`
}

// gatewayFlags is the read-only public gateway allowance.
// routingOff is true only when ExposeRoutingAPI is explicitly false.
type gatewayFlags struct {
	known      bool
	noFetch    bool
	noDNSLink  bool
	routingOff bool
}

func validateAddressDoc(doc addressDoc, flags gatewayFlags, managerAddr string) *codedError {
	_, api, err := addressValues(doc.API)
	if err != nil || !allLoopback(api) {
		return codeErr(CodeAPINotLoopback, "API address is not loopback")
	}
	_, gw, err := addressValues(doc.Gateway)
	if err != nil {
		return codeErr(CodeGatewayNotLoopback, "gateway address is not loopback")
	}
	port, err := managerPort(managerAddr)
	if err != nil {
		return codeErr(CodeUnknown, err.Error())
	}
	if usesPort(api, port) || usesPort(gw, port) {
		return codeErr(CodePanelPortConflict, "API or gateway uses the manager port")
	}
	if allLoopback(gw) {
		return nil
	}
	if publicGatewayShape(gw, GatewayListenPort(managerAddr)) {
		if flags.known && flags.noFetch && flags.noDNSLink && flags.routingOff {
			return nil
		}
		return codeErr(CodeGatewayNotReadonly, "public gateway is not read-only")
	}
	return codeErr(CodeGatewayNotLoopback, "gateway address is not loopback")
}

func publicGatewayShape(addrs []string, port int) bool {
	if len(addrs) == 0 {
		return false
	}
	saw := false
	for _, addr := range addrs {
		if isLoopback(addr) {
			continue
		}
		if addr != fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port) && addr != fmt.Sprintf("/ip6/::/tcp/%d", port) {
			return false
		}
		saw = true
	}
	return saw
}

func flagsFromConfig(data []byte) gatewayFlags {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return gatewayFlags{}
	}
	raw := bytes.TrimSpace(root["Gateway"])
	if len(raw) == 0 || raw[0] != '{' {
		return gatewayFlags{}
	}
	var gw struct {
		NoFetch          *bool `json:"NoFetch"`
		NoDNSLink        *bool `json:"NoDNSLink"`
		ExposeRoutingAPI *bool `json:"ExposeRoutingAPI"`
	}
	if err := json.Unmarshal(raw, &gw); err != nil {
		return gatewayFlags{}
	}
	flags := gatewayFlags{known: true}
	if gw.NoFetch != nil {
		flags.noFetch = *gw.NoFetch
	}
	if gw.NoDNSLink != nil {
		flags.noDNSLink = *gw.NoDNSLink
	}
	if gw.ExposeRoutingAPI != nil && !*gw.ExposeRoutingAPI {
		flags.routingOff = true
	}
	return flags
}

// ConfigError is a stable code from checking a Kubo config file.
type ConfigError struct {
	Code   string
	Detail string
}

func (e *ConfigError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

// AssessConfig reports whether the panel is willing to start a daemon with
// this config file. The API must be loopback. A public gateway is allowed
// only in the read-only shape from the scenes spec.
func AssessConfig(data []byte, managerAddr string) error {
	var root struct {
		Addresses addressDoc `json:"Addresses"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return &ConfigError{Code: CodeUnknown, Detail: "config is not JSON"}
	}
	if err := validateAddressDoc(root.Addresses, flagsFromConfig(data), managerAddr); err != nil {
		return &ConfigError{Code: err.code, Detail: err.detail}
	}
	return nil
}

func portInUseText(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "address already in use") ||
		strings.Contains(l, "address is already in use") ||
		strings.Contains(l, "only one usage of each socket address")
}
