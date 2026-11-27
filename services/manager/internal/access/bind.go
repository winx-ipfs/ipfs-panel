package access

import (
	"fmt"
	"net"
	"strings"
)

// RemoteBind reports whether addr accepts connections from another machine.
// localhost, 127.0.0.1, and ::1 are local. A wildcard, an empty host, any
// other IP, and any other name are remote. The name is not resolved.
func RemoteBind(addr string) (bool, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, fmt.Errorf("listen address %q: use host:port, for IPv6 use [ip]:port", addr)
	}
	return !loopbackHost(host), nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func wildcardHost(host string) bool {
	return host == "" || host == "0.0.0.0" || host == "::"
}

// listenNetwork keeps an IPv4 address on IPv4. "tcp" is only for a hostname
// or an empty host, where the stack may choose.
func listenNetwork(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return "tcp"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "tcp"
	}
	if ip.To4() != nil {
		return "tcp4"
	}
	return "tcp6"
}

// Hostname returns the host part of a Host header or host:port string.
func Hostname(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return trimBrackets(host)
	}
	return trimBrackets(hostport)
}

func trimBrackets(host string) string {
	return strings.Trim(host, "[]")
}

// SplitList splits a comma-separated flag value. Empty pieces are dropped.
func SplitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ParseNets parses IPs and CIDRs. A bare IP is a single address.
func ParseNets(values []string) ([]*net.IPNet, error) {
	var out []*net.IPNet
	for _, value := range values {
		n, err := parseNet(value)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", value, err)
		}
		out = append(out, n)
	}
	return out, nil
}

func parseNet(value string) (*net.IPNet, error) {
	if strings.Contains(value, "/") {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, err
		}
		ones, bits := network.Mask.Size()
		ip := network.IP
		if v4 := ip.To4(); v4 != nil {
			ip = v4
			if bits == 128 {
				ones -= 96
				bits = 32
			}
		}
		mask := net.CIDRMask(ones, bits)
		return &net.IPNet{IP: ip.Mask(mask), Mask: mask}, nil
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return nil, fmt.Errorf("not an IP or CIDR")
	}
	ip = canonIP(ip)
	bits := 128
	if ip.To4() != nil {
		bits = 32
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}, nil
}

func canonIP(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}

// ProxyPeer reports whether remoteHost is a proxy the manager should believe.
// Loopback is always trusted when the caller has already decided the flag is on.
func ProxyPeer(remoteHost string, extra []*net.IPNet) bool {
	ip := net.ParseIP(Hostname(remoteHost))
	if ip == nil {
		return false
	}
	ip = canonIP(ip)
	if ip.IsLoopback() {
		return true
	}
	for _, network := range extra {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// RightmostForwarded returns the last X-Forwarded-For entry that is an IP.
// That is the entry a single proxy appends. Earlier entries are the client's.
func RightmostForwarded(header string) string {
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		item := strings.TrimSpace(parts[i])
		ip := net.ParseIP(item)
		if ip == nil {
			continue
		}
		return canonIP(ip).String()
	}
	return ""
}
