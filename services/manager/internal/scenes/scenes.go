// Package scenes writes the four named Kubo setups from the scenes spec.
// It edits the config file. It does not speak Kubo RPC and it never writes
// Addresses.API or Identity.
package scenes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

const (
	Quiet   = "quiet"
	Share   = "share"
	Pinset  = "pinset"
	Gateway = "gateway"

	RestartNone    = "none"
	RestartLater   = "later"
	RestartStarted = "started"
)

// IDs is the scene order the page shows.
func IDs() []string { return []string{Quiet, Share, Pinset, Gateway} }

// Change is one effective difference, described by a short code.
// The code is not a config value the page should print raw. Gateway
// addresses in particular are not included.
type Change struct {
	Key  string `json:"key"`
	Code string `json:"code"`
}

// View is a preview or a list entry.
type View struct {
	ID          string   `json:"id"`
	Active      bool     `json:"active"`
	Restart     bool     `json:"restart"`
	Changes     []Change `json:"changes"`
	GatewayPort int      `json:"gatewayPort"`
	NodeState   string   `json:"nodeState"`
	Ready       bool     `json:"ready"`
	Revert      bool     `json:"revert"`
}

// Result is a successful apply or revert.
type Result struct {
	ID      string        `json:"id,omitempty"`
	Restart string        `json:"restart"`
	Node    node.Overview `json:"node"`
}

// Runtime is the node lifecycle the apply path is allowed to use.
type Runtime interface {
	Cached(context.Context) (node.Overview, error)
	Restart(context.Context) (node.Overview, error)
}

// Error is a stable scenes failure.
type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

func sceneError(status int, code string) *Error {
	return &Error{Code: code, Status: status}
}

// Catalog lists the four scenes and which one matches the config.
func Catalog(repo, dataDir, managerAddr, state string) (View, []View, error) {
	port := node.GatewayListenPort(managerAddr)
	revert, err := hasBackup(dataDir)
	if err != nil {
		return View{}, nil, err
	}
	doc, ready, err := loadOptional(repo)
	if err != nil {
		return View{}, nil, err
	}
	out := make([]View, 0, len(IDs()))
	for _, id := range IDs() {
		changes, err := diff(doc, ready, id, managerAddr)
		if err != nil {
			return View{}, nil, err
		}
		out = append(out, View{
			ID:          id,
			Active:      ready && len(changes) == 0,
			Restart:     len(changes) > 0,
			Changes:     changes,
			GatewayPort: port,
			NodeState:   state,
			Ready:       ready,
			Revert:      revert,
		})
	}
	return View{GatewayPort: port, NodeState: state, Ready: ready, Revert: revert}, out, nil
}

// Preview is the change list for one scene. It does not write.
func Preview(repo, dataDir, managerAddr, state, id string) (View, error) {
	if !known(id) {
		return View{}, sceneError(400, "unknown_scene")
	}
	_, scenes, err := Catalog(repo, dataDir, managerAddr, state)
	if err != nil {
		return View{}, err
	}
	for _, scene := range scenes {
		if scene.ID == id {
			if !scene.Ready {
				return View{}, sceneError(409, "not_ready")
			}
			return scene, nil
		}
	}
	return View{}, sceneError(400, "unknown_scene")
}

// Apply writes the scene and, when the node is up, asks the runtime to
// restart. A stopped node is left stopped.
func Apply(ctx context.Context, repo, dataDir, managerAddr, id string, rt Runtime) (Result, error) {
	if !known(id) {
		return Result{}, sceneError(400, "unknown_scene")
	}
	ov, err := rt.Cached(ctx)
	if err != nil {
		return Result{}, err
	}
	if busy(ov) {
		return Result{}, sceneError(409, "node_busy")
	}
	doc, err := load(repo)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, sceneError(409, "not_ready")
		}
		return Result{}, err
	}
	if err := refusePublicAPI(doc, managerAddr); err != nil {
		return Result{}, err
	}
	changes, err := diff(doc, true, id, managerAddr)
	if err != nil {
		return Result{}, err
	}
	if len(changes) == 0 {
		return Result{ID: id, Restart: RestartNone, Node: ov}, nil
	}
	previous, err := snapshot(doc)
	if err != nil {
		return Result{}, err
	}
	if err := assignScene(doc, id, managerAddr); err != nil {
		return Result{}, err
	}
	body, err := marshalDoc(doc)
	if err != nil {
		return Result{}, err
	}
	if err := node.AssessConfig(body, managerAddr); err != nil {
		var cfg *node.ConfigError
		if errors.As(err, &cfg) {
			return Result{}, sceneError(409, cfg.Code)
		}
		return Result{}, err
	}
	backupPath, err := writeBackup(dataDir, backup{
		Scene:  id,
		At:     time.Now().UTC(),
		Values: previous,
	})
	if err != nil {
		return Result{}, err
	}
	if err := save(repo, body); err != nil {
		_ = os.Remove(backupPath)
		return Result{}, err
	}
	return finish(ctx, rt, id, ov, RestartLater)
}

// Revert restores the newest backup and restarts when the node is up.
func Revert(ctx context.Context, repo, dataDir, managerAddr string, rt Runtime) (Result, error) {
	ov, err := rt.Cached(ctx)
	if err != nil {
		return Result{}, err
	}
	if busy(ov) {
		return Result{}, sceneError(409, "node_busy")
	}
	item, path, err := latestBackup(dataDir)
	if err != nil {
		return Result{}, err
	}
	if path == "" {
		return Result{}, sceneError(409, "nothing_to_revert")
	}
	doc, err := load(repo)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, sceneError(409, "not_ready")
		}
		return Result{}, err
	}
	if err := restore(doc, item.Values); err != nil {
		return Result{}, err
	}
	body, err := marshalDoc(doc)
	if err != nil {
		return Result{}, err
	}
	if err := node.AssessConfig(body, managerAddr); err != nil {
		var cfg *node.ConfigError
		if errors.As(err, &cfg) {
			return Result{}, sceneError(409, cfg.Code)
		}
		return Result{}, err
	}
	if err := save(repo, body); err != nil {
		return Result{}, err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	return finish(ctx, rt, item.Scene, ov, RestartLater)
}

func finish(ctx context.Context, rt Runtime, id string, before node.Overview, whenDown string) (Result, error) {
	if before.State != node.StateUp {
		return Result{ID: id, Restart: whenDown, Node: before}, nil
	}
	again, err := rt.Cached(ctx)
	if err != nil {
		return Result{}, err
	}
	if again.State != node.StateUp {
		return Result{ID: id, Restart: RestartLater, Node: again}, nil
	}
	ov, err := rt.Restart(ctx)
	if err != nil {
		return Result{}, sceneError(502, "restart_failed")
	}
	return Result{ID: id, Restart: RestartStarted, Node: ov}, nil
}

func busy(ov node.Overview) bool {
	switch ov.State {
	case node.StateStarting, node.StateStopping:
		return true
	case node.StateFailed:
		return ov.Error != nil && ov.Error.Code == node.CodeRepoBusy
	default:
		return false
	}
}

func known(id string) bool {
	switch id {
	case Quiet, Share, Pinset, Gateway:
		return true
	default:
		return false
	}
}

func refusePublicAPI(doc map[string]any, managerAddr string) error {
	body, err := marshalDoc(doc)
	if err != nil {
		return err
	}
	if err := node.AssessConfig(body, managerAddr); err != nil {
		var cfg *node.ConfigError
		if errors.As(err, &cfg) && cfg.Code == node.CodeAPINotLoopback {
			return sceneError(409, cfg.Code)
		}
	}
	return nil
}

func diff(doc map[string]any, ready bool, id, managerAddr string) ([]Change, error) {
	if !ready {
		return []Change{}, nil
	}
	want, err := desired(id, managerAddr)
	if err != nil {
		return nil, err
	}
	var out []Change
	connChanged := false
	for _, key := range managedKeys {
		if strings.HasPrefix(key, "Swarm.ConnMgr.") {
			same, err := sameKey(doc, key, want[key])
			if err != nil {
				return nil, err
			}
			if !same {
				connChanged = true
			}
			continue
		}
		same, err := sameKey(doc, key, want[key])
		if err != nil {
			return nil, err
		}
		if same {
			continue
		}
		out = append(out, Change{Key: key, Code: codeFor(key, want[key])})
	}
	if connChanged {
		code := "usual"
		if want["Swarm.ConnMgr.LowWater"] == "20" {
			code = "quiet"
		}
		out = append(out, Change{Key: "Swarm.ConnMgr", Code: code})
	}
	if out == nil {
		out = []Change{}
	}
	return out, nil
}

func assignScene(doc map[string]any, id, managerAddr string) error {
	want, err := desired(id, managerAddr)
	if err != nil {
		return err
	}
	for _, key := range managedKeys {
		if err := setRaw(doc, key, want[key]); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}

func codeFor(key, raw string) string {
	switch key {
	case "Routing.Type", "AutoNAT.ServiceMode", "Provide.Strategy":
		return strings.Trim(raw, `"`)
	case "Swarm.RelayService.Enabled", "Provide.Enabled", "Gateway.ExposeRoutingAPI":
		if raw == "true" {
			return "on"
		}
		return "off"
	case "Gateway.NoFetch":
		if raw == "true" {
			return "local"
		}
		return "fetch"
	case "Gateway.NoDNSLink":
		if raw == "true" {
			return "off"
		}
		return "on"
	case "Addresses.Gateway":
		if strings.Contains(raw, "0.0.0.0") {
			return "public"
		}
		return "local"
	default:
		return "changed"
	}
}

var managedKeys = []string{
	"Routing.Type",
	"AutoNAT.ServiceMode",
	"Swarm.RelayService.Enabled",
	"Swarm.ConnMgr.Type",
	"Swarm.ConnMgr.LowWater",
	"Swarm.ConnMgr.HighWater",
	"Swarm.ConnMgr.GracePeriod",
	"Provide.Enabled",
	"Provide.Strategy",
	"Gateway.NoFetch",
	"Gateway.NoDNSLink",
	"Gateway.ExposeRoutingAPI",
	"Addresses.Gateway",
}

func desired(id, managerAddr string) (map[string]string, error) {
	port := node.GatewayListenPort(managerAddr)
	local := fmt.Sprintf(`["/ip4/127.0.0.1/tcp/%d"]`, port)
	public := fmt.Sprintf(`["/ip4/0.0.0.0/tcp/%d","/ip6/::/tcp/%d"]`, port, port)
	out := map[string]string{
		"Routing.Type":               `"auto"`,
		"AutoNAT.ServiceMode":        `"enabled"`,
		"Swarm.RelayService.Enabled": `true`,
		"Swarm.ConnMgr.Type":         `"basic"`,
		"Swarm.ConnMgr.LowWater":     `32`,
		"Swarm.ConnMgr.HighWater":    `96`,
		"Swarm.ConnMgr.GracePeriod":  `"20s"`,
		"Provide.Enabled":            `true`,
		"Provide.Strategy":           `"all"`,
		"Gateway.NoFetch":            `false`,
		"Gateway.NoDNSLink":          `false`,
		"Gateway.ExposeRoutingAPI":   `true`,
		"Addresses.Gateway":          local,
	}
	switch id {
	case Quiet:
		out["Routing.Type"] = `"autoclient"`
		out["AutoNAT.ServiceMode"] = `"disabled"`
		out["Swarm.RelayService.Enabled"] = `false`
		out["Swarm.ConnMgr.LowWater"] = `20`
		out["Swarm.ConnMgr.HighWater"] = `40`
		out["Swarm.ConnMgr.GracePeriod"] = `"1m0s"`
	case Share:
		out["Provide.Strategy"] = `"mfs"`
	case Pinset:
		out["Provide.Strategy"] = `"pinned"`
		out["Gateway.NoFetch"] = `true`
	case Gateway:
		out["Provide.Strategy"] = `"pinned+mfs"`
		out["Gateway.NoFetch"] = `true`
		out["Gateway.NoDNSLink"] = `true`
		out["Gateway.ExposeRoutingAPI"] = `false`
		out["Addresses.Gateway"] = public
	default:
		return nil, sceneError(400, "unknown_scene")
	}
	return out, nil
}
