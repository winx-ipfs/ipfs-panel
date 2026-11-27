package kuboinstall

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	keepJobs     = 8
	catalogEvery = 15 * time.Minute
	versionEvery = time.Minute
)

// Controller is the node supervisor. The install job stops and starts
// through it so the binary swap cannot race a click on Start.
type Controller interface {
	Snapshot(ctx context.Context) (state, errCode string, err error)
	Stop(ctx context.Context) error
	Start(ctx context.Context) error
	Hold(on bool)
}

// Actor is the signed-in person, for the audit line only.
type Actor struct {
	Username string
	IP       string
}

// Options configures one service. Empty origins use the operator
// environment, then the public defaults. Pins, when set, replaces the
// compiled table so a test can serve its own archive.
type Options struct {
	DataDir     string
	RepoDir     string
	ExplicitBin string
	DistBase    string
	GitHubBase  string
	VersionsURL string
	GitHubAPI   string
	HTTP        *http.Client
	GOOS        string
	GOARCH      string
	Pins        map[string]string
	Node        Controller
	Logger      *slog.Logger
	LookPath    func(string) (string, error)
	StartWait   time.Duration
	StopWait    time.Duration
}

// Status is the body of GET /api/kubo, without the job.
type Status struct {
	Source    string
	Version   string
	Pinned    string
	Supported string
	Latest    string
	Upstream  string
	Upgrade   bool
	Check     string
	Job       *Job
}

// Job is one install or upgrade. Counts are bytes.
type Job struct {
	ID       string
	Action   string
	State    string
	Phase    string
	Version  string
	Received int64
	Total    int64
	Error    string
	Created  time.Time
}

// Service downloads, checks, and swaps a Kubo binary.
type Service struct {
	dataDir     string
	repoDir     string
	explicit    string
	distBase    string
	githubBase  string
	versionsURL string
	githubAPI   string
	http        *http.Client
	goos        string
	goarch      string
	pins        map[string]string
	node        Controller
	log         *slog.Logger
	look        func(string) (string, error)
	startWait   time.Duration
	stopWait    time.Duration

	mu      sync.Mutex
	jobs    map[string]*Job
	order   []string
	running string

	catMu      sync.Mutex
	catAt      time.Time
	catOK      bool
	latest     string
	upstream   string
	extMu      sync.Mutex
	extPath    string
	extVersion string
	extAt      time.Time
}

// New returns a service. It does not touch the network.
func New(opts Options) *Service {
	if opts.DistBase == "" {
		opts.DistBase = strings.TrimSpace(os.Getenv("IPFS_PANEL_KUBO_DIST"))
	}
	if opts.DistBase == "" {
		opts.DistBase = "https://dist.ipfs.tech"
	}
	if opts.GitHubBase == "" {
		opts.GitHubBase = strings.TrimSpace(os.Getenv("IPFS_PANEL_KUBO_GITHUB"))
	}
	if opts.GitHubBase == "" {
		opts.GitHubBase = "https://github.com/ipfs/kubo/releases/download"
	}
	if opts.VersionsURL == "" {
		opts.VersionsURL = strings.TrimRight(opts.DistBase, "/") + "/kubo/versions"
	}
	if opts.GitHubAPI == "" {
		opts.GitHubAPI = "https://api.github.com/repos/ipfs/kubo/releases?per_page=100"
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 20 * time.Second, Proxy: http.ProxyFromEnvironment}}
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}
	if opts.StartWait <= 0 {
		opts.StartWait = 45 * time.Second
	}
	if opts.StopWait <= 0 {
		opts.StopWait = 40 * time.Second
	}
	return &Service{
		dataDir:     opts.DataDir,
		repoDir:     opts.RepoDir,
		explicit:    strings.TrimSpace(opts.ExplicitBin),
		distBase:    opts.DistBase,
		githubBase:  opts.GitHubBase,
		versionsURL: opts.VersionsURL,
		githubAPI:   opts.GitHubAPI,
		http:        opts.HTTP,
		goos:        opts.GOOS,
		goarch:      opts.GOARCH,
		pins:        opts.Pins,
		node:        opts.Node,
		log:         opts.Logger,
		look:        opts.LookPath,
		startWait:   opts.StartWait,
		stopWait:    opts.StopWait,
		jobs:        map[string]*Job{},
	}
}

func (s *Service) pin() string {
	if s.pins != nil {
		return s.pins[platformKey(s.goos, s.goarch)]
	}
	return PinnedDigest(s.goos, s.goarch)
}

// Source reports which binary would run. It does not use the network.
func (s *Service) Source() string {
	if s.explicit != "" {
		return "flag"
	}
	if CurrentPath(s.dataDir) != "" {
		return "managed"
	}
	if s.look != nil {
		if path, err := s.look("ipfs"); err == nil && path != "" {
			return "path"
		}
	}
	return "missing"
}

// Status reports the installed binary and whether a newer one is inside
// the supported range. A catalog failure leaves the binary alone.
func (s *Service) Status(ctx context.Context) Status {
	src := s.Source()
	st := Status{
		Source:    src,
		Pinned:    Pinned,
		Supported: SupportedLabel,
		Check:     "ok",
		Job:       s.currentJob(),
	}
	switch src {
	case "managed":
		disk, err := readState(s.dataDir)
		if err == nil {
			st.Version = disk.Current
		}
	case "flag", "path":
		st.Version = s.externalVersion(ctx)
	}
	latest, upstream, check := s.catalog(ctx)
	st.Latest = latest
	st.Upstream = upstream
	st.Check = check
	if src == "managed" && check == "ok" && newer(latest, st.Version) {
		st.Upgrade = true
	}
	return st
}

func newer(latest, current string) bool {
	a, aOK := parseVer(latest)
	b, bOK := parseVer(current)
	if !aOK || !bOK {
		return false
	}
	return b.less(a)
}

func (s *Service) catalog(ctx context.Context) (latest, upstream, check string) {
	s.catMu.Lock()
	if s.catOK && time.Since(s.catAt) < catalogEvery {
		latest, upstream = s.latest, s.upstream
		s.catMu.Unlock()
		return latest, upstream, "ok"
	}
	s.catMu.Unlock()
	lines, err := s.fetchCatalog(ctx)
	if err != nil {
		return "", "", "offline"
	}
	latest, upstream = pick(lines)
	s.catMu.Lock()
	s.latest, s.upstream, s.catAt, s.catOK = latest, upstream, time.Now(), true
	s.catMu.Unlock()
	return latest, upstream, "ok"
}

func (s *Service) externalVersion(ctx context.Context) string {
	path := s.explicit
	if path == "" && s.look != nil {
		found, err := s.look("ipfs")
		if err != nil {
			return ""
		}
		path = found
	}
	if path == "" {
		return ""
	}
	s.extMu.Lock()
	if s.extPath == path && time.Since(s.extAt) < versionEvery {
		v := s.extVersion
		s.extMu.Unlock()
		return v
	}
	s.extMu.Unlock()
	ver, err := binaryVersion(ctx, path)
	if err != nil {
		ver = ""
	}
	s.extMu.Lock()
	s.extPath, s.extVersion, s.extAt = path, ver, time.Now()
	s.extMu.Unlock()
	return ver
}

// Install starts a job that fetches the pin. The job keeps running after
// this returns.
func (s *Service) Install(actor Actor) (Job, error) {
	return s.begin("install", "kubo_install", Pinned, actor, func(src string) *Error {
		switch src {
		case "flag", "path":
			s.audit(actor, "kubo_install", "external", "", "")
			return errCode(CodeExternal)
		case "managed":
			s.audit(actor, "kubo_install", "up_to_date", "", "")
			return errCode(CodeAlready)
		default:
			if !supportedPlatform(s.goos, s.goarch) {
				s.audit(actor, "kubo_install", "invalid", Pinned, "")
				return errCode(CodeUnsupported)
			}
			return nil
		}
	})
}

// Upgrade starts a job for the newest supported release newer than the
// managed binary.
func (s *Service) Upgrade(actor Actor) (Job, error) {
	src := s.Source()
	switch src {
	case "flag", "path":
		s.audit(actor, "kubo_upgrade", "external", "", "")
		return Job{}, errCode(CodeExternal)
	case "missing":
		s.audit(actor, "kubo_upgrade", "not_installed", "", "")
		return Job{}, errCode(CodeNotInstalled)
	}
	if !supportedPlatform(s.goos, s.goarch) {
		s.audit(actor, "kubo_upgrade", "invalid", "", "")
		return Job{}, errCode(CodeUnsupported)
	}
	st := s.Status(context.Background())
	if st.Check == "offline" {
		s.audit(actor, "kubo_upgrade", "offline", st.Version, "")
		return Job{}, errCode(CodeOffline)
	}
	if !st.Upgrade {
		s.audit(actor, "kubo_upgrade", "up_to_date", st.Version, "")
		return Job{}, errCode(CodeUpToDate)
	}
	return s.begin("upgrade", "kubo_upgrade", st.Latest, actor, func(string) *Error { return nil })
}

func (s *Service) begin(action, event, version string, actor Actor, gate func(source string) *Error) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running != "" {
		s.audit(actor, event, "busy", version, "")
		return Job{}, errCode(CodeBusy)
	}
	if gate != nil {
		if err := gate(s.Source()); err != nil {
			return Job{}, err
		}
	}
	id, err := newID()
	if err != nil {
		return Job{}, err
	}
	job := &Job{
		ID:      id,
		Action:  action,
		State:   "running",
		Phase:   "downloading",
		Version: version,
		Created: time.Now().UTC(),
	}
	s.jobs[id] = job
	s.order = append(s.order, id)
	s.running = id
	s.pruneLocked()
	snap := *job
	go s.run(id, action, version, actor)
	return snap, nil
}

// Job returns one remembered job.
func (s *Service) Job(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[id]
	if job == nil {
		return Job{}, errCode(CodeNotFound)
	}
	return *job, nil
}

func (s *Service) currentJob() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.order) == 0 {
		return nil
	}
	job := s.jobs[s.order[len(s.order)-1]]
	if job == nil {
		return nil
	}
	snap := *job
	return &snap
}

func (s *Service) pruneLocked() {
	if len(s.order) <= keepJobs {
		return
	}
	drop := len(s.order) - keepJobs
	for _, id := range s.order[:drop] {
		if id == s.running {
			continue
		}
		delete(s.jobs, id)
	}
	s.order = s.order[drop:]
}

func (s *Service) run(id, action, version string, actor Actor) {
	defer s.clearRunning(id)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	event := "kubo_install"
	if action == "upgrade" {
		event = "kubo_upgrade"
	}
	archive, sum, err := s.download(ctx, version, func(got, total int64) {
		s.setProgress(id, got, total)
	})
	if err != nil {
		s.fail(id, actor, event, version, "", err)
		return
	}
	defer func() { _ = os.Remove(archive) }()

	s.setPhase(id, "verifying")
	dest := versionDir(s.dataDir, version)
	exe := exeName(s.goos)
	zipArchive := strings.HasSuffix(assetMust(version, s.goos, s.goarch), ".zip")
	if err := extractBinary(archive, dest, zipArchive, exe); err != nil {
		s.log.Warn("kubo extract", "err", err)
		s.fail(id, actor, event, version, "", errCode(CodeVerify))
		return
	}
	bin := filepath.Join(dest, exe)
	got, err := binaryVersion(ctx, bin)
	if err != nil || got != version {
		_ = os.RemoveAll(dest)
		s.fail(id, actor, event, version, "", errCode(CodeVerify))
		return
	}

	if s.node != nil {
		s.node.Hold(true)
		defer s.node.Hold(false)
		s.setPhase(id, "stopping")
		if err := s.ensureStopped(ctx); err != nil {
			_ = os.RemoveAll(dest)
			s.fail(id, actor, event, version, "", errCode(CodeStop))
			return
		}
	}

	s.setPhase(id, "swapping")
	previous, err := s.activate(version, sum)
	if err != nil {
		s.log.Warn("kubo state", "err", err)
		s.fail(id, actor, event, version, "", errCode(CodeStart))
		return
	}
	before, had := repoVersion(s.repoDir)
	if had {
		s.setPhase(id, "migrating")
		if err := runMigrate(ctx, bin, s.repoDir); err != nil {
			s.log.Warn("kubo migrate", "err", err)
			after, hadAfter := repoVersion(s.repoDir)
			moved := hadAfter && after != before
			if moved || previous == "" {
				s.fail(id, actor, event, version, previous, errCode(CodeMigrate))
				return
			}
			s.rollback(ctx, id, actor, event, version, previous)
			return
		}
	}

	s.setPhase(id, "starting")
	if err := s.startAndWait(ctx); err != nil {
		after, hadAfter := repoVersion(s.repoDir)
		moved := had && hadAfter && after != before
		if moved || previous == "" {
			s.fail(id, actor, event, version, previous, errCode(CodeStart))
			return
		}
		s.rollback(ctx, id, actor, event, version, previous)
		return
	}
	s.succeed(id, actor, event, version, previous)
}

func assetMust(version, goos, goarch string) string {
	name, err := assetName(version, goos, goarch)
	if err != nil {
		return ""
	}
	return name
}

func (s *Service) activate(version, sum string) (string, error) {
	st, err := readState(s.dataDir)
	if err != nil {
		return "", err
	}
	previous := st.Current
	if previous == version {
		previous = st.Previous
	}
	if previous == version {
		previous = ""
	}
	prevSum := st.SHA512
	if previous == "" {
		prevSum = ""
	}
	next := diskState{Current: version, Previous: previous, SHA512: sum, PreviousSHA512: prevSum}
	if err := writeState(s.dataDir, next); err != nil {
		return "", err
	}
	return previous, nil
}

func (s *Service) restore(previous string) error {
	st, err := readState(s.dataDir)
	if err != nil {
		return err
	}
	st.Current = previous
	st.SHA512 = st.PreviousSHA512
	st.Previous = ""
	st.PreviousSHA512 = ""
	return writeState(s.dataDir, st)
}

func (s *Service) ensureStopped(ctx context.Context) error {
	if s.node == nil {
		return nil
	}
	deadline := time.Now().Add(s.stopWait)
	asked := false
	for {
		state, code, err := s.node.Snapshot(ctx)
		if err != nil {
			return err
		}
		switch state {
		case "down", "failed":
			if code == "stop_timeout" {
				return errCode(CodeStop)
			}
			return nil
		case "up":
			if !asked {
				asked = true
				if err := s.node.Stop(ctx); err != nil {
					return err
				}
			}
		}
		if time.Now().After(deadline) {
			return errCode(CodeStop)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (s *Service) startAndWait(ctx context.Context) error {
	if s.node == nil {
		return nil
	}
	if err := s.node.Start(ctx); err != nil {
		return err
	}
	deadline := time.Now().Add(s.startWait)
	for {
		state, _, err := s.node.Snapshot(ctx)
		if err != nil {
			return err
		}
		switch state {
		case "up":
			return nil
		case "failed":
			return errCode(CodeStart)
		}
		if time.Now().After(deadline) {
			return errCode(CodeStart)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (s *Service) rollback(ctx context.Context, id string, actor Actor, event, version, previous string) {
	s.setPhase(id, "rolling_back")
	if previous == "" {
		s.fail(id, actor, event, version, previous, errCode(CodeRollback))
		return
	}
	if err := s.restore(previous); err != nil {
		s.log.Warn("kubo rollback", "err", err)
		s.fail(id, actor, event, version, previous, errCode(CodeRollback))
		return
	}
	if err := s.startAndWait(ctx); err != nil {
		s.log.Warn("kubo rollback start", "err", err)
		s.fail(id, actor, event, version, previous, errCode(CodeRollback))
		return
	}
	s.mu.Lock()
	if job := s.jobs[id]; job != nil && job.State == "running" {
		job.State = "rolled_back"
		job.Phase = "rolling_back"
		job.Error = ""
	}
	s.mu.Unlock()
	s.audit(actor, event, "rolled_back", version, previous)
}

func (s *Service) succeed(id string, actor Actor, event, version, previous string) {
	s.mu.Lock()
	if job := s.jobs[id]; job != nil && job.State == "running" {
		job.State = "succeeded"
		job.Phase = "starting"
		job.Error = ""
	}
	s.mu.Unlock()
	s.audit(actor, event, "success", version, previous)
}

func (s *Service) fail(id string, actor Actor, event, version, previous string, err error) {
	code := CodeVerify
	var ke *Error
	if errors.As(err, &ke) && ke.Code != "" {
		code = ke.Code
	}
	s.mu.Lock()
	if job := s.jobs[id]; job != nil && job.State == "running" {
		job.State = "failed"
		job.Error = code
	}
	s.mu.Unlock()
	result := code
	if code == CodeExternal {
		result = "external"
	}
	s.audit(actor, event, result, version, previous)
}

func (s *Service) setPhase(id, phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job := s.jobs[id]; job != nil && job.State == "running" {
		job.Phase = phase
	}
}

func (s *Service) setProgress(id string, got, total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[id]
	if job == nil || job.State != "running" {
		return
	}
	if got >= job.Received {
		job.Received = got
	}
	if total > 0 {
		job.Total = total
	}
}

func (s *Service) clearRunning(id string) {
	s.mu.Lock()
	if s.running == id {
		s.running = ""
	}
	s.mu.Unlock()
}

func (s *Service) audit(actor Actor, event, result, version, previous string) {
	if s.log == nil {
		return
	}
	args := []any{
		"event", event,
		"result", result,
		"username", actor.Username,
		"ip", actor.IP,
		"version", version,
	}
	if previous != "" {
		args = append(args, "previous", previous)
	}
	s.log.Info("audit", args...)
}

// Busy reports whether a job currently holds the node.
func (s *Service) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running != ""
}

// SortVersions is used by tests that want a stable catalog assertion.
func SortVersions(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
