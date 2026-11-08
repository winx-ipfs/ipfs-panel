package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/files"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type filesHTTP struct {
	sup             *node.Supervisor
	maxBytes        int64
	op              time.Duration
	transferTimeout time.Duration
	status          func(context.Context) (node.Overview, error)
	lookup          func(repo string) (apiBase, gatewayBase string, err error)
}

type filesGate struct {
	client  *files.Client
	gateway string
}

type fileEntryJSON struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	CID      string `json:"cid,omitempty"`
	Modified string `json:"modified,omitempty"`
}

func (h *filesHTTP) list(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	p, err := queryPath(r, true)
	if err != nil {
		writeFilesErr(w, "ls", err)
		return
	}
	if p != "/" {
		st, statErr := g.client.Stat(r.Context(), p)
		if statErr != nil {
			writeFilesErr(w, "ls", statErr)
			return
		}
		if st.Type != "directory" {
			writeFilesErr(w, "ls", &files.OpError{Code: "invalid_path"})
			return
		}
	}
	entries, err := g.client.List(r.Context(), p)
	if err != nil {
		writeFilesErr(w, "ls", err)
		return
	}
	out := make([]fileEntryJSON, 0, len(entries))
	for _, entry := range entries {
		out = append(out, toEntryJSON(entry))
	}
	body := map[string]any{
		"path":     p,
		"maxBytes": g.client.MaxBytes(),
		"entries":  out,
	}
	if g.gateway != "" {
		body["localGateway"] = g.gateway
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *filesHTTP) stat(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	p, err := queryPath(r, true)
	if err != nil {
		writeFilesErr(w, "stat", err)
		return
	}
	entry, err := g.client.Stat(r.Context(), p)
	if err != nil {
		writeFilesErr(w, "stat", err)
		return
	}
	writeJSON(w, http.StatusOK, toEntryJSON(entry))
}

func (h *filesHTTP) download(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	p, err := queryPath(r, false)
	if err != nil {
		writeFilesErr(w, "read", err)
		return
	}
	st, err := g.client.Stat(r.Context(), p)
	if err != nil {
		writeFilesErr(w, "read", err)
		return
	}
	if st.Type == "directory" {
		writeFilesErr(w, "read", &files.OpError{Code: "invalid_path"})
		return
	}
	body, err := g.client.Open(r.Context(), p)
	if err != nil {
		writeFilesErr(w, "read", err)
		return
	}
	defer func() { _ = body.Close() }()
	name, err := files.Base(p)
	if err != nil {
		name = "download"
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", attachmentDisposition(name))
	w.Header().Set("Cache-Control", "no-store")
	if st.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

func (h *filesHTTP) mkdir(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	if err := g.client.Mkdir(r.Context(), body.Path); err != nil {
		writeFilesErr(w, "mkdir", err)
		return
	}
	p, err := files.Clean(body.Path)
	if err != nil {
		writeFilesErr(w, "mkdir", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": p})
}

func (h *filesHTTP) upload(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/octet-stream" {
		writeProblem(w, &statusError{code: http.StatusUnsupportedMediaType, msg: "unsupported_media_type"})
		return
	}
	p, pathErr := queryPath(r, false)
	if pathErr != nil {
		writeFilesErr(w, "write", pathErr)
		return
	}
	limit := h.maxBytes
	if limit <= 0 {
		limit = files.DefaultMaxBytes
	}
	if r.ContentLength > limit {
		writeFilesErr(w, "write", &files.OpError{Code: "too_large"})
		return
	}
	overwrite := r.URL.Query().Get("overwrite") == "1" || strings.EqualFold(r.URL.Query().Get("overwrite"), "true")
	entry, err := g.client.Write(r.Context(), p, r.Body, overwrite)
	if err != nil {
		writeFilesErr(w, "write", err)
		return
	}
	writeJSON(w, http.StatusCreated, toEntryJSON(entry))
}

func (h *filesHTTP) rename(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	dest, err := g.client.Rename(r.Context(), body.Path, body.Name)
	if err != nil {
		writeFilesErr(w, "rename", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": dest})
}

func (h *filesHTTP) move(w http.ResponseWriter, r *http.Request) {
	h.transfer(w, r, "move", func(ctx context.Context, g filesGate, from, to string) error {
		return g.client.Move(ctx, from, to)
	})
}

func (h *filesHTTP) copy(w http.ResponseWriter, r *http.Request) {
	h.transfer(w, r, "copy", func(ctx context.Context, g filesGate, from, to string) error {
		return g.client.Copy(ctx, from, to)
	})
}

func (h *filesHTTP) transfer(w http.ResponseWriter, r *http.Request, op string, fn func(context.Context, filesGate, string, string) error) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	var body struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	if err := fn(r.Context(), g, body.From, body.To); err != nil {
		writeFilesErr(w, op, err)
		return
	}
	to, err := files.Clean(body.To)
	if err != nil {
		writeFilesErr(w, op, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": to})
}

func (h *filesHTTP) delete(w http.ResponseWriter, r *http.Request) {
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	if err := g.client.Remove(r.Context(), body.Path); err != nil {
		writeFilesErr(w, "rm", err)
		return
	}
	p, err := files.Clean(body.Path)
	if err != nil {
		writeFilesErr(w, "rm", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": p})
}

func (h *filesHTTP) open(w http.ResponseWriter, r *http.Request) (filesGate, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return filesGate{}, false
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
		return filesGate{}, false
	}
	apiBase, gateway, err := h.endpoints(ov.RepoPath)
	if err != nil {
		slog.Warn("files endpoints", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return filesGate{}, false
	}
	client, err := files.New(files.Options{
		BaseURL:         apiBase,
		MaxBytes:        h.maxBytes,
		OpTimeout:       h.op,
		TransferTimeout: h.transferTimeout,
	})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return filesGate{}, false
	}
	return filesGate{client: client, gateway: gateway}, true
}

func (h *filesHTTP) overview(ctx context.Context) (node.Overview, error) {
	if h.status != nil {
		return h.status(ctx)
	}
	return h.sup.Cached(ctx)
}

func (h *filesHTTP) endpoints(repo string) (string, string, error) {
	if h.lookup != nil {
		return h.lookup(repo)
	}
	return node.LoopbackEndpoints(repo)
}

func queryPath(r *http.Request, allowEmpty bool) (string, error) {
	raw := r.URL.Query().Get("path")
	if raw == "" {
		if allowEmpty {
			return "/", nil
		}
		return "", &files.OpError{Code: "invalid_path"}
	}
	return files.Clean(raw)
}

func toEntryJSON(entry files.Entry) fileEntryJSON {
	out := fileEntryJSON{
		Name: entry.Name,
		Type: entry.Type,
		Size: entry.Size,
		CID:  entry.CID,
	}
	if !entry.Modified.IsZero() {
		out.Modified = entry.Modified.UTC().Format(time.RFC3339)
	}
	return out
}

func writeFilesErr(w http.ResponseWriter, op string, err error) {
	code := files.CodeOf(err)
	if code == "" {
		code = "unavailable"
	}
	var status int
	switch code {
	case "invalid_path":
		status = http.StatusBadRequest
	case "not_found":
		status = http.StatusNotFound
	case "already_exists":
		status = http.StatusConflict
	case "too_large":
		status = http.StatusRequestEntityTooLarge
	case "timeout":
		status = http.StatusGatewayTimeout
	case "kubo_error":
		status = http.StatusBadGateway
	case "node_not_running":
		status = http.StatusServiceUnavailable
	default:
		code = "unavailable"
		status = http.StatusServiceUnavailable
	}
	if code == "kubo_error" || code == "unavailable" || code == "timeout" || code == "node_not_running" {
		slog.Warn("files", "op", op, "code", code)
	}
	body := map[string]string{"error": code}
	if code == "node_not_running" {
		body["state"] = node.StateDown
	}
	writeJSON(w, status, body)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) *statusError {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &statusError{code: http.StatusUnsupportedMediaType, msg: "unsupported_media_type"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	return nil
}

func attachmentDisposition(name string) string {
	fallback := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if strings.Trim(fallback, "._") == "" {
		fallback = "download"
	}
	return "attachment; filename=\"" + fallback + "\"; filename*=UTF-8''" + url.PathEscape(name)
}
