package pins

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
)

func classify(err error) error {
	if err == nil {
		return nil
	}
	if CodeOf(err) != "" {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return &OpError{Code: "cancelled"}
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
	if unsupportedArg(status, body, "names") {
		return &OpError{Code: "names_unsupported"}
	}
	if unsupportedArg(status, body, "name") {
		return &OpError{Code: "name_unsupported"}
	}
	msg := strings.ToLower(kuboMessage(body))
	switch {
	case strings.Contains(msg, "indirect"):
		return &OpError{Code: "indirect"}
	case strings.Contains(msg, "not pinned"), strings.Contains(msg, "not found"), strings.Contains(msg, "no such"), strings.Contains(msg, "does not exist"):
		return &OpError{Code: "not_found"}
	default:
		if status == http.StatusNotFound {
			return &OpError{Code: "not_found"}
		}
		return &OpError{Code: "kubo_error"}
	}
}

func unsupportedArg(status int, body []byte, arg string) bool {
	if status < 400 || status >= 500 {
		return false
	}
	msg := strings.ToLower(kuboMessage(body))
	if !mentionsArg(msg, arg) {
		return false
	}
	return strings.Contains(msg, "argument") || strings.Contains(msg, "option") || strings.Contains(msg, "flag") ||
		strings.Contains(msg, "unrecognized") || strings.Contains(msg, "unused") || strings.Contains(msg, "unknown")
}

func mentionsArg(msg, arg string) bool {
	if strings.Contains(msg, `"`+arg+`"`) || strings.Contains(msg, "'"+arg+"'") {
		return true
	}
	for _, field := range strings.FieldsFunc(msg, func(r rune) bool {
		return r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if field == arg {
			return true
		}
	}
	return false
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
