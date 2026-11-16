package logs

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const redacted = "[redacted]"

// secretKeys are slog attribute names whose values never enter the ring or a file.
var secretKeys = map[string]struct{}{
	"setup_token":    {},
	"password":       {},
	"privkey":        {},
	"privatekey":     {},
	"private_key":    {},
	"authorization":  {},
	"authorizations": {},
	"csrf":           {},
	"csrf_token":     {},
	"session":        {},
	"cookie":         {},
	"token":          {},
	"secret":         {},
	"apikey":         {},
	"api_key":        {},
}

var textSecrets = []*regexp.Regexp{
	regexp.MustCompile(`(?i)("?(?:identity\.)?privkey"?\s*[:=]\s*)(?:"[^"\n]*"|'[^'\n]*'|\S+)`),
	regexp.MustCompile(`(?i)("?private[ _]?key"?\s*[:=]\s*)(?:"[^"\n]*"|'[^'\n]*'|\S+)`),
	regexp.MustCompile(`(?i)(setup_token\s*[:=]\s*)(?:"[^"\n]*"|'[^'\n]*'|\S+)`),
	regexp.MustCompile(`(?i)(ipfs-panel setup token:\s*)\S+`),
	regexp.MustCompile(`(?i)(ipfs_panel_session=)[^;\s]+`),
	regexp.MustCompile(`(?i)(x-csrf-token\s*[:=]\s*)(?:"[^"\n]*"|'[^'\n]*'|\S+)`),
	regexp.MustCompile(`(?i)(bearer\s+)\S+`),
	regexp.MustCompile(`(?i)(api\.key\s*[:=]\s*)(?:"[^"\n]*"|'[^'\n]*'|\S+)`),
}

// secretAttr reports whether an slog key should have its value replaced.
func secretAttr(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if i := strings.LastIndex(key, "."); i >= 0 {
		key = key[i+1:]
	}
	_, ok := secretKeys[key]
	return ok
}

// redactText masks secrets in a log message. A line with nothing to mask is returned unchanged.
func redactText(s string) string {
	if s == "" {
		return ""
	}
	out := s
	for _, re := range textSecrets {
		out = re.ReplaceAllString(out, "${1}"+redacted)
	}
	return out
}

// stripControls drops bytes that would spoof the event stream or the UI, except tab.
func stripControls(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == ' ' || r >= 0x20 && r != 0x7f {
			return r
		}
		return -1
	}, s)
}

// capLine cuts s to at most maxLineBytes on a UTF-8 boundary.
func capLine(s string) string {
	if len(s) <= maxLineBytes {
		return s
	}
	cut := s[:maxLineBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// clipDetail is the Kubo message shown beside kubo_rejected.
func clipDetail(s string) string {
	s = strings.TrimSpace(stripControls(s))
	if len(s) <= 240 {
		return s
	}
	cut := s[:240]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
