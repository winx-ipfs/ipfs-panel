package configedit

import (
	"context"
	"net/http"
	"os"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/scenes"
)

const (
	// RestartNone means the document already matched the stored config.
	RestartNone = "none"
	// RestartNeeded means Kubo stored a new document and the daemon should restart.
	RestartNeeded = "needed"
)

// Kubo is the config/show and config/replace calls.
type Kubo interface {
	Show(ctx context.Context) ([]byte, error)
	Replace(ctx context.Context, document []byte) error
}

// View is the redacted document and the scene hint around it.
type View struct {
	Text      string
	Scene     string
	SceneKeys []string
	Backup    bool
}

// Result is a replace or a restore.
type Result struct {
	View
	Restart  string
	Diverged bool
}

// Service applies the editor rules in front of one Kubo RPC.
type Service struct {
	kubo        Kubo
	repo        string
	dataDir     string
	managerAddr string
}

// New builds a service. repo is the Kubo repository, dataDir is the manager
// data directory, and managerAddr is the panel's own listen address.
func New(kubo Kubo, repo, dataDir, managerAddr string) *Service {
	return &Service{kubo: kubo, repo: repo, dataDir: dataDir, managerAddr: managerAddr}
}

// Show returns the redacted config. The private key is not in Text.
func (s *Service) Show(ctx context.Context) (View, error) {
	raw, err := s.kubo.Show(ctx)
	if err != nil {
		return View{}, err
	}
	text, _, err := redactShow(raw)
	if err != nil {
		return View{}, err
	}
	return s.view(text)
}

// Replace validates text, backs up the current document, and calls
// config/replace. An unchanged document does not call Kubo.
func (s *Service) Replace(ctx context.Context, text string) (Result, error) {
	return s.apply(ctx, text, true)
}

// Restore sends the newest backup through the same replace path and deletes
// that backup. It does not write a new one.
func (s *Service) Restore(ctx context.Context) (Result, error) {
	text, path, err := latestBackup(s.dataDir)
	if err != nil {
		return Result{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	if path == "" {
		return Result{}, code(http.StatusConflict, "nothing_to_restore")
	}
	res, err := s.apply(ctx, text, false)
	if err != nil {
		return Result{}, err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return Result{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	res.Backup, err = hasBackup(s.dataDir)
	if err != nil {
		return Result{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	return res, nil
}

func (s *Service) apply(ctx context.Context, text string, keepBackup bool) (Result, error) {
	doc, err := parseText(text)
	if err != nil {
		return Result{}, err
	}
	if err := rejectSecrets(doc); err != nil {
		return Result{}, err
	}
	stripSecrets(doc)
	raw, err := s.kubo.Show(ctx)
	if err != nil {
		return Result{}, err
	}
	currentText, current, err := redactShow(raw)
	if err != nil {
		return Result{}, err
	}
	if err := sameAPI(current, doc); err != nil {
		return Result{}, err
	}
	if err := samePeer(current, doc); err != nil {
		return Result{}, err
	}
	body, err := prettyBytes(doc)
	if err != nil {
		return Result{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	if err := assess(body, s.managerAddr); err != nil {
		return Result{}, err
	}
	scene, err := s.activeScene()
	if err != nil {
		return Result{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	nextText, err := pretty(doc)
	if err != nil {
		return Result{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	if nextText == currentText {
		view, err := s.view(currentText)
		if err != nil {
			return Result{}, err
		}
		view.Scene = scene
		return Result{View: view, Restart: RestartNone}, nil
	}
	var backupPath string
	if keepBackup {
		backupPath, err = writeBackup(s.dataDir, currentText)
		if err != nil {
			return Result{}, code(http.StatusServiceUnavailable, "unavailable")
		}
	}
	if err := s.kubo.Replace(ctx, body); err != nil {
		if backupPath != "" {
			_ = os.Remove(backupPath)
		}
		return Result{}, err
	}
	shown, err := s.Show(ctx)
	if err != nil {
		return Result{}, err
	}
	return Result{
		View:     shown,
		Restart:  RestartNeeded,
		Diverged: sceneDiverged(current, doc, scene),
	}, nil
}

func (s *Service) view(text string) (View, error) {
	scene, err := s.activeScene()
	if err != nil {
		return View{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	backup, err := hasBackup(s.dataDir)
	if err != nil {
		return View{}, code(http.StatusServiceUnavailable, "unavailable")
	}
	return View{Text: text, Scene: scene, SceneKeys: scenes.ManagedKeys(), Backup: backup}, nil
}

func (s *Service) activeScene() (string, error) {
	_, list, err := scenes.Catalog(s.repo, s.dataDir, s.managerAddr, "")
	if err != nil {
		return "", err
	}
	for _, item := range list {
		if item.Active {
			return item.ID, nil
		}
	}
	return "", nil
}
