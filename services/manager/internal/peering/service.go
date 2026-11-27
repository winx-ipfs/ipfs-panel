package peering

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/configedit"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/peers"
)

// Editor is the config show/replace path. The raw editor's service satisfies it.
type Editor interface {
	Show(context.Context) (configedit.View, error)
	Replace(context.Context, string) (configedit.Result, error)
}

// Runtime is the live peering service and the swarm, on loopback.
type Runtime interface {
	List(context.Context) ([]Saved, error)
	Add(context.Context, []string) error
	Remove(context.Context, string) error
	Swarm(context.Context) (map[string]struct{}, map[string][]string, error)
}

// Service keeps the saved list and the running peering service together.
type Service struct {
	edit Editor
	rt   Runtime
}

// New builds a service over one editor and one Kubo runtime.
func New(edit Editor, rt Runtime) *Service {
	return &Service{edit: edit, rt: rt}
}

// List returns the saved peers and whether the running service matches them.
func (s *Service) List(ctx context.Context) (View, error) {
	saved, live, connected, err := s.snapshot(ctx)
	if err != nil {
		return View{}, err
	}
	return project(saved, live, connected, ""), nil
}

// AddAddress validates a multiaddr, saves it, and registers it live.
func (s *Service) AddAddress(ctx context.Context, address string) (View, error) {
	addr, id, err := splitAddress(address)
	if err != nil {
		return View{}, err
	}
	if err := s.save(ctx, id, addr); err != nil {
		return View{}, err
	}
	return s.push(ctx, id)
}

// AddPeer saves a connected peer. The address lookup stays in the manager.
func (s *Service) AddPeer(ctx context.Context, peerID string) (View, error) {
	id, ok := peers.CanonicalPeerID(peerID)
	if !ok {
		return View{}, code(http.StatusBadRequest, "invalid_peer")
	}
	_, byID, err := s.rt.Swarm(ctx)
	if err != nil {
		return View{}, err
	}
	found, ok := byID[id]
	if !ok {
		return View{}, code(http.StatusConflict, "not_connected")
	}
	stored := make([]string, 0, len(found))
	for _, addr := range found {
		bare, parsed, splitErr := splitAddress(strings.TrimRight(addr, "/") + "/p2p/" + id)
		if splitErr != nil || parsed != id {
			continue
		}
		stored = append(stored, bare)
	}
	if len(stored) == 0 {
		stored = []string{""}
	}
	for _, addr := range stored {
		if err := s.save(ctx, id, addr); err != nil {
			return View{}, err
		}
	}
	return s.push(ctx, id)
}

// Remove drops a peer id from the saved list and from the running service.
func (s *Service) Remove(ctx context.Context, peerID string) (View, error) {
	id, ok := peers.CanonicalPeerID(peerID)
	if !ok {
		return View{}, code(http.StatusBadRequest, "invalid_peer")
	}
	text, err := s.current(ctx)
	if err != nil {
		return View{}, err
	}
	doc, err := readDoc(text)
	if err != nil {
		return View{}, err
	}
	if removeID(doc, id) {
		next, err := writeDoc(doc)
		if err != nil {
			return View{}, err
		}
		if _, err := s.edit.Replace(ctx, next); err != nil {
			return View{}, fromEdit(err)
		}
	}
	liveErr := s.rt.Remove(ctx, id)
	view, err := s.List(ctx)
	if err != nil {
		return View{}, err
	}
	if liveErr != nil {
		view.Sync = SyncRestart
	}
	return view, nil
}

// Apply pushes the saved list onto the running service. It does not restart.
func (s *Service) Apply(ctx context.Context) (View, error) {
	saved, live, _, err := s.snapshot(ctx)
	if err != nil {
		return View{}, err
	}
	savedSet := indexSaved(saved)
	liveSet := indexSaved(live)
	var liveErr error
	for id := range liveSet {
		if _, ok := savedSet[id]; ok {
			continue
		}
		if err := s.rt.Remove(ctx, id); err != nil {
			liveErr = err
		}
	}
	for id, addrs := range savedSet {
		if sameAddrs(addrs, liveSet[id]) && containsID(live, id) {
			continue
		}
		if err := s.rt.Remove(ctx, id); err != nil && CodeOf(err) != "" {
			// A missing peer still has to be added. Kubo rm of an unknown id
			// is success; any other failure is remembered and Add is still tried.
			liveErr = err
		}
		if err := s.rt.Add(ctx, dialAddrs(id, addrs)); err != nil {
			liveErr = err
		}
	}
	view, err := s.List(ctx)
	if err != nil {
		return View{}, err
	}
	if liveErr != nil {
		view.Sync = SyncRestart
	}
	return view, nil
}

func (s *Service) save(ctx context.Context, id, addr string) error {
	text, err := s.current(ctx)
	if err != nil {
		return err
	}
	doc, err := readDoc(text)
	if err != nil {
		return err
	}
	changed, err := upsert(doc, id, addr)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	next, err := writeDoc(doc)
	if err != nil {
		return err
	}
	if _, err := s.edit.Replace(ctx, next); err != nil {
		return fromEdit(err)
	}
	return nil
}

func (s *Service) push(ctx context.Context, id string) (View, error) {
	text, err := s.current(ctx)
	if err != nil {
		return View{}, err
	}
	doc, err := readDoc(text)
	if err != nil {
		return View{}, err
	}
	_, _, entries := peeringEntries(doc)
	idx := findEntry(entries, id)
	var addrs []string
	if idx >= 0 {
		addrs = entries[idx].addrs
	}
	_ = s.rt.Remove(ctx, id)
	liveErr := s.rt.Add(ctx, dialAddrs(id, addrs))
	view, err := s.List(ctx)
	if err != nil {
		return View{}, err
	}
	if liveErr != nil {
		view.Sync = SyncRestart
	}
	return view, nil
}

func (s *Service) current(ctx context.Context) (string, error) {
	view, err := s.edit.Show(ctx)
	if err != nil {
		return "", fromEdit(err)
	}
	return view.Text, nil
}

func (s *Service) snapshot(ctx context.Context) ([]Saved, []Saved, map[string]struct{}, error) {
	text, err := s.current(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	doc, err := readDoc(text)
	if err != nil {
		return nil, nil, nil, err
	}
	_, _, entries := peeringEntries(doc)
	saved := validEntries(entries)
	live, err := s.rt.List(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	connected, _, err := s.rt.Swarm(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if connected == nil {
		connected = map[string]struct{}{}
	}
	return saved, live, connected, nil
}

func containsID(list []Saved, id string) bool {
	for _, item := range list {
		if item.PeerID == id {
			return true
		}
	}
	return false
}

func fromEdit(err error) error {
	if err == nil {
		return nil
	}
	if CodeOf(err) != "" {
		return err
	}
	var edit *configedit.Error
	if errors.As(err, &edit) && edit != nil {
		return code(edit.Status, edit.Code)
	}
	return code(http.StatusServiceUnavailable, "unavailable")
}
