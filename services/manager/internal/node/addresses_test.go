package node

import (
	"encoding/json"
	"testing"
)

func TestLoopbackAddresses(t *testing.T) {
	ok := []string{
		"/ip4/127.0.0.1/tcp/5001",
		"/ip6/::1/tcp/8081",
		"/ip4/127.0.0.1/tcp/8081/http",
	}
	for _, addr := range ok {
		if !isLoopback(addr) {
			t.Fatalf("%s should be loopback", addr)
		}
	}
	bad := []string{
		"/ip4/0.0.0.0/tcp/5001",
		"/ip4/127.0.0.2/tcp/5001",
		"/ip6/::/tcp/8081",
		"/dns4/localhost/tcp/5001",
		"127.0.0.1:5001",
		"",
	}
	for _, addr := range bad {
		if isLoopback(addr) {
			t.Fatalf("%s should not be loopback", addr)
		}
	}
}

func TestGatewayPort(t *testing.T) {
	if gatewayPort("127.0.0.1:8080") != 8081 {
		t.Fatal("8080")
	}
	if gatewayPort("127.0.0.1:8081") != 8082 {
		t.Fatal("8081")
	}
	if gatewayPort("[::1]:8081") != 8082 {
		t.Fatal("ipv6 manager")
	}
}

func TestValidateAddresses(t *testing.T) {
	loopAPI := raw(`"/ip4/127.0.0.1/tcp/5001"`)
	loopGW := raw(`["/ip4/127.0.0.1/tcp/8081"]`)
	if err := validateAddressDoc(addressDoc{API: loopAPI, Gateway: loopGW}, gatewayFlags{}, "127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	ip6 := addressDoc{
		API:     raw(`"/ip6/::1/tcp/5001"`),
		Gateway: raw(`["/ip6/::1/tcp/8081"]`),
	}
	if err := validateAddressDoc(ip6, gatewayFlags{}, "127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}
	readonly := gatewayFlags{known: true, noFetch: true, noDNSLink: true, routingOff: true}
	public := addressDoc{API: loopAPI, Gateway: raw(`["/ip4/0.0.0.0/tcp/8081","/ip6/::/tcp/8081"]`)}
	if err := validateAddressDoc(public, readonly, "127.0.0.1:8080"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		doc  addressDoc
		addr string
		code string
	}{
		{
			name: "public api",
			doc:  addressDoc{API: raw(`"/ip4/0.0.0.0/tcp/5001"`), Gateway: loopGW},
			addr: "127.0.0.1:8080",
			code: CodeAPINotLoopback,
		},
		{
			name: "api list with public",
			doc:  addressDoc{API: raw(`["/ip4/127.0.0.1/tcp/5001","/ip4/0.0.0.0/tcp/5001"]`), Gateway: loopGW},
			addr: "127.0.0.1:8080",
			code: CodeAPINotLoopback,
		},
		{
			name: "empty api",
			doc:  addressDoc{API: raw(`[]`), Gateway: loopGW},
			addr: "127.0.0.1:8080",
			code: CodeAPINotLoopback,
		},
		{
			name: "public gateway without read-only flags",
			doc:  addressDoc{API: loopAPI, Gateway: raw(`["/ip4/0.0.0.0/tcp/8081"]`)},
			addr: "127.0.0.1:8080",
			code: CodeGatewayNotReadonly,
		},
		{
			name: "specific public address",
			doc:  addressDoc{API: loopAPI, Gateway: raw(`["/ip4/203.0.113.5/tcp/8081"]`)},
			addr: "127.0.0.1:8080",
			code: CodeGatewayNotLoopback,
		},
		{
			name: "empty gateway",
			doc:  addressDoc{API: loopAPI, Gateway: raw(`null`)},
			addr: "127.0.0.1:8080",
			code: CodeGatewayNotLoopback,
		},
		{
			name: "api uses manager port",
			doc:  addressDoc{API: raw(`"/ip4/127.0.0.1/tcp/8080"`), Gateway: loopGW},
			addr: "127.0.0.1:8080",
			code: CodePanelPortConflict,
		},
		{
			name: "gateway uses manager port",
			doc:  addressDoc{API: loopAPI, Gateway: raw(`["/ip4/127.0.0.1/tcp/8080"]`)},
			addr: "127.0.0.1:8080",
			code: CodePanelPortConflict,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAddressDoc(tc.doc, gatewayFlags{}, tc.addr)
			if err == nil || err.code != tc.code {
				t.Fatalf("got %#v", err)
			}
		})
	}
}

func TestPortInUseText(t *testing.T) {
	if !portInUseText("listen tcp 127.0.0.1:5001: bind: address already in use") {
		t.Fatal("unix")
	}
	if !portInUseText("Only one usage of each socket address") {
		t.Fatal("windows")
	}
	if portInUseText("daemon exited") {
		t.Fatal("unrelated")
	}
}

func TestWithIPFSPathReplacesParent(t *testing.T) {
	env := withIPFSPath([]string{"IPFS_PATH=/home/me/.ipfs", "PATH=/bin", "IPFS_PATH=/also"}, "/panel/repo")
	found := 0
	for _, entry := range env {
		if len(entry) >= 10 && entry[:10] == "IPFS_PATH=" {
			found++
			if entry != "IPFS_PATH=/panel/repo" {
				t.Fatalf("entry %s", entry)
			}
		}
	}
	if found != 1 {
		t.Fatalf("count %d in %v", found, env)
	}
}

func raw(s string) json.RawMessage { return json.RawMessage(s) }
