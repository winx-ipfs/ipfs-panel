package node

import "strings"

const (
	StateDown     = "down"
	StateStarting = "starting"
	StateUp       = "up"
	StateStopping = "stopping"
	StateFailed   = "failed"
)

const (
	CodeBinaryNotFound     = "binary_not_found"
	CodeRepoNotWritable    = "repo_not_writable"
	CodeInitFailed         = "init_failed"
	CodeAPINotLoopback     = "api_not_loopback"
	CodeGatewayNotLoopback = "gateway_not_loopback"
	CodePanelPortConflict  = "panel_port_conflict"
	CodePortInUse          = "port_in_use"
	CodeRepoBusy           = "repo_busy"
	CodeDaemonExited       = "daemon_exited"
	CodeHealthTimeout      = "health_timeout"
	CodeStopTimeout        = "stop_timeout"
	CodeUnknown            = "unknown"
)

const detailLimit = 2000

// Overview is the JSON body of GET /api/node and the node action routes.
// It has no Kubo API URL, gateway URL, multiaddr, or port.
type Overview struct {
	State    string     `json:"state"`
	PeerID   *string    `json:"peerId"`
	Version  *string    `json:"version"`
	RepoPath string     `json:"repoPath"`
	Error    *NodeError `json:"error"`
}

// NodeError is a stable code plus a short detail. The UI translates code.
type NodeError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type codedError struct {
	code   string
	detail string
}

func (e *codedError) Error() string {
	if e == nil {
		return ""
	}
	if e.detail == "" {
		return e.code
	}
	return e.code + ": " + e.detail
}

func codeErr(code, detail string) *codedError {
	return &codedError{code: code, detail: capDetail(detail)}
}

func capDetail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= detailLimit {
		return s
	}
	b := []byte(s[len(s)-detailLimit:])
	for len(b) > 0 && (b[0]&0xC0) == 0x80 {
		b = b[1:]
	}
	return string(b)
}
