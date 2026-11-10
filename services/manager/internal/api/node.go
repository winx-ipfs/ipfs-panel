package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/activity"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string {
	if e == nil {
		return ""
	}
	return e.msg
}

type nodeAPI struct {
	sup *node.Supervisor
}

func (n *nodeAPI) get(w http.ResponseWriter, r *http.Request) {
	ov, err := n.sup.Snapshot(r.Context())
	if err != nil {
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
		return
	}
	if ov.State == node.StateUp {
		ov.Activity = n.readActivity(r.Context(), ov.RepoPath)
	}
	writeOverview(w, ov)
}

func (n *nodeAPI) readActivity(ctx context.Context, repo string) *node.Activity {
	apiBase, _, err := node.LoopbackEndpoints(repo)
	if err != nil || apiBase == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	act, err := activity.Fetch(ctx, apiBase)
	if err != nil {
		return nil
	}
	return act
}

func (n *nodeAPI) post(fn func(r *http.Request) (node.Overview, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := rejectBody(w, r); err != nil {
			writeProblem(w, err)
			return
		}
		ov, err := fn(r)
		if err != nil {
			writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
			return
		}
		writeOverview(w, ov)
	}
}

func rejectBody(w http.ResponseWriter, r *http.Request) *statusError {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &statusError{code: http.StatusUnsupportedMediaType, msg: "unsupported_media_type"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	var value any
	if err := dec.Decode(&value); err != nil {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	obj, ok := value.(map[string]any)
	if !ok || len(obj) != 0 {
		return &statusError{code: http.StatusBadRequest, msg: "bad_request"}
	}
	return nil
}

func writeOverview(w http.ResponseWriter, ov node.Overview) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ov)
}

func writeProblem(w http.ResponseWriter, err *statusError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(err.code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.msg})
}
