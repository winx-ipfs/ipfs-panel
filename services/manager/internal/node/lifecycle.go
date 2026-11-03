package node

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Supervisor) runProbe(gen uint64, bin string) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.IDTimeout)
	defer cancel()
	peer, version, err := s.ipfsID(ctx, bin)
	ev := event{kind: evProbe, gen: gen, peerID: peer, version: version, err: err}
	if err != nil {
		ev.detail = err.Error()
	}
	s.emit(ev)
}

func (s *Supervisor) watchExit(gen uint64, child *running) {
	if child == nil {
		return
	}
	<-child.done
	s.emit(event{kind: evExited, gen: gen, detail: child.stderrString()})
}

func (s *Supervisor) runStart(ctx context.Context, gen uint64) {
	bin, cerr := resolveBin(s.cfg.KuboBin)
	if cerr != nil {
		s.fail(ctx, gen, cerr)
		return
	}
	if cerr = ensureRepo(s.cfg.RepoPath); cerr != nil {
		s.fail(ctx, gen, cerr)
		return
	}
	if ctx.Err() != nil {
		s.abort(gen)
		return
	}
	held, err := repoLockHeld(s.cfg.RepoPath)
	if err != nil {
		s.fail(ctx, gen, codeErr(CodeUnknown, err.Error()))
		return
	}
	if held {
		s.fail(ctx, gen, codeErr(CodeRepoBusy, "repo lock is held by another process"))
		return
	}
	configFile := filepath.Join(s.cfg.RepoPath, "config")
	_, statErr := os.Stat(configFile)
	switch {
	case errors.Is(statErr, os.ErrNotExist):
		if cerr = s.initAndPin(ctx, bin); cerr != nil {
			s.fail(ctx, gen, cerr)
			return
		}
	case statErr != nil:
		s.fail(ctx, gen, codeErr(CodeRepoNotWritable, statErr.Error()))
		return
	}
	if ctx.Err() != nil {
		s.abort(gen)
		return
	}
	if cerr = s.readAndCheck(ctx, bin); cerr != nil {
		s.fail(ctx, gen, cerr)
		return
	}
	child, err := spawnDaemon(bin, s.cfg.RepoPath)
	if err != nil {
		s.fail(ctx, gen, spawnDetail(err))
		return
	}
	if err := writePid(s.cfg.RepoPath, child.pid); err != nil {
		_ = kill(child.pid)
		s.waitDone(child)
		s.fail(ctx, gen, codeErr(CodeUnknown, err.Error()))
		return
	}
	s.waitReady(ctx, gen, bin, child)
}

func (s *Supervisor) initAndPin(ctx context.Context, bin string) *codedError {
	if err := ctx.Err(); err != nil {
		return codeErr(CodeUnknown, err.Error())
	}
	cctx, cancel := context.WithTimeout(ctx, s.cfg.InitTimeout)
	res := run(cctx, bin, []string{"init"}, s.cfg.RepoPath)
	cancel()
	if err := ctx.Err(); err != nil {
		return codeErr(CodeUnknown, err.Error())
	}
	if res.err != nil {
		detail := strings.TrimSpace(res.stderr)
		if detail == "" {
			detail = "ipfs init failed"
		}
		return codeErr(CodeInitFailed, detail)
	}
	doc, cerr := s.readAddresses(ctx, bin)
	if cerr != nil {
		return cerr
	}
	kind, _, err := addressValues(doc.API)
	if err != nil {
		return codeErr(CodeUnknown, err.Error())
	}
	if kind == "" {
		kind = "string"
	}
	if cerr = s.writeAPI(ctx, bin, kind); cerr != nil {
		return cerr
	}
	return s.writeGateway(ctx, bin)
}

func (s *Supervisor) writeAPI(ctx context.Context, bin, kind string) *codedError {
	addr := apiMultiaddr()
	var args []string
	if kind == "array" {
		args = []string{"config", "--json", "Addresses.API", fmtJSON([]string{addr})}
	} else {
		args = []string{"config", "Addresses.API", addr}
	}
	return s.configSet(ctx, bin, args)
}

func (s *Supervisor) writeGateway(ctx context.Context, bin string) *codedError {
	addr := gatewayMultiaddr(s.cfg.ManagerAddr)
	args := []string{"config", "--json", "Addresses.Gateway", fmtJSON([]string{addr})}
	return s.configSet(ctx, bin, args)
}

func (s *Supervisor) configSet(ctx context.Context, bin string, args []string) *codedError {
	if err := ctx.Err(); err != nil {
		return codeErr(CodeUnknown, err.Error())
	}
	cctx, cancel := context.WithTimeout(ctx, s.cfg.ConfigTimeout)
	defer cancel()
	res := run(cctx, bin, args, s.cfg.RepoPath)
	if err := ctx.Err(); err != nil {
		return codeErr(CodeUnknown, err.Error())
	}
	if res.err != nil {
		detail := strings.TrimSpace(res.stderr)
		if detail == "" {
			detail = "ipfs config failed"
		}
		return codeErr(CodeUnknown, detail)
	}
	return nil
}

func (s *Supervisor) readAndCheck(ctx context.Context, bin string) *codedError {
	doc, cerr := s.readAddresses(ctx, bin)
	if cerr != nil {
		return cerr
	}
	return validateAddressDoc(doc, s.cfg.ManagerAddr)
}

func (s *Supervisor) readAddresses(ctx context.Context, bin string) (addressDoc, *codedError) {
	if err := ctx.Err(); err != nil {
		return addressDoc{}, codeErr(CodeUnknown, err.Error())
	}
	cctx, cancel := context.WithTimeout(ctx, s.cfg.ConfigTimeout)
	defer cancel()
	res := run(cctx, bin, []string{"config", "--json", "Addresses"}, s.cfg.RepoPath)
	if err := ctx.Err(); err != nil {
		return addressDoc{}, codeErr(CodeUnknown, err.Error())
	}
	if res.err != nil {
		detail := strings.TrimSpace(res.stderr)
		if detail == "" {
			detail = "ipfs config Addresses failed"
		}
		return addressDoc{}, codeErr(CodeUnknown, detail)
	}
	var doc addressDoc
	if err := json.Unmarshal(res.stdout, &doc); err != nil {
		return addressDoc{}, codeErr(CodeUnknown, "ipfs config Addresses returned invalid JSON")
	}
	return doc, nil
}

func (s *Supervisor) ipfsID(ctx context.Context, bin string) (string, string, error) {
	res := run(ctx, bin, []string{"id"}, s.cfg.RepoPath)
	if res.err != nil {
		if strings.TrimSpace(res.stderr) != "" {
			return "", "", errors.New(strings.TrimSpace(res.stderr))
		}
		return "", "", res.err
	}
	return parseID(res.stdout)
}

func (s *Supervisor) waitReady(ctx context.Context, gen uint64, bin string, child *running) {
	deadline := time.Now().Add(s.cfg.ReadyBudget)
	for {
		if ctx.Err() != nil {
			_ = kill(child.pid)
			s.waitDone(child)
			removePid(s.cfg.RepoPath)
			s.abort(gen)
			return
		}
		if child.exited() {
			s.waitDone(child)
			removePid(s.cfg.RepoPath)
			detail := strings.TrimSpace(child.stderrString())
			code := CodeDaemonExited
			if portInUseText(detail) {
				code = CodePortInUse
			}
			if detail == "" {
				detail = "ipfs daemon exited"
			}
			s.fail(ctx, gen, codeErr(code, detail))
			return
		}
		if time.Now().After(deadline) {
			_ = kill(child.pid)
			s.waitDone(child)
			removePid(s.cfg.RepoPath)
			detail := strings.TrimSpace(child.stderrString())
			if detail == "" {
				detail = "ipfs id did not succeed before the ready budget elapsed"
			}
			s.fail(ctx, gen, codeErr(CodeHealthTimeout, detail))
			return
		}
		idCtx, cancel := context.WithTimeout(ctx, s.cfg.IDTimeout)
		peer, version, err := s.ipfsID(idCtx, bin)
		cancel()
		if err == nil {
			if ctx.Err() != nil {
				_ = kill(child.pid)
				s.waitDone(child)
				removePid(s.cfg.RepoPath)
				s.abort(gen)
				return
			}
			s.emit(event{kind: evReady, gen: gen, child: child, peerID: peer, version: version})
			return
		}
		select {
		case <-ctx.Done():
		case <-child.done:
		case <-time.After(s.cfg.PollInterval):
		}
	}
}

func (s *Supervisor) runStop(gen uint64, child *running, bin string) {
	if child == nil {
		s.emit(event{kind: evStopped, gen: gen, clean: true})
		return
	}
	started := time.Now()
	if bin != "" {
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		_ = run(ctx, bin, []string{"shutdown"}, s.cfg.RepoPath)
		cancel()
	}
	if child.exited() {
		s.waitDone(child)
		s.emit(event{kind: evStopped, gen: gen, clean: true})
		return
	}
	_ = terminate(child.pid)
	remain := s.cfg.StopKillAfter - time.Since(started)
	if remain < 0 {
		remain = 0
	}
	timer := time.NewTimer(remain)
	select {
	case <-child.done:
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		s.emit(event{kind: evStopped, gen: gen, clean: true})
	case <-timer.C:
		_ = kill(child.pid)
		s.waitDone(child)
		detail := strings.TrimSpace(child.stderrString())
		if detail == "" {
			detail = "process still alive after stop deadline; sent SIGKILL"
		}
		s.emit(event{kind: evStopped, gen: gen, clean: false, detail: detail})
	}
}

func (s *Supervisor) waitDone(child *running) {
	if child == nil {
		return
	}
	select {
	case <-child.done:
	case <-time.After(3 * time.Second):
	}
}

func (s *Supervisor) fail(ctx context.Context, gen uint64, err *codedError) {
	if ctx.Err() != nil {
		s.abort(gen)
		return
	}
	if err == nil {
		err = codeErr(CodeUnknown, "unknown failure")
	}
	s.emit(event{kind: evFailed, gen: gen, code: err.code, detail: err.detail})
}

func (s *Supervisor) abort(gen uint64) {
	s.emit(event{kind: evAborted, gen: gen})
}
