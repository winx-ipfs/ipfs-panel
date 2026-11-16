package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/logs"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type levelClient interface {
	SetLevel(ctx context.Context, subsystem, level string) (string, error)
	Subsystems(ctx context.Context) ([]string, error)
}

type logsHTTP struct {
	sup     *node.Supervisor
	store   *logs.Store
	log     *slog.Logger
	trust   bool
	status  func(context.Context) (node.Overview, error)
	capture func() bool
	dial    func(context.Context, string) (levelClient, error)
}

func (h *logsHTTP) list(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	q, prob := logQuery(r)
	if prob != nil {
		writeProblem(w, prob)
		return
	}
	page, err := h.store.Query(q)
	if err != nil {
		writeLogsErr(w, err)
		return
	}
	ov, ok := h.snapshot(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, logPageJSON(page, ov, h.isCapturing()))
}

func (h *logsHTTP) stream(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	q, prob := logQuery(r)
	if prob != nil {
		writeProblem(w, prob)
		return
	}
	ov, ok := h.snapshot(w, r)
	if !ok {
		return
	}
	id, ch, err := h.store.Subscribe(q)
	if err != nil {
		writeLogsErr(w, err)
		return
	}
	defer h.store.Unsubscribe(id)
	page, err := h.store.Query(q)
	if err != nil {
		writeLogsErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := writeStatus(w, ov, h.isCapturing()); err != nil {
		return
	}
	last := q.After
	for _, line := range page.Lines {
		if err := writeLine(w, line); err != nil {
			return
		}
		last = line.ID
	}
	flusher.Flush()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case line := <-ch:
			if line.ID <= last {
				continue
			}
			if err := writeLine(w, line); err != nil {
				return
			}
			last = line.ID
			if h.store.TakeOverflow(id) {
				if _, err := fmt.Fprintf(w, "event: gap\ndata: {}\n\n"); err != nil {
					return
				}
			}
			flusher.Flush()
		case <-tick.C:
			if snap, ok := h.snapshotQuiet(r); ok {
				if err := writeStatus(w, snap, h.isCapturing()); err != nil {
					return
				}
			}
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			if h.store.TakeOverflow(id) {
				if _, err := fmt.Fprintf(w, "event: gap\ndata: {}\n\n"); err != nil {
					return
				}
			}
			flusher.Flush()
		}
	}
}

func (h *logsHTTP) download(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = logs.SourceAll
	}
	name := "ipfs-panel-logs.txt"
	switch source {
	case logs.SourceAll:
	case logs.SourceManager:
		name = "ipfs-panel-manager.txt"
	case logs.SourceKubo:
		name = "ipfs-panel-kubo.txt"
	default:
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "bad_request"})
		return
	}
	var buf bytes.Buffer
	if err := h.store.WriteDownload(&buf, source); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", attachmentDisposition(name))
	_, _ = w.Write(buf.Bytes())
}

func (h *logsHTTP) levels(w http.ResponseWriter, r *http.Request) {
	client, ok := h.openLevel(w, r)
	if !ok {
		return
	}
	names, err := client.Subsystems(r.Context())
	if err != nil {
		writeLogsErr(w, err)
		return
	}
	if names == nil {
		names = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"subsystems": names})
}

func (h *logsHTTP) setLevel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Subsystem string `json:"subsystem"`
		Level     string `json:"level"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		h.audit(r, "invalid", "", "")
		writeProblem(w, err)
		return
	}
	if err := logs.CheckLevel(body.Subsystem, body.Level); err != nil {
		h.audit(r, "invalid", body.Subsystem, body.Level)
		writeLogsErr(w, err)
		return
	}
	client, ok := h.openLevel(w, r)
	if !ok {
		h.audit(r, "unavailable", body.Subsystem, body.Level)
		return
	}
	if _, err := client.SetLevel(r.Context(), body.Subsystem, body.Level); err != nil {
		result := "rejected"
		if logs.CodeOf(err) == "bad_level" || logs.CodeOf(err) == "bad_subsystem" {
			result = "invalid"
		} else if logs.CodeOf(err) == "node_not_running" || logs.CodeOf(err) == "unavailable" || logs.CodeOf(err) == "timeout" {
			result = "unavailable"
		}
		h.audit(r, result, body.Subsystem, body.Level)
		writeLogsErr(w, err)
		return
	}
	h.audit(r, "success", body.Subsystem, body.Level)
	writeJSON(w, http.StatusOK, map[string]string{
		"subsystem": body.Subsystem,
		"level":     body.Level,
	})
}

func (h *logsHTTP) openLevel(w http.ResponseWriter, r *http.Request) (levelClient, bool) {
	ov, ok := h.snapshot(w, r)
	if !ok {
		return nil, false
	}
	if ov.State != node.StateUp {
		state := ov.State
		if state == "" {
			state = node.StateDown
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "node_not_running",
			"state": state,
		})
		return nil, false
	}
	if h.dial != nil {
		client, err := h.dial(r.Context(), ov.RepoPath)
		if err != nil {
			writeLogsErr(w, err)
			return nil, false
		}
		return client, true
	}
	apiBase, _, err := node.LoopbackEndpoints(ov.RepoPath)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	client, err := logs.New(logs.ClientOptions{BaseURL: apiBase})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	return client, true
}

func (h *logsHTTP) snapshot(w http.ResponseWriter, r *http.Request) (node.Overview, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return node.Overview{}, false
	}
	return ov, true
}

func (h *logsHTTP) snapshotQuiet(r *http.Request) (node.Overview, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		return node.Overview{}, false
	}
	return ov, true
}

func (h *logsHTTP) overview(ctx context.Context) (node.Overview, error) {
	if h.status != nil {
		return h.status(ctx)
	}
	if h.sup == nil {
		return node.Overview{}, errNoSupervisor
	}
	return h.sup.Cached(ctx)
}

func (h *logsHTTP) isCapturing() bool {
	if h.capture != nil {
		return h.capture()
	}
	if h.sup == nil {
		return false
	}
	return h.sup.Capturing()
}

func (h *logsHTTP) audit(r *http.Request, result, subsystem, level string) {
	username := ""
	if principal, ok := principalFrom(r); ok {
		username = principal.username
	}
	logger := h.log
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("audit",
		"event", "log_level",
		"result", result,
		"username", username,
		"ip", clientIP(r, h.trust),
		"subsystem", subsystem,
		"level", level,
	)
}

var errNoSupervisor = &statusError{code: http.StatusServiceUnavailable, msg: "unavailable"}

func logQuery(r *http.Request) (logs.Query, *statusError) {
	q := r.URL.Query()
	after := int64(0)
	if raw := q.Get("after"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return logs.Query{}, &statusError{code: http.StatusBadRequest, msg: "bad_request"}
		}
		after = n
	}
	limit := 0
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return logs.Query{}, &statusError{code: http.StatusBadRequest, msg: "bad_request"}
		}
		limit = n
	}
	query, err := logs.Normalize(logs.Query{
		After:  after,
		Source: q.Get("source"),
		Level:  q.Get("level"),
		Q:      q.Get("q"),
		Limit:  limit,
	})
	if err != nil {
		return logs.Query{}, &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	return query, nil
}

type logLineJSON struct {
	ID     int64  `json:"id"`
	Time   string `json:"time"`
	Source string `json:"source"`
	Level  string `json:"level"`
	Text   string `json:"text"`
}

func logPageJSON(page logs.Page, ov node.Overview, capture bool) map[string]any {
	lines := make([]logLineJSON, 0, len(page.Lines))
	for _, line := range page.Lines {
		lines = append(lines, logLineJSON{
			ID:     line.ID,
			Time:   line.Time.UTC().Format(time.RFC3339Nano),
			Source: line.Source,
			Level:  line.Level,
			Text:   line.Text,
		})
	}
	state := ov.State
	if state == "" {
		state = node.StateDown
	}
	return map[string]any{
		"lines":   lines,
		"next":    page.Next,
		"node":    state,
		"capture": capture,
	}
}

func writeLine(w http.ResponseWriter, line logs.Line) error {
	body, err := json.Marshal(logLineJSON{
		ID:     line.ID,
		Time:   line.Time.UTC().Format(time.RFC3339Nano),
		Source: line.Source,
		Level:  line.Level,
		Text:   line.Text,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: line\ndata: %s\n\n", line.ID, body)
	return err
}

func writeStatus(w http.ResponseWriter, ov node.Overview, capture bool) error {
	state := ov.State
	if state == "" {
		state = node.StateDown
	}
	body, err := json.Marshal(map[string]any{"node": state, "capture": capture})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: status\ndata: %s\n\n", body)
	return err
}

func writeLogsErr(w http.ResponseWriter, err error) {
	code := logs.CodeOf(err)
	if code == "" {
		code = "unavailable"
	}
	var status int
	switch code {
	case "bad_request", "bad_level", "bad_subsystem":
		status = http.StatusBadRequest
	case "too_large":
		status = http.StatusRequestEntityTooLarge
	case "timeout":
		status = http.StatusGatewayTimeout
	case "kubo_rejected":
		status = http.StatusBadGateway
	case "node_not_running":
		status = http.StatusServiceUnavailable
	default:
		code = "unavailable"
		status = http.StatusServiceUnavailable
	}
	body := map[string]string{"error": code}
	var op *logs.Error
	if asLogs(err, &op) && op.Detail != "" && code == "kubo_rejected" {
		body["detail"] = op.Detail
	}
	if code == "node_not_running" {
		state := node.StateDown
		if asLogs(err, &op) && op.State != "" {
			state = op.State
		}
		body["state"] = state
	}
	writeJSON(w, status, body)
}

func asLogs(err error, dest **logs.Error) bool {
	var op *logs.Error
	if !errors.As(err, &op) {
		return false
	}
	*dest = op
	return true
}
