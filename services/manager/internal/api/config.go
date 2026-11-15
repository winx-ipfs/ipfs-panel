package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

const maxConfigBody = 6 << 20

type configHTTP struct {
	sup        *node.Supervisor
	status     func(context.Context) (node.Overview, error)
	lookup     func(repo string) (apiBase, gatewayBase string, err error)
	edit       *configedit.Service
	trustProxy bool
	log        *slog.Logger
}

func (h *configHTTP) get(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.open(w, r, "")
	if !ok {
		return
	}
	view, err := svc.Show(r.Context())
	if err != nil {
		h.fail(w, r, "", err)
		return
	}
	writeJSON(w, http.StatusOK, configViewJSON(view))
}

func (h *configHTTP) replace(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.open(w, r, "config_replace")
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeConfigJSON(w, r, &body); err != nil {
		h.audit(r, "config_replace", "invalid")
		writeProblem(w, err)
		return
	}
	res, err := svc.Replace(r.Context(), body.Text)
	if err != nil {
		h.fail(w, r, "config_replace", err)
		return
	}
	result := "success"
	if res.Restart == configedit.RestartNone {
		result = "unchanged"
	}
	h.audit(r, "config_replace", result)
	writeJSON(w, http.StatusOK, configResultJSON(res))
}

func (h *configHTTP) restore(w http.ResponseWriter, r *http.Request) {
	svc, ok := h.open(w, r, "config_restore")
	if !ok {
		return
	}
	if err := rejectBody(w, r); err != nil {
		h.audit(r, "config_restore", "invalid")
		writeProblem(w, err)
		return
	}
	res, err := svc.Restore(r.Context())
	if err != nil {
		h.fail(w, r, "config_restore", err)
		return
	}
	result := "success"
	if res.Restart == configedit.RestartNone {
		result = "unchanged"
	}
	h.audit(r, "config_restore", result)
	writeJSON(w, http.StatusOK, configResultJSON(res))
}

func (h *configHTTP) open(w http.ResponseWriter, r *http.Request, event string) (*configedit.Service, bool) {
	ov, err := h.overview(r.Context())
	if err != nil {
		if event != "" {
			h.audit(r, event, "unavailable")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	if ov.State != node.StateUp {
		state := ov.State
		if state == "" {
			state = node.StateDown
		}
		if event != "" {
			h.audit(r, event, "unavailable")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "node_not_running",
			"state": state,
		})
		return nil, false
	}
	if h.edit != nil {
		return h.edit, true
	}
	repo := ov.RepoPath
	if repo == "" && h.sup != nil {
		repo = h.sup.RepoPath()
	}
	apiBase, _, err := h.endpoints(repo)
	if err != nil {
		slog.Warn("config endpoints", "err", err)
		if event != "" {
			h.audit(r, event, "unavailable")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	client, err := configedit.NewClient(apiBase, 0)
	if err != nil {
		if event != "" {
			h.audit(r, event, "unavailable")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return nil, false
	}
	data, addr := "", "127.0.0.1:8080"
	if h.sup != nil {
		data = h.sup.DataDir()
		if h.sup.ManagerAddr() != "" {
			addr = h.sup.ManagerAddr()
		}
	}
	return configedit.New(client, repo, data, addr), true
}

func (h *configHTTP) overview(ctx context.Context) (node.Overview, error) {
	if h.status != nil {
		return h.status(ctx)
	}
	return h.sup.Cached(ctx)
}

func (h *configHTTP) endpoints(repo string) (string, string, error) {
	if h.lookup != nil {
		return h.lookup(repo)
	}
	return node.LoopbackEndpoints(repo)
}

func (h *configHTTP) fail(w http.ResponseWriter, r *http.Request, event string, err error) {
	var edit *configedit.Error
	if !errors.As(err, &edit) || edit == nil {
		if event != "" {
			h.audit(r, event, "unavailable")
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	if event != "" {
		h.audit(r, event, auditResult(edit))
	}
	if edit.Code == "kubo_rejected" || edit.Code == "unavailable" || edit.Code == "timeout" || edit.Code == "node_not_running" {
		h.logger().Warn("config", "code", edit.Code)
	}
	body := map[string]string{"error": edit.Code}
	if edit.Detail != "" {
		body["detail"] = edit.Detail
	}
	if edit.Code == "node_not_running" {
		body["state"] = node.StateDown
	}
	writeJSON(w, edit.Status, body)
}

func auditResult(err *configedit.Error) string {
	switch err.Status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType:
		return "invalid"
	case http.StatusConflict:
		return "forbidden"
	case http.StatusBadGateway:
		return "rejected"
	default:
		return "unavailable"
	}
}

func (h *configHTTP) audit(r *http.Request, event, result string) {
	username := ""
	if principal, ok := principalFrom(r); ok {
		username = principal.username
	}
	h.logger().Info("audit",
		"event", event,
		"result", result,
		"username", username,
		"ip", clientIP(r, h.trustProxy),
	)
}

func (h *configHTTP) logger() *slog.Logger {
	if h.log != nil {
		return h.log
	}
	return slog.Default()
}

type configDocumentJSON struct {
	Text      string   `json:"text"`
	Scene     *string  `json:"scene"`
	SceneKeys []string `json:"sceneKeys"`
	Backup    bool     `json:"backup"`
}

type configWriteJSON struct {
	configDocumentJSON
	Restart  string `json:"restart"`
	Diverged bool   `json:"diverged"`
}

func configViewJSON(view configedit.View) configDocumentJSON {
	return configDocumentJSON{
		Text:      view.Text,
		Scene:     scenePointer(view.Scene),
		SceneKeys: view.SceneKeys,
		Backup:    view.Backup,
	}
}

func configResultJSON(res configedit.Result) configWriteJSON {
	return configWriteJSON{
		configDocumentJSON: configViewJSON(res.View),
		Restart:            res.Restart,
		Diverged:           res.Diverged,
	}
}

func scenePointer(scene string) *string {
	if scene == "" {
		return nil
	}
	return &scene
}

func decodeConfigJSON(w http.ResponseWriter, r *http.Request, dest any) *statusError {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &statusError{code: http.StatusUnsupportedMediaType, msg: "unsupported_media_type"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxConfigBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			return &statusError{code: http.StatusRequestEntityTooLarge, msg: "too_large"}
		}
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	return nil
}
