package node

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// errClosed is returned when the supervisor loop has exited.
var errClosed = errors.New("supervisor closed")

type cmdKind int

const (
	cmdGet cmdKind = iota
	cmdStart
	cmdStop
	cmdRestart
	cmdShutdown
	cmdCached
)

type command struct {
	kind  cmdKind
	reply chan Overview
}

type eventKind int

const (
	evFailed eventKind = iota
	evReady
	evExited
	evStopped
	evAborted
	evProbe
)

type event struct {
	kind    eventKind
	gen     uint64
	child   *running
	peerID  string
	version string
	code    string
	detail  string
	err     error
	clean   bool
}

// Supervisor owns at most one ipfs daemon. HTTP handlers send it a command
// and wait for a snapshot. They do not spawn ipfs themselves.
type Supervisor struct {
	cfg    Config
	cmds   chan command
	events chan event
	ready  chan struct{}
	quit   chan struct{}

	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error
}

type loopState struct {
	state        string
	repo         string
	errCode      string
	errDetail    string
	peerID       string
	version      string
	gen          uint64
	child        *running
	restartAfter bool
	shuttingDown bool
	startCancel  context.CancelFunc
	probing      bool
	waiters      []chan Overview
	shutdownCh   chan Overview
}

// New starts the supervisor. It does not start Kubo until Start or Restart.
func New(cfg Config) (*Supervisor, error) {
	cfg.applyDefaults()
	if strings.TrimSpace(cfg.RepoPath) == "" {
		return nil, errors.New("repo path is empty")
	}
	s := &Supervisor{
		cfg:    cfg,
		cmds:   make(chan command),
		events: make(chan event, 32),
		ready:  make(chan struct{}),
		quit:   make(chan struct{}),
		closed: make(chan struct{}),
	}
	go func() {
		st := &loopState{repo: cfg.RepoPath, state: StateDown}
		s.boot(st)
		close(s.ready)
		s.loop(st)
	}()
	<-s.ready
	return s, nil
}

// Snapshot returns the current overview. While the node is up, it probes
// `ipfs id` before returning.
func (s *Supervisor) Snapshot(ctx context.Context) (Overview, error) {
	return s.call(ctx, cmdGet)
}

// Cached returns the supervisor's state without probing Kubo.
func (s *Supervisor) Cached(ctx context.Context) (Overview, error) {
	return s.call(ctx, cmdCached)
}

// FileLimits returns the upload cap and the files deadlines.
func (s *Supervisor) FileLimits() (maxBytes int64, op, transfer time.Duration) {
	return s.cfg.MaxFileBytes, s.cfg.FilesTimeout, s.cfg.FilesTransferTimeout
}

// DataDir is the manager data directory. Pin labels live here, not in the Kubo repo.
func (s *Supervisor) DataDir() string     { return s.cfg.DataDir }
func (s *Supervisor) RepoPath() string    { return s.cfg.RepoPath }
func (s *Supervisor) ManagerAddr() string { return s.cfg.ManagerAddr }

// PinTimeout is the deadline for one pin/add job.
func (s *Supervisor) PinTimeout() time.Duration { return s.cfg.PinsTimeout }

// Start begins startup when the state table allows it.
// The overview is returned once the state has moved, without waiting for the daemon.
func (s *Supervisor) Start(ctx context.Context) (Overview, error) {
	return s.call(ctx, cmdStart)
}

// Stop begins shutdown of a child this supervisor spawned or adopted.
func (s *Supervisor) Stop(ctx context.Context) (Overview, error) {
	return s.call(ctx, cmdStop)
}

// Restart stops and then starts when no transition is underway.
// From down or a finished failure it only starts.
func (s *Supervisor) Restart(ctx context.Context) (Overview, error) {
	return s.call(ctx, cmdRestart)
}

// Shutdown stops the child, if any, and exits the loop.
func (s *Supervisor) Shutdown(ctx context.Context) error {
	s.closeOnce.Do(func() {
		go func() {
			_, err := s.call(context.Background(), cmdShutdown)
			if errors.Is(err, errClosed) {
				err = nil
			}
			s.closeErr = err
			close(s.closed)
		}()
	})
	select {
	case <-s.closed:
		return s.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Supervisor) call(ctx context.Context, kind cmdKind) (Overview, error) {
	reply := make(chan Overview, 1)
	select {
	case <-ctx.Done():
		return Overview{}, ctx.Err()
	case <-s.quit:
		return Overview{}, errClosed
	case s.cmds <- command{kind: kind, reply: reply}:
	}
	select {
	case <-ctx.Done():
		return Overview{}, ctx.Err()
	case ov := <-reply:
		return ov, nil
	case <-s.quit:
		select {
		case ov := <-reply:
			return ov, nil
		default:
			return Overview{}, errClosed
		}
	}
}

func (st *loopState) overview() Overview {
	ov := Overview{State: st.state, RepoPath: st.repo}
	if st.state == StateUp {
		if st.peerID != "" {
			peer := st.peerID
			ov.PeerID = &peer
		}
		if st.version != "" {
			version := st.version
			ov.Version = &version
		}
	}
	if st.errCode != "" {
		ov.Error = &NodeError{Code: st.errCode, Detail: st.errDetail}
	}
	return ov
}

func (s *Supervisor) loop(st *loopState) {
	for {
		select {
		case c := <-s.cmds:
			if s.handle(st, c) {
				return
			}
		case ev := <-s.events:
			if s.apply(st, ev) {
				return
			}
		}
	}
}

func (s *Supervisor) handle(st *loopState, c command) bool {
	if c.kind == cmdShutdown {
		return s.beginShutdown(st, c.reply)
	}
	if st.shuttingDown {
		c.reply <- st.overview()
		return false
	}
	switch c.kind {
	case cmdGet:
		s.handleGet(st, c)
	case cmdStart:
		s.handleStart(st)
		c.reply <- st.overview()
	case cmdStop:
		s.handleStop(st)
		c.reply <- st.overview()
	case cmdRestart:
		s.handleRestart(st)
		c.reply <- st.overview()
	case cmdCached:
		c.reply <- st.overview()
	default:
		c.reply <- st.overview()
	}
	return false
}

func (s *Supervisor) handleGet(st *loopState, c command) {
	if st.state != StateUp {
		c.reply <- st.overview()
		return
	}
	st.waiters = append(st.waiters, c.reply)
	if st.probing {
		return
	}
	bin, cerr := resolveBin(s.cfg.KuboBin)
	if cerr != nil {
		if st.child == nil || st.child.exited() {
			s.markFailed(st, CodeDaemonExited, cerr.detail)
		}
		s.flush(st)
		return
	}
	st.probing = true
	go s.runProbe(st.gen, bin)
}

func (s *Supervisor) handleStart(st *loopState) {
	switch st.state {
	case StateDown:
		s.beginStart(st)
	case StateFailed:
		if st.errCode == CodeRepoBusy {
			return
		}
		if st.child != nil && !st.child.exited() {
			return
		}
		s.beginStart(st)
	}
}

func (s *Supervisor) handleStop(st *loopState) {
	if st.state == StateUp && st.child != nil && !st.child.exited() {
		s.beginStop(st, false)
	}
}

func (s *Supervisor) handleRestart(st *loopState) {
	switch st.state {
	case StateDown:
		s.beginStart(st)
	case StateUp:
		if st.child != nil && !st.child.exited() {
			s.beginStop(st, true)
		}
	case StateFailed:
		if st.errCode == CodeRepoBusy {
			return
		}
		if st.child != nil && !st.child.exited() {
			return
		}
		s.beginStart(st)
	}
}

func (s *Supervisor) beginShutdown(st *loopState, reply chan Overview) bool {
	st.shuttingDown = true
	st.restartAfter = false
	st.shutdownCh = reply
	if st.startCancel != nil {
		st.startCancel()
	}
	switch st.state {
	case StateUp:
		if st.child != nil && !st.child.exited() {
			s.beginStop(st, false)
			return false
		}
		return s.finishShutdown(st)
	case StateStarting, StateStopping:
		return false
	default:
		return s.finishShutdown(st)
	}
}

func (s *Supervisor) beginStart(st *loopState) {
	if st.startCancel != nil {
		st.startCancel()
		st.startCancel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	st.startCancel = cancel
	st.gen++
	st.state = StateStarting
	st.errCode = ""
	st.errDetail = ""
	st.peerID = ""
	st.version = ""
	st.restartAfter = false
	st.child = nil
	st.probing = false
	s.flush(st)
	s.logState(st)
	go s.runStart(ctx, st.gen)
}

func (s *Supervisor) beginStop(st *loopState, restart bool) {
	st.gen++
	st.state = StateStopping
	st.restartAfter = restart
	st.probing = false
	s.flush(st)
	s.logState(st)
	bin, _ := resolveBin(s.cfg.KuboBin)
	go s.runStop(st.gen, st.child, bin)
}

func (s *Supervisor) apply(st *loopState, ev event) bool {
	switch ev.kind {
	case evReady:
		return s.onReady(st, ev)
	case evFailed:
		return s.onFailed(st, ev)
	case evAborted:
		return s.onAborted(st, ev)
	case evStopped:
		return s.onStopped(st, ev)
	case evExited:
		return s.onExited(st, ev)
	case evProbe:
		return s.onProbe(st, ev)
	default:
		return false
	}
}

func (s *Supervisor) onReady(st *loopState, ev event) bool {
	if ev.gen != st.gen || st.state != StateStarting {
		if ev.child != nil && ev.child != st.child && !ev.child.exited() {
			_ = kill(ev.child.pid)
		}
		return false
	}
	st.child = ev.child
	st.state = StateUp
	st.peerID = ev.peerID
	st.version = ev.version
	st.errCode = ""
	st.errDetail = ""
	go s.watchExit(st.gen, ev.child)
	s.logState(st)
	if st.shuttingDown {
		if st.child != nil && !st.child.exited() {
			s.beginStop(st, false)
			return false
		}
		return s.finishShutdown(st)
	}
	return false
}

func (s *Supervisor) onFailed(st *loopState, ev event) bool {
	if ev.gen != st.gen {
		return false
	}
	s.markFailed(st, ev.code, ev.detail)
	s.flush(st)
	if st.shuttingDown {
		return s.finishShutdown(st)
	}
	return false
}

func (s *Supervisor) onAborted(st *loopState, ev event) bool {
	if ev.gen != st.gen {
		return false
	}
	st.child = nil
	removePid(st.repo)
	st.probing = false
	if st.shuttingDown {
		st.state = StateDown
		st.errCode = ""
		st.errDetail = ""
		return s.finishShutdown(st)
	}
	s.markFailed(st, CodeUnknown, "start aborted")
	s.flush(st)
	return false
}

func (s *Supervisor) onStopped(st *loopState, ev event) bool {
	if ev.gen != st.gen {
		return false
	}
	st.child = nil
	removePid(st.repo)
	st.probing = false
	st.state = StateDown
	if ev.clean {
		st.errCode = ""
		st.errDetail = ""
	} else {
		st.errCode = CodeStopTimeout
		st.errDetail = capDetail(ev.detail)
		if st.errDetail == "" {
			st.errDetail = "process still alive after stop deadline; sent SIGKILL"
		}
	}
	s.logState(st)
	s.flush(st)
	if st.restartAfter && !st.shuttingDown {
		st.restartAfter = false
		s.beginStart(st)
		return false
	}
	st.restartAfter = false
	if st.shuttingDown {
		return s.finishShutdown(st)
	}
	return false
}

func (s *Supervisor) onExited(st *loopState, ev event) bool {
	if ev.gen != st.gen || st.state != StateUp {
		return false
	}
	st.probing = false
	detail := ev.detail
	if strings.TrimSpace(detail) == "" {
		detail = "process exited"
	}
	s.markFailed(st, CodeDaemonExited, detail)
	s.flush(st)
	if st.shuttingDown {
		return s.finishShutdown(st)
	}
	return false
}

func (s *Supervisor) onProbe(st *loopState, ev event) bool {
	st.probing = false
	if ev.gen == st.gen && st.state == StateUp {
		if ev.err == nil {
			st.peerID = ev.peerID
			st.version = ev.version
			st.errCode = ""
			st.errDetail = ""
		} else if st.child == nil || st.child.exited() {
			detail := ev.detail
			if strings.TrimSpace(detail) == "" && ev.err != nil {
				detail = ev.err.Error()
			}
			s.markFailed(st, CodeDaemonExited, detail)
		}
	}
	s.flush(st)
	if st.shuttingDown && st.state != StateStarting && st.state != StateStopping {
		return s.finishShutdown(st)
	}
	return false
}

func (s *Supervisor) markFailed(st *loopState, code, detail string) {
	st.child = nil
	removePid(st.repo)
	st.state = StateFailed
	st.errCode = code
	if st.errCode == "" {
		st.errCode = CodeUnknown
	}
	st.errDetail = capDetail(detail)
	st.peerID = ""
	st.version = ""
	s.logState(st)
}

func (s *Supervisor) flush(st *loopState) {
	if len(st.waiters) == 0 {
		return
	}
	ov := st.overview()
	for _, ch := range st.waiters {
		ch <- ov
	}
	st.waiters = nil
}

func (s *Supervisor) finishShutdown(st *loopState) bool {
	s.flush(st)
	if st.shutdownCh != nil {
		st.shutdownCh <- st.overview()
		st.shutdownCh = nil
	}
	select {
	case <-s.quit:
	default:
		close(s.quit)
	}
	return true
}

func (s *Supervisor) logState(st *loopState) {
	if st.errCode == "" {
		s.cfg.Logger.Info("node", "state", st.state)
		return
	}
	s.cfg.Logger.Info("node", "state", st.state, "code", st.errCode)
}

func (s *Supervisor) emit(ev event) {
	select {
	case s.events <- ev:
	case <-s.quit:
	}
}

func (s *Supervisor) boot(st *loopState) {
	pid, hasPID := readPid(st.repo)
	if hasPID && !pidAlive(pid) {
		removePid(st.repo)
		hasPID = false
	}
	if hasPID && pidAlive(pid) {
		bin, cerr := resolveBin(s.cfg.KuboBin)
		if cerr == nil && sameDaemon(pid, bin, st.repo) {
			st.child = adoptProcess(pid)
			ctx, cancel := context.WithCancel(context.Background())
			st.startCancel = cancel
			st.gen++
			st.state = StateStarting
			s.logState(st)
			go s.waitReady(ctx, st.gen, bin, st.child)
			return
		}
		held, err := repoLockHeld(st.repo)
		if err != nil {
			st.state = StateFailed
			st.errCode = CodeUnknown
			st.errDetail = capDetail(err.Error())
			s.logState(st)
			return
		}
		if held {
			st.state = StateFailed
			st.errCode = CodeRepoBusy
			st.errDetail = "repo lock is held by another process"
			s.logState(st)
			return
		}
		removePid(st.repo)
	}
	held, err := repoLockHeld(st.repo)
	if err != nil {
		st.state = StateFailed
		st.errCode = CodeUnknown
		st.errDetail = capDetail(err.Error())
		s.logState(st)
		return
	}
	if held {
		st.state = StateFailed
		st.errCode = CodeRepoBusy
		st.errDetail = "repo lock is held by another process"
		s.logState(st)
		return
	}
	st.state = StateDown
}
