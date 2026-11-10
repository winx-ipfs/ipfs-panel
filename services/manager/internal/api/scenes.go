package api

import (
	"errors"
	"net/http"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/scenes"
)

func (n *nodeAPI) scenesList(w http.ResponseWriter, r *http.Request) {
	ov, err := n.sup.Cached(r.Context())
	if err != nil {
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
		return
	}
	summary, list, err := scenes.Catalog(n.sup.RepoPath(), n.sup.DataDir(), n.sup.ManagerAddr(), ov.State)
	if err != nil {
		writeSceneErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(list))
	for _, scene := range list {
		items = append(items, map[string]any{
			"id":      scene.ID,
			"active":  scene.Active,
			"restart": scene.Restart,
			"changes": scene.Changes,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":       summary.Ready,
		"gatewayPort": summary.GatewayPort,
		"nodeState":   summary.NodeState,
		"revert":      summary.Revert,
		"scenes":      items,
	})
}

func (n *nodeAPI) scenesPreview(w http.ResponseWriter, r *http.Request) {
	ov, err := n.sup.Cached(r.Context())
	if err != nil {
		writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
		return
	}
	view, err := scenes.Preview(n.sup.RepoPath(), n.sup.DataDir(), n.sup.ManagerAddr(), ov.State, r.URL.Query().Get("id"))
	if err != nil {
		writeSceneErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          view.ID,
		"active":      view.Active,
		"restart":     view.Restart,
		"changes":     view.Changes,
		"gatewayPort": view.GatewayPort,
		"nodeState":   view.NodeState,
		"ready":       view.Ready,
		"revert":      view.Revert,
	})
}

func (n *nodeAPI) scenesApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := readAuthJSON(w, r, &body); err != nil {
		writeProblem(w, err)
		return
	}
	res, err := scenes.Apply(r.Context(), n.sup.RepoPath(), n.sup.DataDir(), n.sup.ManagerAddr(), body.ID, n.sup)
	if err != nil {
		writeSceneErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (n *nodeAPI) scenesRevert(w http.ResponseWriter, r *http.Request) {
	if err := rejectBody(w, r); err != nil {
		writeProblem(w, err)
		return
	}
	res, err := scenes.Revert(r.Context(), n.sup.RepoPath(), n.sup.DataDir(), n.sup.ManagerAddr(), n.sup)
	if err != nil {
		writeSceneErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func writeSceneErr(w http.ResponseWriter, err error) {
	var scene *scenes.Error
	if errors.As(err, &scene) {
		writeProblem(w, &statusError{code: scene.Status, msg: scene.Code})
		return
	}
	writeProblem(w, &statusError{code: http.StatusInternalServerError, msg: "unavailable"})
}
