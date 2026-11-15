package logs

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var (
	ansiRE   = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	callerRE = regexp.MustCompile(`^[\w./\\-]+\.go:\d+$`)
)

func kuboLevel(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return "debug", true
	case "info":
		return "info", true
	case "warn", "warning":
		return "warn", true
	case "error", "dpanic", "panic", "fatal":
		return "error", true
	default:
		return "", false
	}
}

// ParseKubo turns one daemon line into a level, a message, and a time.
// An empty level means the line should be dropped.
func ParseKubo(raw string, now time.Time) (level, text string, at time.Time) {
	at = now.UTC()
	cleaned := strings.TrimSpace(stripControls(ansiRE.ReplaceAllString(raw, "")))
	if cleaned == "" {
		return "", "", at
	}
	if strings.HasPrefix(cleaned, "{") {
		if lv, msg, ts, ok := parseJSONLine(cleaned, at); ok {
			return lv, capLine(redactText(msg)), ts
		}
	}
	parts := strings.Split(cleaned, "\t")
	if len(parts) >= 3 {
		if lv, ok := kuboLevel(parts[1]); ok {
			if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				at = ts.UTC()
			}
			rest := make([]string, 0, len(parts)-2)
			for _, part := range parts[2:] {
				if part == "" || callerRE.MatchString(part) {
					continue
				}
				rest = append(rest, part)
			}
			text = strings.TrimSpace(strings.Join(rest, " "))
			if text == "" {
				text = cleaned
			}
			return lv, capLine(redactText(text)), at
		}
	}
	return "info", capLine(redactText(cleaned)), at
}

func parseJSONLine(s string, fallback time.Time) (level, msg string, at time.Time, ok bool) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return "", "", fallback, false
	}
	msg, _ = doc["msg"].(string)
	if msg == "" {
		msg, _ = doc["message"].(string)
	}
	if msg == "" {
		return "", "", fallback, false
	}
	level = "info"
	if raw, isStr := doc["level"].(string); isStr {
		if mapped, known := kuboLevel(raw); known {
			level = mapped
		}
	}
	at = fallback
	if ts, isStr := doc["ts"].(string); isStr {
		if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			at = parsed.UTC()
		}
	}
	return level, msg, at, true
}
