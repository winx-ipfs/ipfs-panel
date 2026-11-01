package api

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/files"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/pins"
)

type pinsHTTP struct {
	sup        *node.Supervisor
	dataDir    string
	maxBytes   int64
	op         time.Duration
	transfer   time.Duration
	pinTimeout time.Duration
	status     func(context.Context) (node.Overview, error)
	lookup     func(repo string) (apiBase, gatewayBase string, err error)

	once    sync.Once
	jobs    *pins.Book
	labels  *pins.Labels
	support *pins.Support
}

type pinJSON struct {
	CID  string `json:"cid"`
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
	Size *int64 `json:"size,omitempty"`
}

type jobJSON struct {
	ID       string `json:"id"`
	CID      string `json:"cid,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type"`
	State    string `json:"state"`
	Progress int    `json:"progress"`
	Error    string `json:"error,omitempty"`
	Created  string `json:"created"`
}

func (h *pinsHTTP) ensure() {
	h.once.Do(func() {
		if h.jobs == nil {
			h.jobs = pins.NewBook(pins.MaxRunning, pins.KeepFinished)
		}
		if h.labels == nil {
			h.labels = pins.OpenLabels(h.dataDir)
		}
		if h.support == nil {
			h.support = &pins.Support{}
		}
		if h.maxBytes <= 0 {
			h.maxBytes = 512 << 20
		}
		if h.op <= 0 {
			h.op = 20 * time.Second
		}
		if h.transfer <= 0 {
			h.transfer = 10 * time.Minute
		}
		if h.pinTimeout <= 0 {
			h.pinTimeout = pins.DefaultPinTimeout
		}
	})
}

func (h *pinsHTTP) list(w http.ResponseWriter, r *http.Request) {
	h.ensure()
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	found, err := g.client.List(r.Context(), h.support)
	if err != nil {
		writePinsErr(w, "ls", err)
		return
	}
	found = h.labels.Apply(found)
	sort.Slice(found, func(i, j int) bool {
		if found[i].Name == "" && found[j].Name != "" {
			return false
		}
		if found[i].Name != "" && found[j].Name == "" {
			return true
		}
		if !strings.EqualFold(found[i].Name, found[j].Name) {
			return strings.ToLower(found[i].Name) < strings.ToLower(found[j].Name)
		}
		if found[i].CID != found[j].CID {
			return found[i].CID < found[j].CID
		}
		return found[i].Type < found[j].Type
	})
	g.client.FillSizes(r.Context(), found)
	out := make([]pinJSON, 0, len(found))
	for _, pin := range found {
		out = append(out, toPinJSON(pin))
	}
	body := map[string]any{
		"namesFrom": h.support.NamesFrom(),
		"maxBytes":  h.maxBytes,
		"pins":      out,
	}
	if g.gateway != "" {
		body["localGateway"] = g.gateway
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *pinsHTTP) add(w http.ResponseWriter, r *http.Request) {
	h.ensure()
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	var body struct {
		Target    string `json:"target"`
		Name      string `json:"name"`
		Recursive *bool  `json:"recursive"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	target, err := pins.ParseTarget(body.Target)
	if err != nil {
		writePinsErr(w, "add", err)
		return
	}
	if err := pins.ValidName(body.Name); err != nil {
		writePinsErr(w, "add", err)
		return
	}
	recursive := true
	if body.Recursive != nil {
		recursive = *body.Recursive
	}
	job, err := h.start(g.client, target, body.Name, recursive)
	if err != nil {
		writePinsErr(w, "add", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": toJobJSON(job)})
}

func (h *pinsHTTP) remove(w http.ResponseWriter, r *http.Request) {
	h.ensure()
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	var body struct {
		CID  string `json:"cid"`
		Type string `json:"type"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	if body.Type != "recursive" && body.Type != "direct" {
		writeProblem(w, &statusError{code: http.StatusBadRequest, msg: "bad_request"})
		return
	}
	cid, err := pins.ParseTarget(body.CID)
	if err != nil || strings.Contains(cid, "/") {
		writePinsErr(w, "rm", &pins.OpError{Code: "invalid_cid"})
		return
	}
	if err := g.client.Remove(r.Context(), cid, body.Type); err != nil {
		writePinsErr(w, "rm", err)
		return
	}
	if err := h.labels.Delete(cid); err != nil {
		slog.Warn("pin label", "op", "delete")
	}
	writeJSON(w, http.StatusOK, map[string]string{"cid": cid, "type": body.Type})
}

func (h *pinsHTTP) fromFiles(w http.ResponseWriter, r *http.Request) {
	h.ensure()
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
	fc, err := files.New(files.Options{
		BaseURL:         g.base,
		MaxBytes:        h.maxBytes,
		OpTimeout:       h.op,
		TransferTimeout: h.transfer,
	})
	if err != nil {
		writePinsErr(w, "from-files", err)
		return
	}
	st, err := fc.Stat(r.Context(), body.Path)
	if err != nil {
		writePinsErr(w, "from-files", err)
		return
	}
	target, err := pins.ParseTarget(st.CID)
	if err != nil || strings.Contains(target, "/") {
		writePinsErr(w, "from-files", &pins.OpError{Code: "invalid_cid"})
		return
	}
	name := body.Name
	if name == "" {
		if base, baseErr := files.Base(body.Path); baseErr == nil && pins.ValidName(base) == nil {
			name = base
		}
	}
	if err := pins.ValidName(name); err != nil {
		writePinsErr(w, "from-files", err)
		return
	}
	job, err := h.start(g.client, target, name, true)
	if err != nil {
		writePinsErr(w, "from-files", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": toJobJSON(job)})
}

func (h *pinsHTTP) upload(w http.ResponseWriter, r *http.Request) {
	h.ensure()
	g, ok := h.open(w, r)
	if !ok {
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/octet-stream" {
		writeProblem(w, &statusError{code: http.StatusUnsupportedMediaType, msg: "unsupported_media_type"})
		return
	}
	filename := r.URL.Query().Get("filename")
	if !files.Segment(filename) {
		writePinsErr(w, "upload", &files.OpError{Code: "invalid_path"})
		return
	}
	label := r.URL.Query().Get("name")
	if label == "" {
		label = filename
	}
	if err := pins.ValidName(label); err != nil {
		writePinsErr(w, "upload", err)
		return
	}
	if r.ContentLength > h.maxBytes {
		writePinsErr(w, "upload", &pins.OpError{Code: "too_large"})
		return
	}
	pin, err := g.client.Upload(r.Context(), filename, r.Body)
	if err != nil {
		writePinsErr(w, "upload", err)
		return
	}
	pin.Name = label
	if label != "" {
		if err := h.labels.Set(pin.CID, label); err != nil {
			slog.Warn("pin label", "op", "upload")
		}
		nameCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		if err := g.client.RememberName(nameCtx, pin.CID, label, h.support); err != nil {
			slog.Warn("pin name", "op", "remember")
		}
		cancel()
	}
	writeJSON(w, http.StatusCreated, toPinJSON(pin))
}

func (h *pinsHTTP) jobsList(w http.ResponseWriter, _ *http.Request) {
	h.ensure()
	jobs := h.jobs.List()
	out := make([]jobJSON, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, toJobJSON(job))
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
}

func (h *pinsHTTP) jobGet(w http.ResponseWriter, r *http.Request) {
	h.ensure()
	job, err := h.jobs.Get(r.PathValue("id"))
	if err != nil || !jobID(r.PathValue("id")) {
		writePinsErr(w, "job", &pins.OpError{Code: "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, toJobJSON(job))
}

func (h *pinsHTTP) jobCancel(w http.ResponseWriter, r *http.Request) {
	h.ensure()
	if !jobID(r.PathValue("id")) {
		writePinsErr(w, "cancel", &pins.OpError{Code: "not_found"})
		return
	}
	job, err := h.jobs.Cancel(r.PathValue("id"))
	if err != nil {
		writePinsErr(w, "cancel", err)
		return
	}
	writeJSON(w, http.StatusOK, toJobJSON(job))
}

func (h *pinsHTTP) start(client *pins.Client, target, name string, recursive bool) (pins.Job, error) {
	return h.jobs.Start(target, name, recursive, h.pinTimeout, func(ctx context.Context, report func(int)) (string, error) {
		cid, err := client.Add(ctx, target, name, recursive, report, h.support)
		if err != nil {
			return "", err
		}
		if name != "" && cid != "" {
			if err := h.labels.Set(cid, name); err != nil {
				slog.Warn("pin label", "op", "set")
			}
		}
		return cid, nil
	})
}

type pinsGate struct {
	client  *pins.Client
	base    string
	gateway string
}

func (h *pinsHTTP) open(w http.ResponseWriter, r *http.Request) (pinsGate, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return pinsGate{}, false
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
		return pinsGate{}, false
	}
	apiBase, gateway, err := h.endpoints(ov.RepoPath)
	if err != nil {
		slog.Warn("pins endpoints", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return pinsGate{}, false
	}
	client, err := pins.New(pins.Options{
		BaseURL:         apiBase,
		MaxBytes:        h.maxBytes,
		OpTimeout:       h.op,
		TransferTimeout: h.transfer,
	})
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return pinsGate{}, false
	}
	return pinsGate{client: client, base: apiBase, gateway: gateway}, true
}

func (h *pinsHTTP) overview(ctx context.Context) (node.Overview, error) {
	if h.status != nil {
		return h.status(ctx)
	}
	return h.sup.Cached(ctx)
}

func (h *pinsHTTP) endpoints(repo string) (string, string, error) {
	if h.lookup != nil {
		return h.lookup(repo)
	}
	return node.LoopbackEndpoints(repo)
}

func toPinJSON(pin pins.Pin) pinJSON {
	return pinJSON{CID: pin.CID, Name: pin.Name, Type: pin.Type, Size: pin.Size}
}

func toJobJSON(job pins.Job) jobJSON {
	return jobJSON{
		ID:       job.ID,
		CID:      job.CID,
		Name:     job.Name,
		Type:     job.Type,
		State:    job.State,
		Progress: job.Progress,
		Error:    job.Error,
		Created:  job.Created.UTC().Format(time.RFC3339),
	}
}

func jobID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

func writePinsErr(w http.ResponseWriter, op string, err error) {
	code := pins.CodeOf(err)
	if code == "" {
		code = files.CodeOf(err)
	}
	if code == "" {
		code = "unavailable"
	}
	if code == "names_unsupported" || code == "name_unsupported" {
		code = "kubo_error"
	}
	var status int
	switch code {
	case "invalid_cid", "invalid_name", "invalid_path", "bad_request":
		status = http.StatusBadRequest
	case "not_found":
		status = http.StatusNotFound
	case "indirect", "too_many", "already_exists":
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
		slog.Warn("pins", "op", op, "code", code)
	}
	body := map[string]string{"error": code}
	if code == "node_not_running" {
		body["state"] = node.StateDown
	}
	writeJSON(w, status, body)
}
