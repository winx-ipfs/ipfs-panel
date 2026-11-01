package node

import (
	"io"
	"os/exec"
	"time"
)

// running is one Kubo daemon the manager spawned or adopted.
type running struct {
	pid     int
	cmd     *exec.Cmd
	stderr  *tailBuf
	done    chan struct{}
	waitErr error
}

func (r *running) exited() bool {
	if r == nil {
		return true
	}
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

func (r *running) stderrString() string {
	if r == nil || r.stderr == nil {
		return ""
	}
	return r.stderr.String()
}

func spawnDaemon(bin, repo string) (*running, error) {
	// Argument vector only: the binary, then "daemon". No shell, no extra flags.
	cmd := exec.Command(bin, "daemon")
	cmd.Env = withIPFSPath(nilEnv(), repo)
	cmd.Stdout = io.Discard
	stderr := newTail(detailLimit)
	cmd.Stderr = stderr
	cmd.SysProcAttr = daemonSysProcAttr()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	child := &running{
		pid:    cmd.Process.Pid,
		cmd:    cmd,
		stderr: stderr,
		done:   make(chan struct{}),
	}
	go func() {
		child.waitErr = cmd.Wait()
		close(child.done)
	}()
	return child, nil
}

func nilEnv() []string {
	return osEnviron()
}

func adoptProcess(pid int) *running {
	child := &running{
		pid:    pid,
		stderr: newTail(detailLimit),
		done:   make(chan struct{}),
	}
	go func() {
		for pidAlive(pid) {
			time.Sleep(50 * time.Millisecond)
		}
		close(child.done)
	}()
	return child
}
