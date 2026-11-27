package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuboinstall"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type kuboHTTP struct {
	sup   *node.Supervisor
	svc   *kuboinstall.Service
	trust bool
	log   *slog.Logger
	once  sync.Once
}

type supControl struct {
	sup *node.Supervisor
}

func (c supControl) Snapshot(ctx context.Context) (string, string, error) {
	ov, err := c.sup.Snapshot(ctx)
	if err != nil {
		return "", "", err
	}
	code := ""
	if ov.Error != nil {
		code = ov.Error.Code
	}
	return ov.State, code, nil
}

func (c supControl) Stop(ctx context.Context) error {
	_, err := c.sup.Stop(ctx)
	return err
}

func (c supControl) Start(ctx context.Context) error {
	_, err := c.sup.StartForInstall(ctx)
	return err
}

func (c supControl) Hold(on bool) { c.sup.SetStartHold(on) }

func (h *kuboHTTP) service() *kuboinstall.Service {
	h.once.Do(func() {
		if h.svc != nil {
			return
		}
		data, repo, explicit := "", "", ""
		var ctrl kuboinstall.Controller
		if h.sup != nil {
			data = h.sup.DataDir()
			repo = h.sup.RepoPath()
			explicit = h.sup.ExplicitBin()
			ctrl = supControl{h.sup}
		}
		h.svc = kuboinstall.New(kuboinstall.Options{
			DataDir:     data,
			RepoDir:     repo,
			ExplicitBin: explicit,
			Node:        ctrl,
			Logger:      h.logger(),
		})
	})
	return h.svc
}

func (h *kuboHTTP) routes() []route {
	return []route{
		{http.MethodGet, "/api/kubo", h.get},
		{http.MethodPost, "/api/kubo/install", h.install},
		{http.MethodPost, "/api/kubo/upgrade", h.upgrade},
		{http.MethodGet, "/api/kubo/jobs/{id}", h.job},
	}
}

type kuboJobJSON struct {
	ID       string `json:"id"`
	Action   string `json:"action"`
	State    string `json:"state"`
	Phase    string `json:"phase"`
	Version  string `json:"version"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Error    string `json:"error"`
}

type kuboStatusJSON struct {
	Source    string       `json:"source"`
	Version   string       `json:"version"`
	Pinned    string       `json:"pinned"`
	Supported string       `json:"supported"`
	Latest    string       `json:"latest"`
	Upstream  string       `json:"upstream"`
	Upgrade   bool         `json:"upgrade"`
	Check     string       `json:"check"`
	Job       *kuboJobJSON `json:"job"`
}

func kuboJobJSONFrom(job kuboinstall.Job) *kuboJobJSON {
	if job.ID == "" {
		return nil
	}
	return &kuboJobJSON{
		ID:       job.ID,
		Action:   job.Action,
		State:    job.State,
		Phase:    job.Phase,
		Version:  job.Version,
		Received: job.Received,
		Total:    job.Total,
		Error:    job.Error,
	}
}

func statusJSON(st kuboinstall.Status) kuboStatusJSON {
	body := kuboStatusJSON{
		Source:    st.Source,
		Version:   st.Version,
		Pinned:    st.Pinned,
		Supported: st.Supported,
		Latest:    st.Latest,
		Upstream:  st.Upstream,
		Upgrade:   st.Upgrade,
		Check:     st.Check,
	}
	if st.Job != nil {
		body.Job = kuboJobJSONFrom(*st.Job)
	}
	return body
}

func (h *kuboHTTP) get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusJSON(h.service().Status(r.Context())))
}

func (h *kuboHTTP) install(w http.ResponseWriter, r *http.Request) {
	h.post(w, r, "kubo_install", func(actor kuboinstall.Actor) (kuboinstall.Job, error) {
		return h.service().Install(actor)
	})
}

func (h *kuboHTTP) upgrade(w http.ResponseWriter, r *http.Request) {
	h.post(w, r, "kubo_upgrade", func(actor kuboinstall.Actor) (kuboinstall.Job, error) {
		return h.service().Upgrade(actor)
	})
}

func (h *kuboHTTP) post(w http.ResponseWriter, r *http.Request, event string, fn func(kuboinstall.Actor) (kuboinstall.Job, error)) {
	if err := rejectBody(w, r); err != nil {
		h.audit(r, event, "invalid", "", "")
		writeProblem(w, err)
		return
	}
	job, err := fn(h.actor(r))
	if err != nil {
		// The service already wrote the audit line for a refusal it owns.
		// A body the service does not see is audited above.
		writeKuboErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, kuboJobJSONFrom(job))
}

func (h *kuboHTTP) job(w http.ResponseWriter, r *http.Request) {
	job, err := h.service().Job(r.PathValue("id"))
	if err != nil {
		writeKuboErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, kuboJobJSONFrom(job))
}

func writeKuboErr(w http.ResponseWriter, err error) {
	var ke *kuboinstall.Error
	if !errors.As(err, &ke) || ke.Code == "" {
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
		return
	}
	status := http.StatusConflict
	switch ke.Code {
	case kuboinstall.CodeOffline:
		status = http.StatusServiceUnavailable
	case kuboinstall.CodeNotFound:
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": ke.Code})
}

func (h *kuboHTTP) actor(r *http.Request) kuboinstall.Actor {
	name := ""
	if principal, ok := principalFrom(r); ok {
		name = principal.username
	}
	return kuboinstall.Actor{Username: name, IP: clientIP(r, h.trust)}
}

func (h *kuboHTTP) audit(r *http.Request, event, result, version, previous string) {
	username := ""
	if principal, ok := principalFrom(r); ok {
		username = principal.username
	}
	args := []any{
		"event", event,
		"result", result,
		"username", username,
		"ip", clientIP(r, h.trust),
		"version", version,
	}
	if previous != "" {
		args = append(args, "previous", previous)
	}
	h.logger().Info("audit", args...)
}

func (h *kuboHTTP) logger() *slog.Logger {
	if h.log != nil {
		return h.log
	}
	return slog.Default()
}
